package handler

import (
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/sokdak/eternity-bot/pkg/cache"
	"github.com/sokdak/eternity-bot/pkg/environment"
	"slices"
	"strings"
	"time"
)

func ModUserInit(dg *discordgo.Session) error {
	err := RegisterModUserCommand(dg)
	if err != nil {
		panic(err)
	}

	dg.AddHandler(modUserIntegratedHandler)
	return nil
}

func RegisterModUserCommand(dg *discordgo.Session) error {
	commands := []*discordgo.ApplicationCommand{
		{
			Name:        "운영",
			Description: "운영 명령어",
		},
	}

	for _, cmd := range commands {
		_, err := dg.ApplicationCommandCreate(
			dg.State.User.ID,
			environment.DiscordGuildID,
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

func modUserIntegratedHandler(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.Type == discordgo.InteractionApplicationCommand {
		data := i.ApplicationCommandData()
		if data.Name == "운영" {
			if i.ChannelID != environment.DiscordGuildPollChannelID {
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: "운영 명령어는 운영 채널에서만 사용할 수 있습니다.",
					},
				})
				return
			}
			if err := printLandingPage(s, i.Interaction, false); err != nil {
				fmt.Printf("Cannot respond to command: %v\n", err)
			}
		}
	} else if i.Type == discordgo.InteractionMessageComponent {
		data := i.MessageComponentData()
		switch data.CustomID {
		case "landing-page":
			printLandingPage(s, i.Interaction, true)
		case "register-guild-member-list":
			registerGuildMemberListing(s, i.Interaction)
		case "register-guild-member-selected":
			registerGuildMemberModal(s, i.Interaction, data.Values[0])
		case "remove-guild-permission":
			deregisterGuildMember(s, i.Interaction)
		case "kick-member":
			kickMember(s, i.Interaction)
		}
	} else if i.Type == discordgo.InteractionModalSubmit {
		data := i.ModalSubmitData()
		args := strings.Split(data.CustomID, "_")
		op := args[0]
		switch op {
		case "register-guild-member-modal":
			if len(args) < 2 {
				return
			}
			registerGuildMemberModalSubmit(s, i.Interaction, data, args[1])
		}
	}
}

func printLandingPage(s *discordgo.Session, i *discordgo.Interaction, update bool) error {
	t := discordgo.InteractionResponseChannelMessageWithSource
	if update {
		t = discordgo.InteractionResponseUpdateMessage
	}

	err := s.InteractionRespond(i, &discordgo.InteractionResponse{
		Type: t,
		Data: &discordgo.InteractionResponseData{
			Content: "안녕하세요, 영원길드 멤버 관리 명령어입니다.",
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.Button{
							Label:    "길드권한 추가",
							Style:    discordgo.PrimaryButton,
							CustomID: "register-guild-member-list",
						},
						discordgo.Button{
							Label:    "길드권한 삭제",
							Style:    discordgo.DangerButton,
							CustomID: "remove-guild-permission",
						},
						discordgo.Button{
							Label:    "디스코드 추방",
							Style:    discordgo.DangerButton,
							CustomID: "kick-member",
						},
					},
				},
			},
		},
	})
	return err
}

func registerGuildMemberListing(s *discordgo.Session, i *discordgo.Interaction) {
	members, err := s.GuildMembers(environment.DiscordGuildID, "", 1000)

	// 영원 role id 조회
	roleID := cache.GetRoleID("영원")

	// 현재 시각으로부터 6시간 전 기준
	oneHourAgo := time.Now().Add(-6 * time.Hour)
	var recentMembers []*discordgo.Member
	for _, m := range members {
		if m.JoinedAt.After(oneHourAgo) && !slices.Contains(m.Roles, roleID) {
			recentMembers = append(recentMembers, m)
		}
	}

	// create modal
	var components []discordgo.SelectMenuOption
	for _, m := range recentMembers {
		components = append(components, discordgo.SelectMenuOption{
			Label: fmt.Sprintf("%s (%s 가입)", m.User.GlobalName, m.JoinedAt.Format("2006-01-02 15:04:05")),
			Value: m.User.ID,
		})
	}

	if len(components) == 0 {
		err = s.InteractionRespond(i, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseUpdateMessage,
			Data: &discordgo.InteractionResponseData{
				Content: "최근 6시간 내 영원 길드 가입자가 없거나 이미 길드에 등록되었습니다.",
				Components: []discordgo.MessageComponent{
					discordgo.ActionsRow{
						Components: []discordgo.MessageComponent{
							discordgo.Button{
								Label:    "처음으로 돌아가기",
								Style:    discordgo.SecondaryButton,
								CustomID: "landing-page",
							},
						},
					},
				},
			},
		})
		if err != nil {
			fmt.Printf("Cannot respond to command: %v\n", err)
			return
		}
	}

	err = s.InteractionRespond(i, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "길드원 등록을 할 멤버를 선택하세요.\n(최근 1시간 내 영원 길드 가입자 목록, 이미 길드에 등록되었을 시 나타나지 않음)",
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.SelectMenu{
							CustomID: "register-guild-member-selected",
							Options:  components,
						},
					},
				},
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.Button{
							Label:    "처음으로 돌아가기",
							Style:    discordgo.SecondaryButton,
							CustomID: "landing-page",
						},
					},
				},
			},
		},
	})
	if err != nil {
		fmt.Printf("Cannot respond to command: %v\n", err)
		return
	}
}

