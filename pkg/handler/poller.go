package handler

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/sokdak/eternity-bot/pkg/cache"
	"github.com/sokdak/eternity-bot/pkg/environment"
	"github.com/sokdak/eternity-bot/pkg/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

var pdb *gorm.DB

type StringSlice []string

func (s *StringSlice) Scan(value interface{}) error {
	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("failed to convert %v to []byte", value)
	}
	return json.Unmarshal(bytes, s)
}

func (s StringSlice) Value() (driver.Value, error) {
	if len(s) == 0 {
		return nil, nil
	}
	return json.Marshal(s)
}

type Poll struct {
	gorm.Model
	Title        string
	Identifiable bool
	Targets      StringSlice `gorm:"type:TEXT"`
	Values       StringSlice `gorm:"type:TEXT"`
	Description  string
	Duration     int
	Closed       bool
	StartedAt    time.Time
}

type PollResult struct {
	gorm.Model
	DiscordUserID string
	Value         string
	PollID        uint
	Poll          Poll `gorm:"foreignKey:PollID"`
}

var loc, _ = time.LoadLocation("Asia/Seoul")

func PollerInit(dg *discordgo.Session) error {
	// setup gorm with sqlite
	g, err := gorm.Open(sqlite.Open(environment.PollSQLiteDBPath), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("failed to connect database: %w", err)
	}
	pdb = g

	if err := g.AutoMigrate(&Poll{}); err != nil {
		return fmt.Errorf("failed to migrate poll table: %w", err)
	}
	if err := g.AutoMigrate(&PollResult{}); err != nil {
		return fmt.Errorf("failed to migrate poll result table: %w", err)
	}

	// add handler for discordgo create message to watch user response
	dg.AddHandler(userDMPollHandler)
	dg.AddHandler(guildPollManageHandler)
	return nil
}

func PollerFinalize() {
	sqlDB, err := pdb.DB()
	if err != nil {
		fmt.Println("failed to get db connection for close: %w", err)
		return
	}
	_ = sqlDB.Close()
}

func userDMPollHandler(s *discordgo.Session, m *discordgo.MessageCreate) {
	// ignore messages from the bot
	if m.Author.ID == s.State.User.ID {
		return
	}

	// ignore messages from guild
	if m.GuildID != "" {
		return
	}

	// ignore direct message from user who is not in the guild
	member := cache.GetGuildMember(m.Author.ID)
	if member == nil {
		sendMessage(s, m.Author.ID, "길드에 가입하지 않은 유저는 사용할 수 없습니다.")
		return
	}

	if strings.HasPrefix(m.Content, "!투표 응답 ") {
		argsRaw := strings.TrimPrefix(m.Content, "!투표 응답 ")

		// parse argument but quoted string also should be considered
		// e.g. !투표 응답 1 1
		// should be parsed as ["1", "1"]
		args := parseArguments(argsRaw)
		if len(args) != 2 {
			sendMessage(s, m.Author.ID, "투표 응답 명령어 사용법이 잘못되었습니다.")
			return
		}

		// parse to int
		pollID, err := strconv.Atoi(args[0])
		if err != nil {
			sendMessage(s, m.Author.ID, "투표 번호가 올바르지 않습니다.")
			return
		}

		// check if the pollresult is already created
		var poll PollResult
		err = pdb.Where(&PollResult{PollID: uint(pollID), DiscordUserID: m.Author.ID}).First(&poll).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			sendMessage(s, m.Author.ID, "투표 응답 중 오류가 발생했습니다.")
			return
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			sendMessage(s, m.Author.ID, "이미 투표에 응답하셨습니다.")
			return
		}

		// get poll
		var p Poll
		if err := pdb.Where("id = ?", pollID).First(&p).Error; err != nil {
			sendMessage(s, m.Author.ID, "해당 투표를 찾을 수 없거나 이미 종료된 투표입니다.")
			return
		}

		// check if poll is started
		if p.StartedAt.IsZero() {
			sendMessage(s, m.Author.ID, "해당 투표를 찾을 수 없거나 이미 종료된 투표입니다.")
			return
		}

		// check if poll is expired
		if time.Now().In(loc).After(p.StartedAt.Add(time.Duration(p.Duration) * time.Hour)) {
			sendMessage(s, m.Author.ID, "해당 투표를 찾을 수 없거나 이미 종료된 투표입니다.")
			return
		}

		// check if valueIndex is valid
		valueIndex, err := strconv.Atoi(args[1])
		if err != nil || (valueIndex < 1 || valueIndex > len(p.Values)) {
			sendMessage(s, m.Author.ID, "응답 번호가 올바르지 않습니다.")
			return
		}

		// create poll result
		poll = PollResult{
			DiscordUserID: m.Author.ID,
			Value:         p.Values[valueIndex-1],
			PollID:        uint(pollID),
		}
		if err := pdb.Create(&poll).Error; err != nil {
			sendMessage(s, m.Author.ID, "투표 응답 중 오류가 발생했습니다.")
			return
		}
		sendMessage(s, m.Author.ID, fmt.Sprintf("투표에 '%s' 선택지로 응답하셨습니다. 감사합니다.", p.Values[valueIndex-1]))
	}
}

