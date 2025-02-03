package handler

import (
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/dstotijn/go-notion"
	"github.com/sokdak/eternity-bot/pkg/cache"
	"github.com/sokdak/eternity-bot/pkg/discord"
)

var nc *notion.Client
var ncdbid string

func Counsel2Init(dg *discordgo.Session, n *notion.Client, notionCounselDBID string) error {
	if err := RegisterCounselCommand(dg); err != nil {
		return err
	}

	nc = n
	ncdbid = notionCounselDBID

	dg.AddHandler(counselHandler)
	return nil
}

func RegisterCounselCommand(dg *discordgo.Session) error {
	commands := []*discordgo.ApplicationCommand{
		{
			Name:        "건의사항",
			Description: "건의사항 명령어",
		},
	}

	for _, cmd := range commands {
		_, err := dg.ApplicationCommandCreate(
			dg.State.User.ID,
			"",
			cmd,
		)
		if err != nil {
			fmt.Printf("Cannot create '%v' command: %v\n", cmd.Name, err)
			return err
		}

		fmt.Printf("Registered command: /%s\n", cmd.Name)
	}

	return nil
}

func counselHandler(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.Type == discordgo.InteractionApplicationCommand {
		data, ok := i.Data.(discordgo.ApplicationCommandInteractionData)
		if !ok || data.Name != "건의사항" {
			return
		}

		// check member
		m := cache.GetGuildMember(i.User.ID)
		if m == nil {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "메이플랜드 영원 길드원이 아닙니다.",
				},
			})
			return
		}

		// print counsel modal
		discord.SendCounselModal(s, i.Interaction)
		return
	}

	if i.Type == discordgo.InteractionModalSubmit {
		data, ok := i.Data.(discordgo.ModalSubmitInteractionData)
		if !ok || data.CustomID != "counsel-modal" {
			return
		}
		counselModalHandler(s, i, data.CustomID)
	}
}

func counselModalHandler(s *discordgo.Session, i *discordgo.InteractionCreate, id string) {
	data := i.ModalSubmitData()
	var privacy, category, title, content string
	for _, comp := range data.Components {
		if ar, ok := comp.(*discordgo.ActionsRow); ok {
			if ti, ok := ar.Components[0].(*discordgo.TextInput); ok {
				switch ti.CustomID {
				case "counsel-privacy":
					privacy = ti.Value
				case "counsel-category":
					category = ti.Value
				case "counsel-title":
					title = ti.Value
				case "counsel-content":
					content = ti.Value
				}
			}
		}
	}

	// register to notion
	nick := "익명"
	if privacy == "실명" {
		m := cache.GetGuildMember(i.User.ID)
		if m == nil {
			fmt.Println("Cannot find member")
			return
		}

		mn, err := GetMemberInfoFromMember(m)
		if err != nil {
			fmt.Println(err)
			return
		}

		nick = fmt.Sprintf("Lv %d %s", mn.Level, mn.Nickname)
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "건의사항을 제출하는 중입니다...",
		},
	})

	var body string
	if privacy == "익명" {
		body = fmt.Sprintf("[%s] 익명 건의사항\n제목: %s\n\n%s", category, title, content)
	} else {
		body = fmt.Sprintf("[%s] %s님 건의사항\n제목: %s\n\n%s", category, nick, title, content)
	}
	if err := createPage(nc, ncdbid, nick, "bot-counsel-forward", body); err != nil {
		fmt.Println(err)
		return
	}

	// send response
	s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Content: notion.StringPtr("건의사항이 제출되었습니다."),
	})
}
