package discord

import (
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/sokdak/eternity-bot/pkg/model"
	"strconv"
	"strings"
	"time"
)

func SendInteractionWithButtons(s *discordgo.Session, i *discordgo.Interaction, description string, buttons map[string]string, update bool) {
	var components []discordgo.MessageComponent
	var actionRow discordgo.ActionsRow
	for k, v := range buttons {
		style := discordgo.SecondaryButton
		if strings.Contains(v, "_") {
			style = discordgo.SuccessButton
		}
		actionRow.Components = append(actionRow.Components, discordgo.Button{
			Label:    k,
			Style:    style,
			CustomID: v,
		})
	}
	components = append(components, actionRow)
	if update {
		s.InteractionRespond(i, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseUpdateMessage,
			Data: &discordgo.InteractionResponseData{
				Content:    description,
				Components: components,
			},
		})
	} else {
		s.InteractionRespond(i, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content:    description,
				Components: components,
			},
		})
	}
}

func SendNewRaidModal(s *discordgo.Session, i *discordgo.Interaction) {
	err := s.InteractionRespond(i, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseModal,
		Data: &discordgo.InteractionResponseData{
			Title:    "공대 추가",
			CustomID: "add-raid-modal",
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.TextInput{
							CustomID:    "raid-name",
							Label:       "공대 이름",
							Style:       discordgo.TextInputShort,
							Placeholder: "영원공대",
							Required:    true,
							MinLength:   3,
							MaxLength:   20,
						},
					},
				},
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.TextInput{
							CustomID:    "raid-type",
							Label:       "공대 타입",
							Style:       discordgo.TextInputShort,
							Placeholder: "자쿰, 혼테일, 파풀라투스, 피아누스",
							Required:    true,
						},
					},
				},
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.TextInput{
							CustomID:    "raid-description",
							Label:       "공대 설명",
							Style:       discordgo.TextInputParagraph,
							Placeholder: "설명",
							Required:    false,
						},
					},
				},
			},
		},
	})

	if err != nil {
		panic(err)
	}
}

func SendNewRaidScheduleModal(s *discordgo.Session, i *discordgo.Interaction, raidID string) {
	err := s.InteractionRespond(i, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseModal,
		Data: &discordgo.InteractionResponseData{
			Title:    "일정 추가",
			CustomID: "add-raid-schedule-modal_" + raidID,
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.TextInput{
							CustomID:    "start-time",
							Placeholder: "2025-01-02 21:00",
							Value:       time.Now().In(loc).Format("2006-01-02 15:04"),
							Label:       "시작 날짜 및 시간",
							Required:    true,
							Style:       discordgo.TextInputShort,
						},
					},
				},
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.TextInput{
							CustomID:    "subscription-end-time",
							Placeholder: "2025-01-02 21:00",
							Value:       time.Now().In(loc).Format("2006-01-02 15:04"),
							Label:       "모집 마감 날짜 및 시간",
							Required:    true,
							Style:       discordgo.TextInputShort,
						},
					},
				},
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.TextInput{
							CustomID:    "try-count",
							Label:       "트라이",
							Placeholder: "1",
							Required:    true,
							Style:       discordgo.TextInputShort,
						},
					},
				},
			},
		},
	})

	if err != nil {
		panic(err)
	}
}

var loc, _ = time.LoadLocation("Asia/Seoul")

func SendEditRaidScheduleModal(s *discordgo.Session, i *discordgo.Interaction, schedule model.RaidSchedule) {
	err := s.InteractionRespond(i, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseModal,
		Data: &discordgo.InteractionResponseData{
			Title:    "일정 수정",
			CustomID: fmt.Sprintf("edit-raid-schedule-modal_%d", schedule.ID),
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.TextInput{
							CustomID:    "start-time",
							Placeholder: "2025-01-02 21:00",
							Value:       schedule.StartTime.In(loc).Format("2006-01-02 15:04"),
							Label:       "시작 날짜 및 시간",
							Required:    true,
							Style:       discordgo.TextInputShort,
						},
					},
				},
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.TextInput{
							CustomID:    "subscription-end-time",
							Placeholder: "2025-01-02 21:00",
							Value:       schedule.SubscriptionEndTime.In(loc).Format("2006-01-02 15:04"),
							Label:       "모집 마감 날짜 및 시간",
							Required:    true,
							Style:       discordgo.TextInputShort,
						},
					},
				},
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.TextInput{
							CustomID:    "try-count",
							Label:       "트라이",
							Value:       strconv.Itoa(schedule.TryCount),
							Placeholder: "1",
							Required:    true,
							Style:       discordgo.TextInputShort,
						},
					},
				},
			},
		},
	})

	if err != nil {
		panic(err)
	}
}

