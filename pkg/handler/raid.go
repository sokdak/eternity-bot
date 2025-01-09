package handler

import (
	"errors"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/sokdak/eternity-bot/pkg/cache"
	"github.com/sokdak/eternity-bot/pkg/discord"
	"github.com/sokdak/eternity-bot/pkg/environment"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"slices"
	"strconv"
	"strings"
	"time"
)

type Raid struct {
	gorm.Model
	RaidName    string
	Type        string
	Description string
	Manager     string
}

type RaidSchedule struct {
	gorm.Model
	RaidID              uint
	Raid                Raid `gorm:"foreignKey:RaidID"`
	TryCount            int
	SubscriptionEndTime time.Time
	StartTime           time.Time
	MessageID           string
}

type RaidAttend struct {
	gorm.Model
	MemberInfo
	Canceled       bool
	RaidScheduleID uint
	RaidSchedule   RaidSchedule `gorm:"foreignKey:RaidScheduleID"`
}

type RaidInfo struct {
	gorm.Model
	EntranceTime   time.Time
	StartTime      time.Time
	EndTime        time.Time
	RaidScheduleID uint
	RaidSchedule   RaidSchedule `gorm:"foreignKey:RaidScheduleID"`
}

var rdb *gorm.DB

func RaidSubscriptionRefresh(dg *discordgo.Session) error {
	// list schedules
	var schedules []RaidSchedule
	rdb.Find(&schedules)

	for _, sc := range schedules {
		// find attends
		var attends []RaidAttend
		rdb.Where("raid_schedule_id = ?", sc.ID).Find(&attends)

		// populate member list
		memberListByRole := map[string][]string{}
		for _, a := range attends {
			role := a.SubRoleName
			if memberListByRole[role] == nil {
				memberListByRole[role] = make([]string, 0)
			}
			memberListByRole[role] = append(memberListByRole[role], fmt.Sprintf("* %s", a.Mention))
		}

		// extract key and sort
		var keys []string
		for k := range memberListByRole {
			keys = append(keys, k)
		}
		slices.Sort(keys)

		// send message
		var msg string

		msg += fmt.Sprintf("**%s - %d트라이 (%s 출발)**\n\n",
			sc.StartTime.Format("01월 02일"), sc.TryCount, sc.StartTime.Format("15:04"))

		for _, k := range keys {
			msg += fmt.Sprintf("**%s**\n", k)
			msg += strings.Join(memberListByRole[k], "\n")
			msg += "\n\n"
		}

		// get latest message
		cmsg, err := dg.ChannelMessage(environment.DiscordGuildRaidSubscriptionChannelID, sc.MessageID)
		if err != nil {
			fmt.Println("failed to get message:", err)
			continue
		}

		if cmsg.Content == msg {
			continue
		}

		// send message
		_, err = dg.ChannelMessageEdit(environment.DiscordGuildRaidSubscriptionChannelID, sc.MessageID, msg)
		if err != nil {
			fmt.Println("failed to edit message:", err)
			continue
		}
	}

	return nil
}

func RaidInit(dg *discordgo.Session) error {
	var err error
	rdb, err = gorm.Open(sqlite.Open(environment.RaidSQLiteDBPath), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	err = rdb.AutoMigrate(&Raid{})
	if err != nil {
		return fmt.Errorf("failed to migrate database: %w", err)
	}

	err = rdb.AutoMigrate(&RaidSchedule{})
	if err != nil {
		return fmt.Errorf("failed to migrate database: %w", err)
	}

	err = rdb.AutoMigrate(&RaidAttend{})
	if err != nil {
		return fmt.Errorf("failed to migrate database: %w", err)
	}

	err = rdb.AutoMigrate(&RaidInfo{})
	if err != nil {
		return fmt.Errorf("failed to migrate database: %w", err)
	}

	// add watchers
	dg.AddHandler(raidScheduleHandler)
	return nil
}

func RaidFinalize() {
	sqlDB, err := rdb.DB()
	if err != nil {
		fmt.Println("failed to get db connection for close: %w", err)
		return
	}
	_ = sqlDB.Close()
}

func RegisterRaidCommands(dg *discordgo.Session) error {
	commands := []*discordgo.ApplicationCommand{
		{
			Name:        "레이드",
			Description: "레이드 참여 명령어",
		},
		{
			Name:        "레이드관리",
			Description: "레이드 일정 관리 명령어",
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

func UnregisterCommands(s *discordgo.Session) {
	//registeredCommands, err := s.ApplicationCommands(s.State.User.ID, "")
	//if err != nil {
	//	fmt.Println("Error fetching registered commands:", err)
	//}

	//for _, cmd := range registeredCommands {
	//err := s.ApplicationCommandDelete(s.State.User.ID, "", cmd.ID)
	//if err != nil {
	//	fmt.Printf("Cannot delete '%v' command: %v\n", cmd.Name, err)
	//} else {
	//	fmt.Printf("Deleted command: /%s\n", cmd.Name)
	//}
	//}
}

func raidScheduleUserInitialHandler(s *discordgo.Session, i *discordgo.InteractionCreate) {
	memberInfo := cache.GetGuildMember(i.User.ID)
	if memberInfo == nil {
		return
	}
	m, err := GetMemberInfoFromMember(memberInfo)
	if err != nil {
		return
	}

	// list my schedules
	var attends []RaidAttend
	rdb.Preload("RaidSchedule").Preload("RaidSchedule.Raid").Where("mention = ?", m.Mention).Find(&attends)

	var attendList []string
	for _, a := range attends {
		raidName := a.RaidSchedule.Raid.RaidName
		raidStartTime := a.RaidSchedule.StartTime.Format("2006-01-02 15:04")
		raidTryCount := a.RaidSchedule.TryCount

		// 다가오는 스케줄만 보여주기
		if a.RaidSchedule.StartTime.After(time.Now().In(loc)) {
			attendList = append(attendList, fmt.Sprintf("[%s] %s (%d트라이)", raidName, raidStartTime, raidTryCount))
		}
	}

	msg := fmt.Sprintf("안녕하세요 %s 님, 영원길드 레이드 관리 시스템입니다.\n\n", m.Nickname)

	if len(attendList) > 0 {
		msg += "**[참가중인 레이드 일정]**\n"
		msg += strings.Join(attendList, "\n")
		msg += "\n"
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: msg,
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.Button{
							Label:    "참가 신청",
							Style:    discordgo.PrimaryButton,
							CustomID: "user-attend-schedule",
						},
						discordgo.Button{
							Label:    "참가 취소",
							Style:    discordgo.DangerButton,
							CustomID: "user-cancel-schedule",
						},
					},
				},
			},
		},
	})
}