func guildPollManageHandler(s *discordgo.Session, m *discordgo.MessageCreate) {
	// ignore messages from the bot
	if m.Author.ID == s.State.User.ID {
		return
	}

	// ignore messages from DM
	if m.GuildID == "" {
		return
	}

	// ignore messages from guild that is not the target guild
	if m.GuildID != environment.DiscordGuildID {
		return
	}

	// ignore messages from guild that is not a guild channel
	if m.ChannelID != environment.DiscordGuildPollChannelID {
		return
	}

	if strings.HasPrefix(m.Content, "!투표 생성 ") {
		argsRaw := strings.TrimPrefix(m.Content, "!투표 생성 ")

		// parse argument but quoted string also should be considered
		// e.g. !투표 생성 무기명 전체 "투표 제목" 네,아니오,모르겠음 1
		// should be parsed as ["무기명", "전체", "투표 제목", "네,아니오,모르겠음", "1"]
		args := parseArguments(argsRaw)
		if len(args) != 5 {
			sendGuildMessage(s, m.ChannelID, "투표 생성 명령어 사용법이 잘못되었습니다.")
			return
		}

		// check if the poll is already created
		var poll Poll
		if err := pdb.Where("title = ?", args[2]).First(&poll).Error; err == nil {
			sendGuildMessage(s, m.ChannelID, "이미 생성된 투표가 있습니다.")
			return
		}

		// parse to int
		duration, err := strconv.Atoi(args[4])
		if err != nil {
			sendGuildMessage(s, m.ChannelID, "투표 기간이 올바르지 않습니다.")
			return
		}

		// create poll
		poll = Poll{
			Title:        args[2],
			Identifiable: args[0] == "기명",
			Targets:      strings.Split(args[1], ","),
			Values:       strings.Split(args[3], ","),
			Description:  "",
			Duration:     duration,
		}
		if err := pdb.Create(&poll).Error; err != nil {
			sendGuildMessage(s, m.ChannelID, "투표 생성 중 오류가 발생했습니다.")
			return
		}
		sendGuildMessage(s, m.ChannelID, fmt.Sprintf("투표('%s')가 생성되었습니다.", poll.Title))
	} else if strings.HasPrefix(m.Content, "!투표 시작 ") {
		argsRaw := strings.TrimPrefix(m.Content, "!투표 시작 ")
		args := parseArguments(argsRaw)

		if len(args) != 1 {
			sendGuildMessage(s, m.ChannelID, "투표 시작 명령어 사용법이 잘못되었습니다.")
			return
		}

		var poll Poll
		if err := pdb.Where("title = ?", args[0]).First(&poll).Error; err != nil {
			sendGuildMessage(s, m.ChannelID, "해당 투표를 찾을 수 없습니다.")
			return
		}

		if !poll.StartedAt.IsZero() {
			sendGuildMessage(s, m.ChannelID, "이미 시작된 투표입니다.")
			return
		}

		poll.StartedAt = time.Now().In(loc)
		if err := pdb.Save(&poll).Error; err != nil {
			sendGuildMessage(s, m.ChannelID, "투표 시작 중 오류가 발생했습니다.")
			return
		}
		sendGuildMessage(s, m.ChannelID, fmt.Sprintf("투표('%s')가 시작되었습니다.", poll.Title))

		// send polls
		sendPolls(s, m.ChannelID, poll)
	} else if strings.HasPrefix(m.Content, "!투표 종료 ") {
		sendGuildMessage(s, m.ChannelID, "투표 강제 종료 기능은 아직 지원하지 않습니다.")
		//argsRaw := strings.TrimPrefix(m.Content, "!투표 종료 ")
		//args := parseArguments(argsRaw)
		//
		//if len(args) != 1 {
		//	sendGuildMessage(s, m.ChannelID, "투표 종료 명령어 사용법이 잘못되었습니다.")
		//	return
		//}
		//
		//var poll Poll
		//if err := pdb.Where("title = ?", args[0]).First(&poll).Error; err != nil {
		//	sendGuildMessage(s, m.ChannelID, "해당 투표를 찾을 수 없습니다.")
		//	return
		//}
		//
		//if poll.StartedAt.IsZero() {
		//	sendGuildMessage(s, m.ChannelID, "투표가 시작되지 않았습니다.")
		//	return
		//}
		//
		//if time.Now().After(poll.StartedAt.Add(time.Duration(poll.Duration) * time.Hour)) {
		//	sendGuildMessage(s, m.ChannelID, "투표가 이미 종료되었습니다.")
		//	return
		//}
	} else if strings.HasPrefix(m.Content, "!투표 설명 ") {
		hdescs := strings.SplitN(m.Content, "\n", 2)
		if len(hdescs) != 2 {
			sendGuildMessage(s, m.ChannelID, "투표 설명 명령어 사용법이 잘못되었습니다.")
			return
		}

		argsRaw := strings.TrimPrefix(hdescs[0], "!투표 설명 ")
		args := parseArguments(argsRaw)

		if len(args) != 1 {
			sendGuildMessage(s, m.ChannelID, "투표 설명 명령어 사용법이 잘못되었습니다.")
			return
		}

		var poll Poll
		if err := pdb.Where("title = ?", args[0]).First(&poll).Error; err != nil {
			sendGuildMessage(s, m.ChannelID, "해당 투표를 찾을 수 없습니다.")
			return
		}

		if !poll.StartedAt.IsZero() {
			sendGuildMessage(s, m.ChannelID, "투표가 이미 시작되었습니다.")
			return
		}

		poll.Description = hdescs[1]
		if err := pdb.Save(&poll).Error; err != nil {
			sendGuildMessage(s, m.ChannelID, "투표 설명을 저장하는 중 오류가 발생했습니다.")
			return
		}
		sendGuildMessage(s, m.ChannelID, fmt.Sprintf("투표('%s')에 설명이 추가되었습니다.", poll.Title))
	} else if strings.HasPrefix(m.Content, "!투표 재발송 ") {
		argsRaw := strings.TrimPrefix(m.Content, "!투표 재발송 ")
		args := parseArguments(argsRaw)

		if len(args) != 1 {
			sendGuildMessage(s, m.ChannelID, "투표 재발송 명령어 사용법이 잘못되었습니다.")
			return
		}

		var poll Poll
		if err := pdb.Where("title = ?", args[0]).First(&poll).Error; err != nil {
			sendGuildMessage(s, m.ChannelID, "해당 투표를 찾을 수 없습니다.")
			return
		}

		if poll.StartedAt.IsZero() {
			sendGuildMessage(s, m.ChannelID, "투표가 시작되지 않았습니다.")
			return
		}

		if time.Now().In(loc).After(poll.StartedAt.Add(time.Duration(poll.Duration) * time.Hour)) {
			sendGuildMessage(s, m.ChannelID, "투표가 이미 종료되었습니다.")
			return
		}

		// send polls
		sendPolls(s, m.ChannelID, poll)
		sendGuildMessage(s, m.ChannelID, fmt.Sprintf("투표('%s') 알림이 재발송되었습니다.", poll.Title))
	} else if strings.HasPrefix(m.Content, "!투표 목록") {
		polls := []Poll{}
		if err := pdb.Find(&polls).Error; err != nil {
			sendGuildMessage(s, m.ChannelID, "투표 목록을 가져오는 중 오류가 발생했습니다.")
			return
		}

		pollMap := map[string][]Poll{}
		for _, poll := range polls {
			if poll.StartedAt.IsZero() {
				pollMap["inactive"] = append(pollMap["inactive"], poll)
				continue
			} else {
				// if poll is expired, append to expired
				if time.Now().In(loc).After(poll.StartedAt.Add(time.Duration(poll.Duration)*time.Hour)) ||
					poll.Closed {
					pollMap["expired"] = append(pollMap["expired"], poll)
					continue
				}
				pollMap["active"] = append(pollMap["active"], poll)
			}
		}

		msg := "**[현재 진행중인 투표 목록]**\n"
		for _, p := range pollMap["active"] {
			id := "무기명"
			if p.Identifiable {
				id = "기명"
			}
			if p.StartedAt.IsZero() {
				msg += fmt.Sprintf("* [%s] '%s' - 시작 전 (응답 기한 %d시간)\n", id, p.Title, p.Duration)
				continue
			}
			msg += fmt.Sprintf("* [%s] '%s' - %s 까지 (응답 기한 %d시간 남음)\n",
				id,
				p.Title, p.StartedAt.Add(time.Duration(p.Duration)*time.Hour).In(loc).Format("2006-01-02 15:04:05"),
				int(time.Until(p.StartedAt.Add(time.Duration(p.Duration)*time.Hour)).Hours()))
		}
		if len(pollMap["active"]) == 0 {
			msg += "* 없음\n"
		}

		msg += "\n**[아직 시작하지 않은 투표 목록]**\n"
		for _, p := range pollMap["inactive"] {
			id := "무기명"
			if p.Identifiable {
				id = "기명"
			}
			msg += fmt.Sprintf("* [%s] '%s' - 시작 전 (응답 기한 %d시간)\n", id, p.Title, p.Duration)
		}
		if len(pollMap["inactive"]) == 0 {
			msg += "* 없음\n"
		}

		msg += "\n**[종료된 투표 목록]**\n"
		for _, p := range pollMap["expired"] {
			id := "무기명"
			if p.Identifiable {
				id = "기명"
			}
			msg += fmt.Sprintf("* [%s] '%s' - %s\n", id, p.Title, p.StartedAt.Add(time.Duration(p.Duration)*time.Hour).In(loc).Format("2006-01-02 15:04:05"))
		}
		if len(pollMap["expired"]) == 0 {
			msg += "* 없음\n"
		}

		sendGuildMessage(s, m.ChannelID, msg)
	} else if strings.HasPrefix(m.Content, "!투표 결과 ") {
		argsRaw := strings.TrimPrefix(m.Content, "!투표 결과 ")
		args := parseArguments(argsRaw)

		if len(args) != 1 {
			sendGuildMessage(s, m.ChannelID, "투표 결과 명령어 사용법이 잘못되었습니다.")
			return
		}

		var poll Poll
		if err := pdb.Where("title = ?", args[0]).First(&poll).Error; err != nil {
			sendGuildMessage(s, m.ChannelID, "해당 투표를 찾을 수 없습니다.")
			return
		}

		if poll.StartedAt.IsZero() {
			sendGuildMessage(s, m.ChannelID, "투표가 시작되지 않았습니다.")
			return
		}

		if !poll.Closed && time.Now().In(loc).Before(poll.StartedAt.Add(time.Duration(poll.Duration)*time.Hour)) {
			sendGuildMessage(s, m.ChannelID, "투표가 아직 종료되지 않았습니다.")
			return
		}

		// get poll results
		results := []PollResult{}
		if err := pdb.Where("poll_id = ?", poll.ID).Find(&results).Error; err != nil {
			sendGuildMessage(s, m.ChannelID, "투표 결과를 가져오는 중 오류가 발생했습니다.")
			return
		}

		// print results
		if err := printPollResult(s, poll, results); err != nil {
			return
		}
	} else if strings.HasPrefix(m.Content, "!투표 정보 ") {
		argsRaw := strings.TrimPrefix(m.Content, "!투표 정보 ")
		args := parseArguments(argsRaw)

		if len(args) != 1 {
			sendGuildMessage(s, m.ChannelID, "투표 정보 명령어 사용법이 잘못되었습니다.")
			return
		}

		var poll Poll
		if err := pdb.Where("title = ?", args[0]).First(&poll).Error; err != nil {
			sendGuildMessage(s, m.ChannelID, "해당 투표를 찾을 수 없습니다.")
			return
		}

		id := "무기명"
		if poll.Identifiable {
			id = "기명"
		}

		msg := fmt.Sprintf("**[투표 정보: '%s']**\n", poll.Title)
		msg += fmt.Sprintf("* 투표 번호: %d\n", poll.ID)
		msg += fmt.Sprintf("* 투표 대상: %s\n", strings.Join(poll.Targets, "/"))
		msg += fmt.Sprintf("* 투표 제목: %s\n", poll.Title)
		msg += fmt.Sprintf("* 투표 종류: %s\n", id)
		msg += fmt.Sprintf("* 투표 기간: %d시간 (%s 까지)\n", poll.Duration,
			poll.StartedAt.Add(time.Duration(poll.Duration)*time.Hour).In(loc).Format("2006-01-02 15:04"))
		msg += "* 투표 선택지:\n"
		for i, value := range poll.Values {
			msg += fmt.Sprintf("  %d. %s\n", i+1, value)
		}
		msg += fmt.Sprintf("---\n[투표 설명]\n%s", poll.Description)
		sendGuildMessage(s, m.ChannelID, msg)
	} else {
		help := fmt.Sprintf(`
!투표 도움말
투표를 생성하고 관리하는 명령어입니다.
투표 종료는 기간이 도래하거나 모든 인원이 투표 참여에 완료하면 자동으로 종료되고, 결과가 채널에 나타납니다.

사용법:
* !투표 생성 [기명/무기명] [전체/직업군(다수 직업군은 ,로 구분)/u_메랜닉네임] [투표 제목] [선택지(,로 구분)] [투표 기간]
  * 투표 생성이 접수되면, 투표 내용을 입력할 수 있습니다.
  * 투표 내용을 입력받고 나면 투표 정보가 맞는지 확인한 뒤 투표를 시작할 수 있습니다.
  * 투표 기간은 1시간부터 168시간(7일)까지 설정할 수 있습니다.
* !투표 설명 [투표 이름]\n[투표 설명]
  * 투표 설명을 추가합니다.
  * 투표가 시작되기 전에 설명을 추가 할 수 있습니다.
* !투표 시작 [투표 이름]
  * 투표를 시작합니다.
* !투표 재발송 [투표 이름]
  * 투표를 다시 발송합니다.
* !투표 정보 [투표 이름]
  * 투표 정보를 확인합니다.
* !투표 종료 [투표 이름]
  * 투표를 종료하고 결과를 발표합니다.
  * 기능 미구현으로 투표 강제 종료가 불가능합니다.
* !투표 목록
  * 현재 진행 중인 투표 목록을 확인합니다.
* !투표 결과 [투표 이름]
  * 종료된 투표의 현황을 확인합니다.
`)
		sendGuildMessage(s, m.ChannelID, help)
		return
	}
}

