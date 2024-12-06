package handler

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

func UpdateMessageWithRoles(s *discordgo.Session, guildID, channelID, messageID string) error {
	members, err := s.GuildMembers(guildID, "", 1000)
	if err != nil {
		return fmt.Errorf("failed to fetch guild members: %w", err)
	}

	roleList := []string{"용기사", "크루세이더", "나이트", "레인저", "저격수", "썬콜", "불독", "프리스트", "허밋", "시프마스터"}
	roleMembers := make(map[string][]string)
	newMembers := []string{}
	now := time.Now()

	for _, member := range members {
		mention := fmt.Sprintf("<@%s>", member.User.ID)
		if now.Sub(member.JoinedAt).Hours() <= 72 {
			newMembers = append(newMembers, mention)
		}

		for _, roleID := range member.Roles {
			roleMembers[roleID] = append(roleMembers[roleID], mention)
		}
	}

	var sb strings.Builder
	loc, _ := time.LoadLocation("Asia/Seoul")
	sb.WriteString(fmt.Sprintf("**[⚒️ 길드원 목록 ⚒️]** (%s 기준)\n", time.Now().In(loc).Format("2006-01-02 15:04:05")))
	memberCount := 0
	for roleID, mentions := range roleMembers {
		role, err := s.State.Role(guildID, roleID)
		if err != nil {
			return fmt.Errorf("failed to fetch role: %w", err)
		}
		if !slices.Contains(roleList, role.Name) {
			continue
		}
		memberCount = memberCount + len(mentions)
		sb.WriteString(fmt.Sprintf("\n**%s** (%d명): ", role.Name, len(mentions)))
		sb.WriteString(strings.Join(mentions, " "))
		sb.WriteString("\n")
	}

	//if len(newMembers) > 0 {
	//	sb.WriteString("\n**신규 멤버 (가입 3일 이내):**\n")
	//	sb.WriteString(strings.Join(newMembers, " "))
	//	sb.WriteString("\n")
	//}
	sb.WriteString(fmt.Sprintf("\n**[총 인원: %d명]**\n", memberCount))

	_, err = s.ChannelMessageEdit(channelID, messageID, sb.String())
	if err != nil {
		return fmt.Errorf("failed to edit message: %w", err)
	}

	return nil
}

type MemberInfo struct {
	RoleName string
	Level    int
	Nickname string
	Mention  string
}

func UpdateMessagesWithLevels(s *discordgo.Session, guildID, channelID, messageID string) error {
	members, err := s.GuildMembers(guildID, "", 1000)
	if err != nil {
		return fmt.Errorf("failed to fetch guild members: %w", err)
	}

	roleList := []string{"용기사", "크루세이더", "나이트", "레인저", "저격수", "썬콜", "불독", "프리스트", "허밋", "시프마스터"}
	roleMembers := make(map[string][]MemberInfo)

	for _, member := range members {
		for _, roleID := range member.Roles {
			role, err := s.State.Role(guildID, roleID)
			if err != nil {
				return fmt.Errorf("failed to fetch role: %w", err)
			}
			if !slices.Contains(roleList, role.Name) {
				continue
			}
			m, err := getMemberInfoFromMember(member, role.Name)
			if err != nil {
				return fmt.Errorf("failed to get member info: %w", err)
			}
			if m == nil {
				return fmt.Errorf("member info is nil")
			}
			roleMembers[role.Name] = append(roleMembers[role.Name], *m)
		}
	}

	return nil
}

func getMemberInfoFromMember(member *discordgo.Member, role string) (*MemberInfo, error) {
	// get username
	username := member.Nick

	// check if username is already generalized
	// if generalized, the name must be like:
	// Lv123 (username)

	// remove whitespaces
	newUsername := strings.ReplaceAll(username, " ", "")

	// remove dot
	newUsername = strings.ReplaceAll(newUsername, ".", "")

	// check if username starts with 'lv'
	if !strings.HasPrefix(newUsername, "lv") || !strings.HasPrefix(newUsername, "Lv") ||
		!strings.HasPrefix(newUsername, "LV") {
		return nil, fmt.Errorf("username is not started with 'lv': %s", username)
	}

	// check if the rest of the username is the combination of digits+string
	// possible level digit range: 1-200
	trim1 := strings.TrimPrefix(newUsername, "lv")
	trim2 := strings.TrimPrefix(trim1, "Lv")
	trim3 := strings.TrimPrefix(trim2, "LV")
	lv, nickname := extractLevelAndNickname(trim3)
	if lv == 0 {
		// cannot separate level and nickname
		// do nothing, but log error
		return nil, fmt.Errorf("cannot separate level and nickname: %s", username)
	}

	return &MemberInfo{
		RoleName: role,
		Level:    lv,
		Nickname: nickname,
		Mention:  fmt.Sprintf("<@%s>", member.User.ID),
	}, nil
}