func raidScheduleAdminInitialHandler(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// list schedules
	var schedules []RaidSchedule
	rdb.Preload("Raid").Find(&schedules)

	// list schedules
	var upcoming []string
	for _, sc := range schedules {
		raidName := sc.Raid.RaidName
		raidStartTime := sc.StartTime.Format("2006-01-02 15:04")
		raidTryCount := sc.TryCount
		// 24시간 이내 다가오는 스케줄 보여주기
		if sc.StartTime.Before(time.Now().In(loc).AddDate(0, 0, 1)) {
			upcoming = append(upcoming, fmt.Sprintf("* [%s] %s (%d트라이)", raidName, raidStartTime, raidTryCount))
		}
	}

	msg := "영원길드 레이드 참가관리 시스템입니다.\n\n"
	if len(upcoming) > 0 {
		msg += "**[다가오는 레이드 일정]**\n"
		msg += strings.Join(upcoming, "\n")
		msg += "\n\n"
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: msg,
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.Button{
							Label:    "일정 추가",
							Style:    discordgo.PrimaryButton,
							CustomID: "admin-add-schedule",
						},
						discordgo.Button{
							Label:    "일정 수정",
							Style:    discordgo.SecondaryButton,
							CustomID: "admin-edit-schedule",
						},
						discordgo.Button{
							Label:    "참가자 관리",
							Style:    discordgo.SecondaryButton,
							CustomID: "admin-edit-attendance",
						},
						discordgo.Button{
							Label:    "레이드 기록",
							Style:    discordgo.SecondaryButton,
							CustomID: "admin-manage-info",
						},
						discordgo.Button{
							Label:    "일정 삭제",
							Style:    discordgo.DangerButton,
							CustomID: "admin-remove-schedule",
						},
					},
				},
			},
		},
	})
}