func sendPolls(s *discordgo.Session, guildBotManageChannelID string, poll Poll) {
	roleNameIdMap := cache.ListAllRoles()
	userMap := cache.ListAllMembersNicknameMap()
	targetDgMembers, err := filterPollTarget(poll, roleNameIdMap, userMap)
	if err != nil {
		sendGuildMessage(s, guildBotManageChannelID, err.Error())
		return
	}

	// filter that only not voted members
	var votedMembers []string
	var pollResults []PollResult
	if err := pdb.Where("poll_id = ?", poll.ID).Find(&pollResults).Error; err != nil {
		sendGuildMessage(s, guildBotManageChannelID, "투표 결과를 가져오는 중 오류가 발생했습니다.")
		return
	}
	for _, r := range pollResults {
		votedMembers = append(votedMembers, r.DiscordUserID)
	}
	for k, v := range targetDgMembers {
		if slices.Contains(votedMembers, v.User.ID) {
			delete(targetDgMembers, k)
		}
	}

	// send dm to each target
	identifiableStr := "무기명 (익명, 선택지 통계만 확인가능)"
	if poll.Identifiable {
		identifiableStr = "기명 (직업, 레벨, 닉네임)"
	}

	// redact the poll target if it specifies a user
	redactedUserTargets := []string{}
	specifiedUserCount := 0
	for i := 0; i < len(poll.Targets); i++ {
		if strings.HasPrefix(poll.Targets[i], "u_") {
			specifiedUserCount++
		} else {
			redactedUserTargets = append(redactedUserTargets, poll.Targets[i])
		}
	}
	if specifiedUserCount > 0 {
		if len(redactedUserTargets) > 0 {
			redactedUserTargets = append(redactedUserTargets, fmt.Sprintf("외 특정 길드원 %d명", specifiedUserCount))
		} else {
			redactedUserTargets = append(redactedUserTargets, fmt.Sprintf("특정 길드원 %d명", specifiedUserCount))
		}
	}

	nicks := []string{}
	for nickname, v := range targetDgMembers {
		nicks = append(nicks, nickname)
		msg := fmt.Sprintf("**[메이플랜드 영원 길드 - 투표 시스템 알림]**\n")
		msg += fmt.Sprintf("안녕하세요, %s 님! 메이플랜드 영원 길드 봇 입니다.\n", nickname)
		msg += fmt.Sprintf("길드 운영 계획에 있어 %s 님의 의견이 필요하여 투표를 요청드리게 되었습니다.\n", nickname)
		msg += fmt.Sprintf("```* 투표 번호: %d\n", poll.ID)
		msg += fmt.Sprintf("* 투표 대상: %s\n", strings.Join(redactedUserTargets, "/"))
		msg += fmt.Sprintf("* 투표 제목: %s\n", poll.Title)
		msg += fmt.Sprintf("* 투표 종류: %s\n", identifiableStr)
		msg += fmt.Sprintf("* 투표 기간: %d시간 (%s 까지)\n", poll.Duration,
			poll.StartedAt.Add(time.Duration(poll.Duration)*time.Hour).Format("2006-01-02 15:04"))
		msg += fmt.Sprintf("* 투표 선택지:\n")
		for i, value := range poll.Values {
			msg += fmt.Sprintf("  %d. %s\n", i+1, value)
		}
		msg += fmt.Sprintf("---\n[투표 설명]\n%s\n", poll.Description)
		msg += "```"
		msg += fmt.Sprintf("\n!투표 응답 [투표번호] [투표선택지] 를 입력하여 투표를 진행해 주세요.\n")
		msg += fmt.Sprintf("입력 예시:\n")
		for i := 0; i < len(poll.Values); i++ {
			msg += fmt.Sprintf("* `!투표 응답 %d %d`: %s\n", poll.ID, i+1, poll.Values[i])
		}
		msg += fmt.Sprintf("\n%s 님의 소중한 의견이 길드 운영에 큰 도움이 됩니다.", nickname)
		sendMessage(s, v.User.ID, msg)
	}
	if poll.Identifiable {
		sendGuildMessage(s, guildBotManageChannelID, "투표 알림이 다음 인원에게 발송되었습니다: "+strings.Join(nicks, ", "))
	} else {
		sendGuildMessage(s, guildBotManageChannelID, fmt.Sprintf("투표 알림이 다음 인원에게 발송되었습니다. (%d 명)", len(nicks)))
	}
}

