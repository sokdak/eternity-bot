package handler

import (
	"fmt"
	"github.com/sokdak/eternity-bot/pkg/cache"
	"strings"

	"github.com/bwmarrin/discordgo"
)

func GeneralizeUsername(s *discordgo.Session, guildID string) error {
	members := cache.ListAllMembers()
	for _, member := range members {
		username := member.Nick
		if strings.HasPrefix(strings.ToLower(username), "lv") {
			lv, nickname := cache.ExtractLevelAndNickname(username)
			if lv == 0 {
				// cannot separate level and nickname
				// do nothing, but log error
				return fmt.Errorf("cannot separate level and nickname: %s", username)
			}

			// set nickname as generalized form
			newNickname := fmt.Sprintf("Lv %d %s", lv, nickname)
			if newNickname == username {
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
			// TODO: enable user dm with timer
			// assume that user should set their nickname as policy
			//dmchannel, err := s.UserChannelCreate(member.User.ID)
			//if err != nil {
			//	return err
			//}
			//
			//_, err = s.ChannelMessageSend(dmchannel.ID,
			//	fmt.Sprintf("안녕하세요. 메이플랜드 영원 길드 자동화 봇 입니다.\n길드 디스코드 내에서 서버 프로필 변경을 통해 닉네임을 인게임 레벨 닉네임으로 변경해 주세요.\n* 예시) `Lv 200 홍길동`"))
			//if err != nil {
			//	return err
			//}
			fmt.Printf("Cannot generalize name; Nickname is not started with 'lv': %s\n", username)
		}
	}
	return nil
}
