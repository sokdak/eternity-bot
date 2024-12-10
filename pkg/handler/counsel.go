package handler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/dstotijn/go-notion"
	"time"
)

func CounselPoller(s *discordgo.Session, n *notion.Client, startTime time.Time, notionCounselDBID, guildID, counselChannelID string) error {
	msgs, err := s.ChannelMessages(counselChannelID, 100, "", "", "")
	if err != nil {
		return fmt.Errorf("error getting messages: %w", err)
	}

	for _, msg := range msgs {
		// ignoring messages before the bot was started
		if msg.Timestamp.Before(startTime) {
			continue
		}

		id := msg.ID

		// generate sha-256 hash of the message content
		h := sha256.Sum256([]byte(msg.Content))
		sum := hex.EncodeToString(h[:])

		// check if the counsel message is already posted to notion
		// search notion database with the message id
		filter := &notion.DatabaseQueryFilter{
			Property: "CounselID",
			DatabaseQueryPropertyFilter: notion.DatabaseQueryPropertyFilter{
				RichText: &notion.TextPropertyFilter{
					Equals: id,
				},
			},
		}

		// 데이터베이스 쿼리 실행
		res, err := n.QueryDatabase(context.Background(), notionCounselDBID, &notion.DatabaseQuery{
			Filter: filter,
		})
		if err != nil {
			return fmt.Errorf("error querying database: %w", err)
		}

		pages := res.Results
		if len(pages) == 0 {
			// create a new page in the database
			// get user info
			user, err := s.GuildMember(guildID, msg.Author.ID)
			if err != nil {
				return fmt.Errorf("error getting user: %w", err)
			}

			// create a new page
			if err := createPage(n, notionCounselDBID, user.Nick, msg.ID, msg.Content); err != nil {
				return err
			}
			continue
		}

		// check hash of the message content
		page := pages[0]
		properties := page.Properties.(notion.DatabasePageProperties)
		if hash, ok := properties["CounselHash"]; ok && len(hash.RichText) != 0 {
			if hash.RichText[0].PlainText == sum {
				// skip this message
				continue
			}
		} else {
			// get user info
			user, err := s.GuildMember(guildID, msg.Author.ID)
			if err != nil {
				return fmt.Errorf("error getting user: %w", err)
			}
			if err := modifyPage(n, page.ID, user.User.Username, msg.Content); err != nil {
				return err
			}
		}
	}
	return nil
}

func createPage(n *notion.Client, parentID, authorNick, contentID, content string) error {
	// create a new page
	cParams := notion.CreatePageParams{
		ParentID:   parentID,
		ParentType: "database_id",
		DatabasePageProperties: &notion.DatabasePageProperties{
			"CounselHash": notion.DatabasePageProperty{
				RichText: []notion.RichText{
					{
						Text: &notion.Text{
							Content: "",
						},
					},
				},
			},
			"CounselID": notion.DatabasePageProperty{
				RichText: []notion.RichText{
					{
						Text: &notion.Text{
							Content: contentID,
						},
					},
				},
			},
			"진행 상태": notion.DatabasePageProperty{
				Status: &notion.SelectOptions{
					Name: "시작 전",
				},
			},
			"작업": notion.DatabasePageProperty{
				Title: []notion.RichText{
					{
						Text: &notion.Text{
							Content: fmt.Sprintf("%s님 건의사항 (%s)", authorNick, time.Now().Format("060102 15:04:05")),
						},
					},
				},
			},
		},
	}
	page, err := n.CreatePage(context.Background(), cParams)
	if err != nil {
		return fmt.Errorf("error creating page: %w", err)
	}

	// create a new block
	params := []notion.Block{
		&notion.ParagraphBlock{
			RichText: []notion.RichText{
				{
					Text: &notion.Text{
						Content: fmt.Sprintf("건의자: %s", authorNick),
					},
				},
			},
		},
		&notion.CodeBlock{
			RichText: []notion.RichText{
				{
					Text: &notion.Text{
						Content: fmt.Sprintf(content),
					},
				},
			},
			Language: notion.StringPtr("markdown"),
		},
	}

	// 블록 추가 실행
	_, err = n.AppendBlockChildren(context.Background(), page.ID, params)
	if err != nil {
		return fmt.Errorf("error appending block children: %w", err)
	}

	h := sha256.Sum256([]byte(content))
	sum := hex.EncodeToString(h[:])

	// set the hash to the current message content
	updatedProperties := notion.DatabasePageProperties{
		"CounselHash": notion.DatabasePageProperty{
			RichText: []notion.RichText{
				{
					Text: &notion.Text{
						Content: sum,
					},
				},
			},
		},
	}

	// update the page
	_, err = n.UpdatePage(context.Background(), page.ID, notion.UpdatePageParams{
		DatabasePageProperties: updatedProperties,
	})
	if err != nil {
		return fmt.Errorf("error updating page: %w", err)
	}

	return nil
}

func modifyPage(n *notion.Client, pageID, authorNick, content string) error {
	// if no richtext, assume that writing has been stopped before full write counsel
	// need to update the content and hash
	blocks, err := n.FindBlockChildrenByID(context.Background(), pageID, nil)
	if err != nil {
		return fmt.Errorf("error finding block children: %w", err)
	}

	// remove all blocks if found
	for _, b := range blocks.Results {
		_, err := n.DeleteBlock(context.Background(), b.ID())
		if err != nil {
			return fmt.Errorf("error deleting block: %w", err)
		}
	}

	// create a new block
	params := []notion.Block{
		&notion.ParagraphBlock{
			RichText: []notion.RichText{
				{
					Text: &notion.Text{
						Content: fmt.Sprintf("건의자: %s", authorNick),
					},
				},
			},
		},
		&notion.CodeBlock{
			RichText: []notion.RichText{
				{
					Text: &notion.Text{
						Content: fmt.Sprintf(content),
					},
				},
			},
			Language: notion.StringPtr("markdown"),
		},
	}

	// 블록 추가 실행
	_, err = n.AppendBlockChildren(context.Background(), pageID, params)
	if err != nil {
		return fmt.Errorf("error appending block children: %w", err)
	}

	h := sha256.Sum256([]byte(content))
	sum := hex.EncodeToString(h[:])

	// set the hash to the current message content
	updatedProperties := notion.DatabasePageProperties{
		"CounselHash": notion.DatabasePageProperty{
			RichText: []notion.RichText{
				{
					Text: &notion.Text{
						Content: sum,
					},
				},
			},
		},
	}

	// update the page
	_, err = n.UpdatePage(context.Background(), pageID, notion.UpdatePageParams{
		DatabasePageProperties: updatedProperties,
	})
	if err != nil {
		return fmt.Errorf("error updating page: %w", err)
	}

	return nil
}