func filterPollTarget(poll Poll, roleNameIdMap map[string]string,
	userMap map[string]*discordgo.Member) (map[string]*discordgo.Member, error) {
	targetDgMembers := map[string]*discordgo.Member{}
	for _, target := range poll.Targets {
		if target == "전체" {
			for k, v := range userMap {
				if v.User.Bot {
					continue
				}
				targetDgMembers[k] = v
			}
			break
		}

		// check if target is a user
		if strings.HasPrefix(target, "u_") {
			target = strings.TrimPrefix(target, "u_")
			if _, ok := userMap[target]; !ok {
				return nil, fmt.Errorf("'%s' 님을 찾을 수 없습니다", target)
			}
			targetDgMembers[target] = userMap[target]
			continue
		}

		// check if target is a role
		if _, ok := roleNameIdMap[target]; !ok {
			return nil, fmt.Errorf("'%s' 직업을 찾을 수 없습니다", target)
		}

		// get members by role from previously built map
		for k, v := range userMap {
			if v.User.Bot {
				continue
			}
			if slices.Contains(v.Roles, roleNameIdMap[target]) {
				targetDgMembers[k] = v
			}
		}
	}
	return targetDgMembers, nil
}

func PollFinishChecker(dg *discordgo.Session) error {
	// check if poll is finished
	polls := []Poll{}
	if err := pdb.Find(&polls).Error; err != nil {
		return fmt.Errorf("failed to get polls: %w", err)
	}

	for _, poll := range polls {
		if poll.StartedAt.IsZero() {
			continue
		}

		if poll.Closed {
			continue
		}

		// check if poll is expired
		if time.Now().In(loc).After(poll.StartedAt.Add(time.Duration(poll.Duration) * time.Hour)) {
			// get poll results
			results := []PollResult{}
			if err := pdb.Where("poll_id = ?", poll.ID).Find(&results).Error; err != nil {
				return fmt.Errorf("failed to get poll results: %w", err)
			}

			// print results
			if err := printPollResult(dg, poll, results); err != nil {
				return fmt.Errorf("failed to print poll result: %w", err)
			}

			// update poll
			poll.Closed = true
			if err := pdb.Save(&poll).Error; err != nil {
				return fmt.Errorf("failed to save poll: %w", err)
			}
			sendGuildMessage(dg, environment.DiscordGuildPollChannelID, fmt.Sprintf("투표('%s')가 기간 도래로 종료되었습니다.", poll.Title))
		}

		// check if all members voted
		roleNameIdMap := cache.ListAllRoles()
		userMap := cache.ListAllMembersNicknameMap()
		targetDgMembers, err := filterPollTarget(poll, roleNameIdMap, userMap)
		if err != nil {
			return fmt.Errorf("failed to filter poll target: %w", err)
		}

		// filter that only not voted members
		var votedMembers []string
		var pollResults []PollResult
		if err := pdb.Where("poll_id = ?", poll.ID).Find(&pollResults).Error; err != nil {
			return fmt.Errorf("failed to get poll results: %w", err)
		}
		for _, r := range pollResults {
			votedMembers = append(votedMembers, r.DiscordUserID)
		}

		for k, v := range targetDgMembers {
			if slices.Contains(votedMembers, v.User.ID) {
				delete(targetDgMembers, k)
			}
		}

		// if there are members who did not vote, continue
		if len(targetDgMembers) != 0 {
			continue
		}

		// get poll results
		results := []PollResult{}
		if err := pdb.Where("poll_id = ?", poll.ID).Find(&results).Error; err != nil {
			return fmt.Errorf("failed to get poll results: %w", err)
		}

		if err := printPollResult(dg, poll, results); err != nil {
			return fmt.Errorf("failed to print poll result: %w", err)
		}

		// update poll
		poll.Closed = true
		if err := pdb.Save(&poll).Error; err != nil {
			return fmt.Errorf("failed to save poll: %w", err)
		}
		sendGuildMessage(dg, environment.DiscordGuildPollChannelID, fmt.Sprintf("투표('%s')가 전원 투표로 조기 종료되었습니다.", poll.Title))
	}

	return nil
}

