package handler

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
)

func GeneralizeUsername(s *discordgo.Session, guildID string) error {
	members, err := s.GuildMembers(guildID, "", 1000)
	if err != nil {
		return err
	}

	for _, member := range members {
		// filter user by role
		if !IsUserGamer(s, guildID, member.Roles) {
			continue
		}

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
		if strings.HasPrefix(newUsername, "lv") || strings.HasPrefix(newUsername, "Lv") || strings.HasPrefix(newUsername, "LV") {
			// check if the rest of the username is the combination of digits+string
			// possible level digit range: 1-200
			trim1 := strings.TrimPrefix(newUsername, "lv")
			trim2 := strings.TrimPrefix(trim1, "Lv")
			trim3 := strings.TrimPrefix(trim2, "LV")
			lv, nickname := extractLevelAndNickname(trim3)
			if lv == 0 {
				// cannot separate level and nickname
				// do nothing, but log error
				return fmt.Errorf("cannot separate level and nickname: %s", username)
			}

			// set nickname as generalized form
			newNickname := fmt.Sprintf("Lv %d %s", lv, nickname)
			if strings.ToLower(newNickname) == strings.ToLower(username) {
				fmt.Printf("Nickname is already generalized: %s\n", username)
				// do nothing
				continue
			}

			// update nickname
			err := s.GuildMemberNickname(guildID, member.User.ID, newNickname)
			if err != nil {
				return err
			}
		} else {
			// assume that user should set their nickname as policy
			dmchannel, err := s.UserChannelCreate(member.User.ID)
			if err != nil {
				return err
			}

			_, err = s.ChannelMessageSend(dmchannel.ID,
				fmt.Sprintf("안녕하세요. 메이플랜드 영원 길드 자동화 봇 입니다.\n길드 디스코드 내에서 서버 프로필 변경을 통해 닉네임을 인게임 레벨 닉네임으로 변경해 주세요.\n* 예시) `Lv 200 홍길동`"))
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func IsUserGamer(s *discordgo.Session, guildID string, roles []string) bool {
	rs, err := s.GuildRoles(guildID)
	if err != nil {
		return false
	}

	roleList := []string{"용기사", "크루세이더", "나이트", "레인저", "저격수", "썬콜", "불독", "프리스트", "허밋", "시프마스터"}
	for _, r := range rs {
		if slices.Contains(roleList, r.Name) {
			if slices.Contains(roles, r.ID) {
				return true
			}
		}
	}
	return false
}

func extractLevelAndNickname(text string) (int, string) {
	re := regexp.MustCompile(`^(\d{1,3})(.*)`)
	match := re.FindStringSubmatch(text)
	if len(match) > 0 {
		digits := match[1]
		nickname := match[2]
		for i := len(digits); i > 0; i-- {
			// try to convert the level digits and check if it's in the range
			levelNum, err := strconv.Atoi(digits[:i])
			if err == nil {
				if levelNum >= 85 && levelNum <= 200 {
					return levelNum, digits[i:] + nickname
				}
			}
		}
	}
	// no match
	return 0, text
}