func raidScheduleIntegratedHandler(s *discordgo.Session, i *discordgo.InteractionCreate, actionID string) {
	args := strings.Split(actionID, "_")
	if len(args) == 1 {
		switch args[0] {
		case "admin-add-new-raid":
			discord.SendNewRaidModal(s, i.Interaction)
		case "admin-add-schedule":
			// get existing raids
			var raids []Raid
			rdb.Find(&raids)

			// list raids
			raidSelectionMap := make(map[string]string)
			for _, r := range raids {
				raidSelectionMap[r.RaidName] = "admin-add-schedule-select-raid_" + fmt.Sprintf("%d", r.ID)
			}
			raidSelectionMap["새로운 레이드 추가"] = "admin-add-new-raid"

			// send message
			discord.SendInteractionWithButtons(s, i.Interaction, "추가 할 레이드 일정을 선택하세요.", raidSelectionMap, true)
		case "admin-remove-schedule":
			// list schedules
			var schedules []RaidSchedule
			rdb.Preload("Raid").Find(&schedules)

			// list schedules
			scheduleSelectionMap := make(map[string]string)
			for _, sc := range schedules {
				raidName := sc.Raid.RaidName
				raidStartTime := sc.StartTime.Format("2006-01-02 15:04")
				raidTryCount := sc.TryCount
				// 오늘 기준으로 3일 앞뒤 스케줄만 보여주기
				if sc.StartTime.Before(time.Now().In(loc).AddDate(0, 0, 3)) && sc.StartTime.After(time.Now().In(loc).AddDate(0, 0, -3)) {
					scheduleSelectionMap[fmt.Sprintf("[%s] %s (%d트라이)", raidName, raidStartTime, raidTryCount)] = "admin-remove-schedule-select-schedule_" + fmt.Sprintf("%d", sc.ID)
				}
			}

			// send message
			discord.SendInteractionWithButtons(s, i.Interaction, "삭제 할 레이드 일정을 선택하세요.", scheduleSelectionMap, true)
		case "admin-edit-schedule":
			// list schedules
			var schedules []RaidSchedule
			rdb.Preload("Raid").Find(&schedules)

			// list schedules
			scheduleSelectionMap := make(map[string]string)
			for _, sc := range schedules {
				raidName := sc.Raid.RaidName
				raidStartTime := sc.StartTime.Format("2006-01-02 15:04")
				raidTryCount := sc.TryCount
				scheduleSelectionMap[fmt.Sprintf("[%s] %s (%d트라이)", raidName, raidStartTime, raidTryCount)] = "admin-edit-schedule-select-schedule_" + fmt.Sprintf("%d", sc.ID)
			}

			// generating selectOptions
			var selectOptions []discordgo.SelectMenuOption
			for _, sc := range schedules {
				raidName := sc.Raid.RaidName
				raidStartTime := sc.StartTime.Format("2006-01-02 15:04")
				raidTryCount := sc.TryCount

				// 오늘 기준으로 3일 앞뒤 스케줄만 보여주기
				if sc.StartTime.Before(time.Now().In(loc).AddDate(0, 0, 3)) && sc.StartTime.After(time.Now().In(loc).AddDate(0, 0, -3)) {
					selectOptions = append(selectOptions, discordgo.SelectMenuOption{
						Label: fmt.Sprintf("[%s] %s (%d트라이)", raidName, raidStartTime, raidTryCount),
						Value: fmt.Sprintf("%d", sc.ID),
					})
				}
			}

			if len(selectOptions) == 0 {
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseUpdateMessage,
					Data: &discordgo.InteractionResponseData{
						Flags:   discordgo.MessageFlagsEphemeral,
						Content: "수정 할 레이드 일정이 없습니다.",
					},
				})
				return
			}

			// send message with selectOptions
			err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseUpdateMessage,
				Data: &discordgo.InteractionResponseData{
					Content: "수정 할 레이드 일정을 선택하세요.",
					Components: []discordgo.MessageComponent{
						discordgo.ActionsRow{
							Components: []discordgo.MessageComponent{
								discordgo.SelectMenu{
									CustomID:    "admin-edit-schedule-select-schedule",
									Placeholder: "일정 선택",
									Options:     selectOptions,
								},
							},
						},
					},
				},
			})

			if err != nil {
				panic(err)
			}
		case "admin-edit-schedule-select-schedule":
			selectMenuValues := i.MessageComponentData().Values

			if len(selectMenuValues) > 1 {
				return
			}

			scheduleID := selectMenuValues[0]

			// get schedule
			var schedule RaidSchedule
			err := rdb.First(&schedule, scheduleID).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return
			}

			// modal
			discord.SendEditRaidScheduleModal(s, i.Interaction, scheduleID)
		case "admin-edit-attendance":
			// list schedules
			var schedules []RaidSchedule
			rdb.Preload("Raid").Find(&schedules)

			// create selections
			var selectOptions []discordgo.SelectMenuOption
			for _, sc := range schedules {
				raidName := sc.Raid.RaidName
				raidStartTime := sc.StartTime.Format("2006-01-02 15:04")
				raidTryCount := sc.TryCount

				// 곧 진행될 스케줄 만 보여주기
				if sc.StartTime.After(time.Now().In(loc)) {
					selectOptions = append(selectOptions, discordgo.SelectMenuOption{
						Label: fmt.Sprintf("[%s] %s (%d트라이)", raidName, raidStartTime, raidTryCount),
						Value: fmt.Sprintf("%d", sc.ID),
					})
				}
			}

			if len(selectOptions) == 0 {
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseUpdateMessage,
					Data: &discordgo.InteractionResponseData{
						Flags:   discordgo.MessageFlagsEphemeral,
						Content: "참가자 관리 할 레이드 일정이 없습니다.",
					},
				})
				return
			}

			// send message
			err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseUpdateMessage,
				Data: &discordgo.InteractionResponseData{
					Content: "참가자 관리 할 레이드 일정을 선택하세요.",
					Components: []discordgo.MessageComponent{
						discordgo.ActionsRow{
							Components: []discordgo.MessageComponent{
								discordgo.SelectMenu{
									CustomID:    "admin-edit-attendance-select-schedule",
									Placeholder: "일정 선택",
									Options:     selectOptions,
								},
							},
						},
					},
				},
			})

			if err != nil {
				panic(err)
			}
		case "admin-edit-attendance-select-schedule":
			selectMenuValues := i.MessageComponentData().Values
			if len(selectMenuValues) > 1 {
				return
			}

			scheduleID := selectMenuValues[0]

			// get schedule
			var schedule RaidSchedule
			err := rdb.Preload("Raid").First(&schedule, scheduleID).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return
			}

			// get attendees
			var attends []RaidAttend
			rdb.Preload("MemberInfo").Where("raid_schedule_id = ?", scheduleID).Find(&attends)

			// create user list
			var attendList []string
			for _, a := range attends {
				attendList = append(attendList, fmt.Sprintf("%s / %d / %s", a.MemberInfo.SubRoleName, a.Level, a.Nickname))
			}
			attendListStr := strings.Join(attendList, "\n")

			// send message
			err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseUpdateMessage,
				Data: &discordgo.InteractionResponseData{
					Content: fmt.Sprintf("**[%s] %s (%d 트라이) 참가자 목록**\n%s",
						schedule.Raid.RaidName,
						schedule.StartTime.Format("2006-01-02 15:04"),
						schedule.TryCount,
						attendListStr),
					Components: []discordgo.MessageComponent{
						discordgo.ActionsRow{
							Components: []discordgo.MessageComponent{
								discordgo.Button{
									Label:    "참가자 추가",
									Style:    discordgo.PrimaryButton,
									CustomID: "admin-edit-attendance-add_" + scheduleID,
								},
								discordgo.Button{
									Label:    "미편성 처리",
									Style:    discordgo.SecondaryButton,
									CustomID: "admin-edit-attendance-specout_" + scheduleID,
								},
								discordgo.Button{
									Label:    "참가자 삭제",
									Style:    discordgo.DangerButton,
									CustomID: "admin-edit-attendance-remove_" + scheduleID,
								},
							},
						},
					},
				},
			})

			if err != nil {
				panic(err)
			}
		case "user-attend-schedule":
			// get member
			memberInfo := cache.GetGuildMember(i.User.ID)
			if memberInfo == nil {
				return
			}
			m, err := GetMemberInfoFromMember(memberInfo)
			if err != nil {
				return
			}

			// list attend by user
			var attends []RaidAttend
			err = rdb.Where("nickname = ?", m.Nickname).Find(&attends).Error
			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return
			}

			var excludeRaidScheduleIDs []string
			for _, a := range attends {
				excludeRaidScheduleIDs = append(excludeRaidScheduleIDs, strconv.Itoa(int(a.RaidScheduleID)))
			}

			// list schedules
			var schedules []RaidSchedule
			rdb.Preload("Raid").Find(&schedules)

			// create selections
			var selectOptions []discordgo.SelectMenuOption
			for _, sc := range schedules {
				if slices.Contains(excludeRaidScheduleIDs, fmt.Sprintf("%d", sc.ID)) {
					continue
				}

				raidName := sc.Raid.RaidName
				raidStartTime := sc.StartTime.Format("2006-01-02 15:04")
				raidTryCount := sc.TryCount

				// subscriptionEndTime이 지나지 않은 스케줄만 보여주기
				if sc.SubscriptionEndTime.After(time.Now().In(loc)) {
					selectOptions = append(selectOptions, discordgo.SelectMenuOption{
						Label: fmt.Sprintf("[%s] %s (%d트라이)", raidName, raidStartTime, raidTryCount),
						Value: fmt.Sprintf("%d", sc.ID),
					})
				}
			}

			if len(selectOptions) == 0 {
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseUpdateMessage,
					Data: &discordgo.InteractionResponseData{
						Flags:   discordgo.MessageFlagsEphemeral,
						Content: "참가 할 수 있는 레이드 일정이 없습니다.\n참가신청 기한이 마감되었을 수 있으니 참가를 원하시면 공대장에게 문의해 주세요.",
					},
				})
				return
			}

			// send message
			err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseUpdateMessage,
				Data: &discordgo.InteractionResponseData{
					Content: "참가할 레이드 일정을 선택하세요.\n참가 신청 이전에 닉네임에 레벨이 최신화 되었는지 반드시 확인해주세요.",
					Components: []discordgo.MessageComponent{
						discordgo.ActionsRow{
							Components: []discordgo.MessageComponent{
								discordgo.SelectMenu{
									CustomID:    "user-attend-schedule-select-schedule",
									Placeholder: "일정 선택",
									Options:     selectOptions,
								},
							},
						},
					},
				},
			})

			if err != nil {
				panic(err)
			}
		case "user-attend-schedule-select-schedule":
			selectMenuValues := i.MessageComponentData().Values
			if len(selectMenuValues) > 1 {
				return
			}
			scheduleID := selectMenuValues[0]

			// get schedule
			var schedule RaidSchedule
			err := rdb.Preload("Raid").First(&schedule, scheduleID).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return
			}

			// get member info
			memberInfo := cache.GetGuildMember(i.User.ID)
			if memberInfo == nil {
				return
			}
			m, err := GetMemberInfoFromMember(memberInfo)
			if err != nil {
				return
			}

			// check if already attended
			var attend RaidAttend
			err = rdb.Where("nickname = ? AND raid_schedule_id = ?", m.Nickname, schedule.ID).First(&attend).Error
			if err == nil {
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseUpdateMessage,
					Data: &discordgo.InteractionResponseData{
						Flags: discordgo.MessageFlagsEphemeral,
						Content: fmt.Sprintf("[%s] %s (%d 트라이) 에 이미 참가하고 있습니다.",
							schedule.Raid.RaidName, schedule.StartTime.Format("2006-01-02 15:04"), schedule.TryCount),
					},
				})
				return
			}
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return
			}

			// create new attend
			newAttend := RaidAttend{
				MemberInfo:     *m,
				RaidScheduleID: schedule.ID,
				Canceled:       false,
			}
			rdb.Create(&newAttend)

			// send message
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseUpdateMessage,
				Data: &discordgo.InteractionResponseData{
					Content: fmt.Sprintf("[%s] %s (%d 트라이) 참가 신청이 완료되었습니다.",
						schedule.Raid.RaidName, schedule.StartTime.Format("2006-01-02 15:04"), schedule.TryCount),
				},
			})
		case "user-cancel-schedule":
			memberInfo := cache.GetGuildMember(i.User.ID)
			if memberInfo == nil {
				return
			}

			m, err := GetMemberInfoFromMember(memberInfo)
			if err != nil {
				return
			}

			// list attend
			var attends []RaidAttend
			rdb.Preload("RaidSchedule").Preload("RaidSchedule.Raid").Where("nickname = ?", m.Nickname).Find(&attends)

			var selectOptions []discordgo.SelectMenuOption
			for _, a := range attends {
				raidName := a.RaidSchedule.Raid.RaidName
				raidStartTime := a.RaidSchedule.StartTime.Format("2006-01-02 15:04")
				raidTryCount := a.RaidSchedule.TryCount

				// subscriptionEndTime이 지나지 않은 스케줄만 보여주기
				if a.RaidSchedule.SubscriptionEndTime.After(time.Now().In(loc)) {
					selectOptions = append(selectOptions, discordgo.SelectMenuOption{
						Label: fmt.Sprintf("[%s] %s (%d트라이)", raidName, raidStartTime, raidTryCount),
						Value: fmt.Sprintf("%d", a.ID),
					})
				}
			}

			if len(selectOptions) == 0 {
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseUpdateMessage,
					Data: &discordgo.InteractionResponseData{
						Flags:   discordgo.MessageFlagsEphemeral,
						Content: "참가 취소할 레이드 일정이 없습니다.",
					},
				})
				return
			}

			// send message
			err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseUpdateMessage,
				Data: &discordgo.InteractionResponseData{
					Content: "취소할 레이드 일정을 선택하세요.",
					Components: []discordgo.MessageComponent{
						discordgo.ActionsRow{
							Components: []discordgo.MessageComponent{
								discordgo.SelectMenu{
									CustomID:    "user-cancel-schedule-select-schedule",
									Placeholder: "일정 선택",
									Options:     selectOptions,
								},
							},
						},
					},
				},
			})

			if err != nil {
				panic(err)
			}
		case "user-cancel-schedule-select-schedule":
			selectMenuValues := i.MessageComponentData().Values
			if len(selectMenuValues) > 1 {
				return
			}
			attendID := selectMenuValues[0]

			// get attend
			var attend RaidAttend
			err := rdb.Preload("RaidSchedule").Preload("RaidSchedule.Raid").Where("id = ?", attendID).First(&attend).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseUpdateMessage,
					Data: &discordgo.InteractionResponseData{
						Flags: discordgo.MessageFlagsEphemeral,
						Content: fmt.Sprintf("[%s] %s (%d 트라이) 참가 신청 내역이 없습니다.",
							attend.RaidSchedule.Raid.RaidName, attend.RaidSchedule.StartTime.Format("2006-01-02 15:04"), attend.RaidSchedule.TryCount),
					},
				})
				return
			}

			// delete attend
			rdb.Delete(&attend)

			// send message
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseUpdateMessage,
				Data: &discordgo.InteractionResponseData{
					Content: fmt.Sprintf("[%s] %s (%d 트라이) 참가 신청이 취소되었습니다.",
						attend.RaidSchedule.Raid.RaidName, attend.RaidSchedule.StartTime.Format("2006-01-02 15:04"), attend.RaidSchedule.TryCount),
				},
			})
		case "admin-edit-attendance-remove-select-attendee":
			selectMenuValues := i.MessageComponentData().Values
			if len(selectMenuValues) > 1 {
				return
			}

			attendeeID := selectMenuValues[0]

			// get attendee
			var attend RaidAttend
			err := rdb.Preload("RaidSchedule").Preload("RaidSchedule.Raid").First(&attend, attendeeID).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseUpdateMessage,
					Data: &discordgo.InteractionResponseData{
						Content: "참가자 정보를 찾을 수 없습니다.",
						Flags:   discordgo.MessageFlagsEphemeral,
					},
				})
				return
			}
			if err != nil {
				return
			}

			// delete attendee
			rdb.Delete(&attend)

			// send message
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseUpdateMessage,
				Data: &discordgo.InteractionResponseData{
					Content: fmt.Sprintf("[%s] %s (%d 트라이)에서 참가자(%s)가 삭제되었습니다.",
						attend.RaidSchedule.Raid.RaidName, attend.RaidSchedule.StartTime.Format("2006-01-02 15:04"), attend.RaidSchedule.TryCount,
						attend.Nickname),
				},
			})
		case "admin-manage-info":
			// list schedules
			var schedules []RaidSchedule

			// startTime 기준 최신 25개 불러오기
			rdb.Order("start_time desc").Limit(25).Preload("Raid").Find(&schedules)

			// list schedules
			var selectOptions []discordgo.SelectMenuOption
			for _, sc := range schedules {
				raidName := sc.Raid.RaidName
				raidStartTime := sc.StartTime.Format("2006-01-02 15:04")
				raidTryCount := sc.TryCount

				selectOptions = append(selectOptions, discordgo.SelectMenuOption{
					Label: fmt.Sprintf("[%s] %s (%d트라이)", raidName, raidStartTime, raidTryCount),
					Value: fmt.Sprintf("%d", sc.ID),
				})
			}

			if len(selectOptions) == 0 {
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseUpdateMessage,
					Data: &discordgo.InteractionResponseData{
						Flags:   discordgo.MessageFlagsEphemeral,
						Content: "레이드 기록이 없습니다.",
					},
				})
				return
			}

			// send message
			err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseUpdateMessage,
				Data: &discordgo.InteractionResponseData{
					Content: "레이드 기록을 확인할 일정을 선택하세요.",
					Components: []discordgo.MessageComponent{
						discordgo.ActionsRow{
							Components: []discordgo.MessageComponent{
								discordgo.SelectMenu{
									CustomID:    "admin-manage-info-select-schedule",
									Placeholder: "일정 선택",
									Options:     selectOptions,
								},
							},
						},
					},
				},
			})

			if err != nil {
				panic(err)
			}
		case "admin-manage-info-select-schedule":
			selectMenuValues := i.MessageComponentData().Values
			if len(selectMenuValues) > 1 {
				return
			}

			scheduleID := selectMenuValues[0]

			// get schedule
			var schedule RaidSchedule
			err := rdb.Preload("Raid").First(&schedule, scheduleID).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return
			}

			// get attend
			var attends []RaidAttend
			rdb.Preload("MemberInfo").Where("raid_schedule_id = ? AND canceled = ?", scheduleID, false).Find(&attends)
			attendCount := len(attends)

			// get info
			var info RaidInfo
			err = rdb.Where("raid_schedule_id = ?", scheduleID).First(&info).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				info = RaidInfo{
					RaidScheduleID: schedule.ID,
				}
				rdb.Create(&info)
			}

			// send message
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseUpdateMessage,
				Data: &discordgo.InteractionResponseData{
					Content: fmt.Sprintf("**[%s] %s (%d 트라이) 레이드 기록**\n\n* 입장: %s\n* 시작: %s\n* 종료: %s\n* 참가자: %d명",
						schedule.Raid.RaidName, schedule.StartTime.Format("2006-01-02 15:04"), schedule.TryCount,
						info.EntranceTime.Format("2006-01-02 15:04"),
						info.StartTime.Format("2006-01-02 15:04"),
						info.EndTime.Format("2006-01-02 15:04"),
						attendCount,
					),
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
							},
						},
					},
				},
			})
		}
		return
	}

	switch args[0] {
	case "admin-add-schedule-select-raid":
		raidID := args[1]

		// get raid
		var raid Raid
		err := rdb.First(&raid, raidID).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return
		}

		// send message
		discord.SendNewRaidScheduleModal(s, i.Interaction, raidID)
	case "admin-remove-schedule-select-schedule":
		scheduleID := args[1]

		// get schedule
		var schedule RaidSchedule
		err := rdb.First(&schedule, scheduleID).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return
		}

		// delete schedule
		rdb.Delete(&schedule)

		// send message
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: fmt.Sprintf("레이드 일정이 삭제되었습니다."),
			},
		})
	case "admin-edit-attendance-remove":
		scheduleID := args[1]

		// get schedule
		var schedule RaidSchedule
		err := rdb.First(&schedule, scheduleID).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return
		}

		// get attendees
		var attends []RaidAttend
		rdb.Where("raid_schedule_id = ?", scheduleID).Find(&attends)

		// create selections
		var selectOptions []discordgo.SelectMenuOption
		for _, a := range attends {
			selectOptions = append(selectOptions, discordgo.SelectMenuOption{
				Label: fmt.Sprintf("%s / %d / %s", a.SubRoleName, a.Level, a.Nickname),
				Value: fmt.Sprintf("%d", a.ID),
			})
		}

		if len(selectOptions) == 0 {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseUpdateMessage,
				Data: &discordgo.InteractionResponseData{
					Content: "삭제할 참가자가 없습니다.",
				},
			})
			return
		}

		// send message
		err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseUpdateMessage,
			Data: &discordgo.InteractionResponseData{
				Content:  "삭제할 참가자를 선택하세요.",
				CustomID: "admin-edit-attendance-remove-select-attendee_" + scheduleID,
				Components: []discordgo.MessageComponent{
					discordgo.ActionsRow{
						Components: []discordgo.MessageComponent{
							discordgo.SelectMenu{
								CustomID:    "admin-edit-attendance-remove-select-attendee",
								Placeholder: "참가자 선택",
								Options:     selectOptions,
							},
						},
					},
				},
			},
		})
	case "admin-edit-attendance-add":
		scheduleID := args[1]
		discord.SendAdminAddAttendeeModal(s, i.Interaction, scheduleID)
	case "admin-info-record-entrance":
		infoID := args[1]

		// get info
		var info RaidInfo
		err := rdb.Preload("RaidSchedule").Preload("RaidSchedule.Raid").First(&info, infoID).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return
		}

		// get attend
		var attends []RaidAttend
		rdb.Where("raid_schedule_id = ? AND canceled = ?", info.RaidScheduleID, false).Find(&attends)
		attendCount := len(attends)

		// update info
		info.EntranceTime = time.Now().In(loc)
		rdb.Save(&info)

		// send message
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseUpdateMessage,
			Data: &discordgo.InteractionResponseData{
				Content: fmt.Sprintf("**[%s] %s (%d 트라이) 레이드 기록**\n\n* 입장: %s\n* 시작: %s\n* 종료: %s\n* 참가자: %d명",
					info.RaidSchedule.Raid.RaidName, info.RaidSchedule.StartTime.Format("2006-01-02 15:04"), info.RaidSchedule.TryCount,
					info.EntranceTime.Format("2006-01-02 15:04"),
					info.StartTime.Format("2006-01-02 15:04"),
					info.EndTime.Format("2006-01-02 15:04"),
					attendCount,
				),
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
						},
					},
				},
			},
		})
	case "admin-info-record-start":
		infoID := args[1]

		// get info
		var info RaidInfo
		err := rdb.Preload("RaidSchedule").Preload("RaidSchedule.Raid").First(&info, infoID).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return
		}

		// get attend
		var attends []RaidAttend
		rdb.Where("raid_schedule_id = ? AND canceled = ?", info.RaidScheduleID, false).Find(&attends)
		attendCount := len(attends)

		// update info
		info.StartTime = time.Now().In(loc)
		rdb.Save(&info)

		// send message
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseUpdateMessage,
			Data: &discordgo.InteractionResponseData{
				Content: fmt.Sprintf("**[%s] %s (%d 트라이) 레이드 기록**\n\n* 입장: %s\n* 시작: %s\n* 종료: %s\n* 참가자: %d명",
					info.RaidSchedule.Raid.RaidName, info.RaidSchedule.StartTime.Format("2006-01-02 15:04"), info.RaidSchedule.TryCount,
					info.EntranceTime.Format("2006-01-02 15:04"),
					info.StartTime.Format("2006-01-02 15:04"),
					info.EndTime.Format("2006-01-02 15:04"),
					attendCount,
				),
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
						},
					},
				},
			},
		})
	case "admin-info-record-end":
		infoID := args[1]

		// get info
		var info RaidInfo
		err := rdb.Preload("RaidSchedule").Preload("RaidSchedule.Raid").First(&info, infoID).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return
		}

		// get attend
		var attends []RaidAttend
		rdb.Where("raid_schedule_id = ? AND canceled = ?", info.RaidScheduleID, false).Find(&attends)
		attendCount := len(attends)

		// update info
		info.EndTime = time.Now().In(loc)
		rdb.Save(&info)

		// send message
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseUpdateMessage,
			Data: &discordgo.InteractionResponseData{
				Content: fmt.Sprintf("**[%s] %s (%d 트라이) 레이드 기록**\n\n* 입장: %s\n* 시작: %s\n* 종료: %s\n* 참가자: %d명",
					info.RaidSchedule.Raid.RaidName, info.RaidSchedule.StartTime.Format("2006-01-02 15:04"), info.RaidSchedule.TryCount,
					info.EntranceTime.Format("2006-01-02 15:04"),
					info.StartTime.Format("2006-01-02 15:04"),
					info.EndTime.Format("2006-01-02 15:04"),
					attendCount,
				),
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
						},
					},
				},
			},
		})
	}
}