func printPollResult(dg *discordgo.Session, poll Poll, results []PollResult) error {
	// count results
	counts := map[string]int{}
	for _, r := range results {
		counts[r.Value]++
	}

	// query users
	pollMemberMap := getPollMemberMap(results)
	if pollMemberMap == nil {
		return fmt.Errorf("failed to get identifiable users")
	}

	//// remove poll members
	//for value, members := range pollMemberMap {
	//	v := []string{"가샤", "이잉몽실이", "전전궁붕", "o1dboy", "보라빛향기", "돔적", "머미킴", "jun두사", "느그바램", "Only뮌헨"}
	//	filtered := make([]MemberInfo, 0, len(members))
	//	for _, member := range members {
	//		if !slices.Contains(v, member.Nickname) {
	//			filtered = append(filtered, member)
	//		}
	//	}
	//	pollMemberMap[value] = filtered
	//}

	// print results
	msg := fmt.Sprintf("**[투표 결과: '%s']**\n", poll.Title)
	msg += fmt.Sprintf("* 참여자: %d명\n", len(results))
	for _, value := range poll.Values {
		if !poll.Identifiable {
			statisticsMsg := ""
			if len(pollMemberMap[value]) > 0 {
				// sum up the main role number
				roleCounts := map[string]int{}
				for _, member := range pollMemberMap[value] {
					roleCounts[member.MainRoleName]++
				}
				statisticsMsg += "  * 직업군 통계:"
				for role, count := range roleCounts {
					statisticsMsg += fmt.Sprintf(" %s(%d명)", role, count)
				}
			}
			msg += fmt.Sprintf("* %s: %d\n", value, counts[value])
			msg += fmt.Sprintf("%s\n", statisticsMsg)
		} else {
			nicks := []string{}
			for _, member := range pollMemberMap[value] {
				nicks = append(nicks, fmt.Sprintf("%s/%s", member.Nickname, member.SubRoleName))
			}
			msg += fmt.Sprintf("* %s: %d", value, counts[value])
			msg += fmt.Sprintf("  * %s\n", strings.Join(nicks, ", "))
		}
	}

	sendGuildMessage(dg, environment.DiscordGuildPollChannelID, msg)
	return nil
}