func registerGuildMemberModal(s *discordgo.Session, interaction *discordgo.Interaction, memberId string) {
	s.InteractionRespond(interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseModal,
		Data: &discordgo.InteractionResponseData{
			Title:    "길드원 등록",
			CustomID: "register-guild-member-modal_" + memberId,
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.TextInput{
							CustomID:    "nickname",
							Label:       "닉네임",
							Style:       discordgo.TextInputShort,
							Placeholder: "닉네임을 입력해주세요.",
							Required:    true,
						},
					},
				},
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.TextInput{
							CustomID:    "level",
							Label:       "레벨",
							Style:       discordgo.TextInputShort,
							Placeholder: "레벨을 입력해주세요.",
							Required:    true,
						},
					},
				},
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.TextInput{
							CustomID:    "job",
							Label:       "직업",
							Style:       discordgo.TextInputShort,
							Placeholder: "아크메이지(썬,콜)/아크메이지(불,독)/나이트로드...",
							Required:    true,
						},
					},
				},
			},
		},
	})
}

func registerGuildMemberModalSubmit(s *discordgo.Session, i *discordgo.Interaction, data discordgo.ModalSubmitInteractionData, memberID string) {
	// cast component data to text input
	nickname := data.Components[0].(*discordgo.ActionsRow).Components[0].(*discordgo.TextInput).Value
	level := data.Components[1].(*discordgo.ActionsRow).Components[0].(*discordgo.TextInput).Value
	job := data.Components[2].(*discordgo.ActionsRow).Components[0].(*discordgo.TextInput).Value

	// get member role id
	roleID := cache.GetRoleID("영원")
	jobID := cache.GetRoleID(job)
	if len(jobID) == 0 {
		s.InteractionRespond(i, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseUpdateMessage,
			Data: &discordgo.InteractionResponseData{
				Content: fmt.Sprintf("직업 '%s'를 찾을 수 없습니다. 직업 명을 정확하게 입력해주세요.\n(히어로/팔라딘/다크나이트/보우마스터/신궁/아크메이지(썬,콜)/아크메이지(불,독)/나이트로드/섀도어)", job),
				Components: []discordgo.MessageComponent{
					discordgo.ActionsRow{
						Components: []discordgo.MessageComponent{
							discordgo.Button{
								Label:    "처음으로 돌아가기",
								Style:    discordgo.SecondaryButton,
								CustomID: "landing-page",
							},
						},
					},
				},
			},
		})
		return
	}

	m := cache.ListAllMembersNicknameMap()
	if _, exist := m[nickname]; exist {
		s.InteractionRespond(i, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: fmt.Sprintf("'%s'는 이미 길드에 등록된 닉네임입니다. 초대하려는 캐릭터의 닉네임을 확인해주세요.", nickname),
				Components: []discordgo.MessageComponent{
					discordgo.ActionsRow{
						Components: []discordgo.MessageComponent{
							discordgo.Button{
								Label:    "처음으로 돌아가기",
								Style:    discordgo.SecondaryButton,
								CustomID: "landing-page",
							},
						},
					},
				},
			},
		})
		return
	}

	// add member to guild
	err := s.GuildMemberRoleAdd(environment.DiscordGuildID, memberID, roleID)
	if err != nil {
		fmt.Printf("Cannot add member to guild: %v\n", err)
		return
	}

	// add job role
	err = s.GuildMemberRoleAdd(environment.DiscordGuildID, memberID, jobID)
	if err != nil {
		fmt.Printf("Cannot add job role to guild: %v\n", err)
		return
	}

	// change nickname
	err = s.GuildMemberNickname(environment.DiscordGuildID, memberID, fmt.Sprintf("Lv %s %s", level, nickname))
	if err != nil {
		fmt.Printf("Cannot change nickname: %v\n", err)
		return
	}

	// send message to user
	err = s.InteractionRespond(i, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: fmt.Sprintf("'%s'의 길드원 등록이 완료되었습니다.", nickname),
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.Button{
							Label:    "처음으로 돌아가기",
							Style:    discordgo.SecondaryButton,
							CustomID: "landing-page",
						},
					},
				},
			},
		},
	})
	if err != nil {
		fmt.Printf("Cannot respond to command: %v\n", err)
	}
}

func deregisterGuildMember(s *discordgo.Session, interaction *discordgo.Interaction) {
	s.InteractionRespond(interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Content: "길드 권한 삭제 기능은 준비중입니다.",
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.Button{
							Label:    "처음으로 돌아가기",
							Style:    discordgo.SecondaryButton,
							CustomID: "landing-page",
						},
					},
				},
			},
		},
	})
}

func kickMember(s *discordgo.Session, interaction *discordgo.Interaction) {
	s.InteractionRespond(interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Content: "디스코드 추방 기능은 준비중입니다.",
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.Button{
							Label:    "처음으로 돌아가기",
							Style:    discordgo.SecondaryButton,
							CustomID: "landing-page",
						},
					},
				},
			},
		},
	})
}