func raidScheduleModalHandler(s *discordgo.Session, i *discordgo.InteractionCreate, modalID string) {
	modalIdSplit := strings.Split(modalID, "_")

	switch modalIdSplit[0] {
	case "add-raid-modal":
		modalData := i.ModalSubmitData().Components

		var raidName, raidType, raidDescription string
		for _, comp := range modalData {
			if ar, ok := comp.(*discordgo.ActionsRow); ok {
				if ti, ok := ar.Components[0].(*discordgo.TextInput); ok {
					if ti.CustomID == "raid-name" {
						raidName = ti.Value
					}
					if ti.CustomID == "raid-type" {
						raidType = ti.Value
					}
					if ti.CustomID == "raid-description" {
						raidDescription = ti.Value
					}
				}
			}
		}

		// find existing raid with raidName
		var raid Raid
		err := rdb.Where("raid_name = ?", raidName).First(&raid).Error
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return
		}

		// create new raid
		newRaid := Raid{
			RaidName:    raidName,
			Type:        raidType,
			Description: raidDescription,
		}
		rdb.Create(&newRaid)

		// send message
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: fmt.Sprintf("레이드 '%s'가 추가되었습니다.", raidName),
				Components: []discordgo.MessageComponent{
					discordgo.ActionsRow{
						Components: []discordgo.MessageComponent{
							discordgo.Button{
								Label:    "일정 추가로 돌아가기",
								Style:    discordgo.PrimaryButton,
								CustomID: "admin-add-schedule",
							},
						},
					},
				},
			},
		})
	case "add-raid-schedule-modal":
		modalData := i.ModalSubmitData().Components
		var startTime, subscriptionEndTime, tryCount string
		for _, comp := range modalData {
			if ar, ok := comp.(*discordgo.ActionsRow); ok {
				if ti, ok := ar.Components[0].(*discordgo.TextInput); ok {
					if ti.CustomID == "start-time" {
						startTime = ti.Value
					}
					if ti.CustomID == "try-count" {
						tryCount = ti.Value
					}
					if ti.CustomID == "subscription-end-time" {
						subscriptionEndTime = ti.Value
					}
				}
			}
		}

		// get raidID
		raidID := modalIdSplit[1]

		// get raid
		var raid Raid
		err := rdb.First(&raid, raidID).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return
		}

		// create new raid schedule
		raidCountInt, err := strconv.Atoi(tryCount)
		if err != nil {
			return
		}
		t, err := time.Parse("2006-01-02 15:04", startTime)
		if err != nil {
			return
		}
		te, err := time.Parse("2006-01-02 15:04", subscriptionEndTime)
		if err != nil {
			return
		}

		// create member attend message in channel
		m, err := s.ChannelMessageSend(environment.DiscordGuildRaidSubscriptionChannelID, fmt.Sprintf("**%s - %d트라이 (%s 출발)**",
			t.Format("01월 02일"), raidCountInt, t.Format("15:04")))
		if err != nil {
			return
		}

		newRaidSchedule := RaidSchedule{
			RaidID:              raid.ID,
			TryCount:            raidCountInt,
			StartTime:           t,
			SubscriptionEndTime: te,
			MessageID:           m.ID,
		}

		rdb.Create(&newRaidSchedule)

		// send message
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: fmt.Sprintf("레이드 일정이 추가되었습니다."),
			},
		})
	case "edit-raid-schedule-modal":
		modalData := i.ModalSubmitData().Components

		var startTime, tryCount string
		for _, comp := range modalData {
			if ar, ok := comp.(*discordgo.ActionsRow); ok {
				if ti, ok := ar.Components[0].(*discordgo.TextInput); ok {
					if ti.CustomID == "start-time" {
						startTime = ti.Value
					}
					if ti.CustomID == "try-count" {
						tryCount = ti.Value
					}
				}
			}
		}

		// get scheduleID
		scheduleID := modalIdSplit[1]

		// get schedule
		var schedule RaidSchedule
		err := rdb.First(&schedule, scheduleID).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return
		}

		// update schedule
		raidCountInt, err := strconv.Atoi(tryCount)
		if err != nil {
			return
		}

		t, err := time.Parse("2006-01-02 15:04", startTime)
		if err != nil {
			return
		}

		schedule.TryCount = raidCountInt
		schedule.StartTime = t
		rdb.Save(&schedule)

		// send message
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: fmt.Sprintf("레이드 일정이 수정되었습니다."),
			},
		})
	case "admin-add-attendee-modal":
		modalData := i.ModalSubmitData().Components

		var nickname string
		for _, comp := range modalData {
			if ar, ok := comp.(*discordgo.ActionsRow); ok {
				if ti, ok := ar.Components[0].(*discordgo.TextInput); ok {
					if ti.CustomID == "nickname" {
						nickname = ti.Value
					}
				}
			}
		}

		// get scheduleID
		scheduleID := modalIdSplit[1]

		// get user
		memberMap := cache.ListAllMembersNicknameMap()
		if _, ok := memberMap[nickname]; !ok {
			return
		}

		// get member
		m, err := GetMemberInfoFromMember(memberMap[nickname])
		if err != nil {
			return
		}

		// check schedule is valid
		var schedule RaidSchedule
		err = rdb.First(&schedule, scheduleID).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return
		}

		// create attend
		newAttend := RaidAttend{
			MemberInfo:     *m,
			RaidScheduleID: schedule.ID,
			Canceled:       false,
		}

		rdb.Create(&newAttend)

		// send message
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: fmt.Sprintf("참가자 '%s'가 추가되었습니다.", nickname),
				Components: []discordgo.MessageComponent{
					discordgo.ActionsRow{
						Components: []discordgo.MessageComponent{
							discordgo.Button{
								Label:    "참가자 추가로 돌아가기",
								Style:    discordgo.PrimaryButton,
								CustomID: "admin-edit-attendance",
							},
						},
					},
				},
			},
		})
	}
}