func getPollMemberMap(results []PollResult) map[string][]model.MemberInfo {
	var valueUserMap = map[string][]model.MemberInfo{}
	for _, r := range results {
		m := cache.GetGuildMember(r.DiscordUserID)
		if m == nil {
			fmt.Printf("not existing guild member: %s\n", r.DiscordUserID)
			continue
		}
		member, err := GetMemberInfoFromMember(m)
		if err != nil {
			return nil
		}
		if member == nil {
			return nil
		}
		valueUserMap[r.Value] = append(valueUserMap[r.Value], *member)
	}
	return valueUserMap
}

func sendMessage(dg *discordgo.Session, userID, message string) {
	c, err := dg.UserChannelCreate(userID)
	if err != nil {
		fmt.Println("failed to create user channel: %w", err)
		return
	}
	if len(message) > 2000 {
		_ = sendSplitMessage(dg, c.ID, message)
		return
	}
	_, _ = dg.ChannelMessageSend(c.ID, message)
}

func sendSplitMessage(s *discordgo.Session, channelID, content string) error {
	lines := strings.Split(content, "\n")

	var sb strings.Builder // 현재 메시지 조각을 누적할 버퍼
	inCodeBlock := false   // 코드 블록(```)이 현재 열려 있는지 여부 추적

	// chunk(조각)를 실제로 Discord에 전송하는 함수
	sendChunk := func() error {
		if sb.Len() == 0 {
			return nil
		}

		// 만약 코드 블록이 열려 있는 상태라면, 현 메시지의 끝에서 먼저 닫아 준다
		msg := sb.String()
		if inCodeBlock {
			msg += "\n```"
		}

		_, err := s.ChannelMessageSend(channelID, msg)
		if err != nil {
			return err
		}

		// 전송 후, sb를 비워 준다
		sb.Reset()

		// 방금까지 코드 블록이 열려 있었다면, 새 메시지의 시작에서 다시 ```로 열어 준다
		if inCodeBlock {
			sb.WriteString("```")
		}

		return nil
	}

	for i, line := range lines {
		// 이 라인을 추가했을 때 2000자 제한을 넘는지 미리 확인
		// 코드 블록이 열려 있으면, 메시지 끝에서 \n``` 를 추가해야 하므로 그 길이도 약간 고려
		// (간단화를 위해 대략적인 계산만 수행)
		overhead := 0
		if inCodeBlock {
			// 끝에서 ```(3) + \n(1) = 4글자 추가로 들어갈 수 있으니 여유 공간을 생각
			overhead += 4
		}

		// +1은 현재 버퍼에 들어갈 newline(\n) 용도
		if sb.Len()+len(line)+1+overhead > 2000 {
			// 이미 채워 놓은 chunk를 먼저 보낸다
			if err := sendChunk(); err != nil {
				return err
			}
		}

		// 보낼 chunk에 현재 줄을 추가
		// (이미 무언가 있다면 줄바꿈 후 추가)
		if sb.Len() > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(line)

		// 현재 줄(line)에 ```가 몇 번 등장하는지 찾아서, 그만큼 코드 블록 상태를 토글
		count := strings.Count(line, "```")
		for c := 0; c < count; c++ {
			inCodeBlock = !inCodeBlock
		}

		// 마지막 줄이라면, 남은 chunk를 최종 전송
		if i == len(lines)-1 {
			if err := sendChunk(); err != nil {
				return err
			}
		}
	}

	return nil
}

func sendGuildMessage(dg *discordgo.Session, channelID, message string) {
	_, _ = dg.ChannelMessageSend(channelID, message)
}

func parseArguments(input string) []string {
	// Regular expression to match quoted strings or non-whitespace sequences
	re := regexp.MustCompile(`"([^"]*)"|(\S+)`)
	matches := re.FindAllStringSubmatch(input, -1)

	var args []string
	for _, match := range matches {
		if match[1] != "" {
			args = append(args, match[1])
		} else {
			args = append(args, match[2])
		}
	}
	return args
}
