package handler

import (
	"fmt"
	"github.com/bwmarrin/discordgo"
	"slices"
	"strings"
	"time"
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
	sb.WriteString(fmt.Sprintf("**[⚒️ 길드원 목록 ⚒️]** (%s 기준)\n", time.Now().Format("2006-01-02 15:04:05")))
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