func raidScheduleHandler(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.Type == discordgo.InteractionApplicationCommand {
		data, ok := i.Data.(discordgo.ApplicationCommandInteractionData)
		if !ok {
			return
		}
		switch data.Name {
		case "레이드":
			if i.GuildID != "" {
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: "레이드 개인 일정 관리기능은 DM에서만 사용 가능합니다.",
						Flags:   discordgo.MessageFlagsEphemeral,
					},
				})
				return
			}

			// member check
			memberInfo := cache.GetGuildMember(i.User.ID)
			if memberInfo == nil {
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: "영원길드 멤버가 아닙니다.",
						Flags:   discordgo.MessageFlagsEphemeral,
					},
				})
				return
			}

			raidScheduleUserInitialHandler(s, i)
		case "레이드관리":
			if i.ChannelID != environment.DiscordGuildRaidManageChannelID {
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: "레이드 관리 기능은 관리 채널에서만 사용 가능합니다.",
						Flags:   discordgo.MessageFlagsEphemeral,
					},
				})
				return
			}
			raidScheduleAdminInitialHandler(s, i)
		}
	}

	if i.Type == discordgo.InteractionMessageComponent {
		data, ok := i.Data.(discordgo.MessageComponentInteractionData)
		if !ok {
			return
		}
		raidScheduleIntegratedHandler(s, i, data.CustomID)
	}

	if i.Type == discordgo.InteractionModalSubmit {
		data, ok := i.Data.(discordgo.ModalSubmitInteractionData)
		if !ok {
			return
		}
		raidScheduleModalHandler(s, i, data.CustomID)
	}
}