func SendAdminAddAttendeeModal(s *discordgo.Session, i *discordgo.Interaction, scheduleID string) {
	err := s.InteractionRespond(i, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseModal,
		Data: &discordgo.InteractionResponseData{
			Title:    "참가자 추가",
			CustomID: "admin-add-attendee-modal_" + scheduleID,
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.TextInput{
							CustomID: "nickname",
							Label:    "닉네임",
							Required: true,
							Style:    discordgo.TextInputShort,
						},
					},
				},
			},
		},
	})

	if err != nil {
		panic(err)
	}
}

func SendAdminRaidInfoResponse(s *discordgo.Session, i *discordgo.Interaction, schedule model.RaidSchedule, info model.RaidInfo, attendCount int) error {
	// raid record
	msg := fmt.Sprintf("**[%s] %s (%d 트라이) 레이드 기록**\n\n", schedule.Raid.RaidName, schedule.StartTime.Format("2006-01-02 15:04"), schedule.TryCount)
	if info.EntranceTime.IsZero() {
		msg += "* 입장: 기록 없음\n"
	} else {
		msg += fmt.Sprintf("* 입장: %s\n", info.EntranceTime.Format("2006-01-02 15:04"))
	}
	if info.StartTime.IsZero() {
		msg += "* 시작: 기록 없음\n"
	} else {
		msg += fmt.Sprintf("* 시작: %s\n", info.StartTime.Format("2006-01-02 15:04"))
	}
	if info.EndTime.IsZero() {
		msg += "* 종료: 기록 없음\n"
	} else {
		msg += fmt.Sprintf("* 종료: %s\n", info.EndTime.Format("2006-01-02 15:04"))
	}
	msg += fmt.Sprintf("* 참가자: %d명", attendCount)

	s.InteractionRespond(i, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Content: msg,
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.Button{
							Label:    "입장 기록",
							Style:    discordgo.PrimaryButton,
							CustomID: fmt.Sprintf("admin-info-record-entrance_%d", info.ID),
						},
						discordgo.Button{
							Label:    "시작 기록",
							Style:    discordgo.SecondaryButton,
							CustomID: fmt.Sprintf("admin-info-record-start_%d", info.ID),
						},
						discordgo.Button{
							Label:    "종료 기록",
							Style:    discordgo.SecondaryButton,
							CustomID: fmt.Sprintf("admin-info-record-end_%d", info.ID),
						},
						discordgo.Button{
							Label:    "파티 구성",
							Style:    discordgo.SecondaryButton,
							CustomID: fmt.Sprintf("admin-info-party-formation_%d", info.ID),
						},
					},
				},
			},
		},
	})

	return nil
}

func SendCounselModal(s *discordgo.Session, i *discordgo.Interaction) {
	err := s.InteractionRespond(i, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseModal,
		Data: &discordgo.InteractionResponseData{
			Title:    "건의사항 제출",
			CustomID: "counsel-modal",
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.TextInput{
							CustomID:    "counsel-privacy",
							Label:       "공개 여부",
							Style:       discordgo.TextInputShort,
							Placeholder: "실명 or 익명",
							Required:    true,
						},
					},
				},
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.TextInput{
							CustomID:    "counsel-category",
							Label:       "카테고리",
							Style:       discordgo.TextInputShort,
							Placeholder: "제안/신고/버그/기타",
							Required:    true,
						},
					},
				},
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.TextInput{
							CustomID:    "counsel-title",
							Label:       "제목",
							Style:       discordgo.TextInputShort,
							Placeholder: "제목을 입력하세요",
							Required:    true,
						},
					},
				},
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.TextInput{
							CustomID:    "counsel-content",
							Label:       "내용",
							Style:       discordgo.TextInputParagraph,
							Placeholder: "상세 내용을 입력해주세요. (생략 가능)",
						},
					},
				},
			},
		},
	})

	if err != nil {
		panic(err)
	}
}
