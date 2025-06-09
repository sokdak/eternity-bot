package handler

import (
	"fmt"
	"github.com/sokdak/eternity-bot/pkg/cache"
	"github.com/sokdak/eternity-bot/pkg/model"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

var mainRoleList = map[string][]string{
	"전사": {
		"히어로",
		"팔라딘",
		"다크나이트",
	},
	"궁수": {
		"보우마스터",
		"신궁",
	},
	"마법사": {
		"아크메이지(썬,콜)",
		"아크메이지(불,독)",
		"비숍",
	},
	"도적": {
		"나이트로드",
		"섀도어",
	},
}

func UpdateMessageWithRoles(s *discordgo.Session, channelID string, messageIDs []string) error {
	members := cache.ListAllMembers()

	var ms []model.MemberInfo
	for _, member := range members {
		m, err := GetMemberInfoFromMember(member)
		if err != nil {
			return fmt.Errorf("failed to get member info: %w", err)
		}
		if m == nil {
			return fmt.Errorf("member info is nil")
		}
		ms = append(ms, *m)
	}

	// sort by main role
	// sort order by 전사, 궁수, 마법사, 도적
	roleOrder := map[string]int{
		"히어로":        1,
		"팔라딘":        2,
		"다크나이트":      3,
		"보우마스터":      4,
		"신궁":         5,
		"아크메이지(썬,콜)": 6,
		"아크메이지(불,독)": 7,
		"비숍":         8,
		"나이트로드":      9,
		"섀도어":        10,
	}

	// sort by role order, then by level
	sort.Slice(ms, func(i, j int) bool {
		return roleOrder[ms[i].SubRoleName] < roleOrder[ms[j].SubRoleName]
	})
	sort.SliceStable(ms, func(i, j int) bool {
		if roleOrder[ms[i].SubRoleName] == roleOrder[ms[j].SubRoleName] {
			return ms[i].Level > ms[j].Level
		}
		return false
	})

	// averaging mainrole level
	mainroleAverageLevel := make(map[string]float64)
	mainroleCount := make(map[string]int)
	for _, m := range ms {
		mainroleAverageLevel[m.MainRoleName] += float64(m.Level)
		mainroleCount[m.MainRoleName]++
	}
	for k, _ := range mainroleAverageLevel {
		mainroleAverageLevel[k] /= float64(mainroleCount[k])
	}

	// averaging subrole level
	subroleAverageLevel := make(map[string]map[string]float64)
	subroleCount := make(map[string]map[string]int)
	for _, m := range ms {
		if subroleAverageLevel[m.MainRoleName] == nil {
			subroleAverageLevel[m.MainRoleName] = make(map[string]float64)
			subroleCount[m.MainRoleName] = make(map[string]int)
		}
		subroleAverageLevel[m.MainRoleName][m.SubRoleName] += float64(m.Level)
		subroleCount[m.MainRoleName][m.SubRoleName]++
	}
	for k, _ := range subroleAverageLevel {
		for kk, _ := range subroleAverageLevel[k] {
			subroleAverageLevel[k][kk] /= float64(subroleCount[k][kk])
		}
	}

	loc, _ := time.LoadLocation("Asia/Seoul")

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("**[직업 별 길드원 분포]** (%s 기준)\n", time.Now().In(loc).Format("2006-01-02 15:04:05")))
	memberCount := 0

	// using ms instead of roleMembers
	currentMainRole := ""
	currentSubRole := ""
	for _, mk := range ms {
		if currentMainRole != mk.MainRoleName {
			if currentMainRole != "" {
				sb.WriteString("\n")
			}
			currentMainRole = mk.MainRoleName
			currentSubRole = mk.SubRoleName

			sb.WriteString(fmt.Sprintf("\n**%s** (%d명 / 평균 %.1f)\n", mk.MainRoleName, mainroleCount[mk.MainRoleName], mainroleAverageLevel[mk.MainRoleName]))
			sb.WriteString(fmt.Sprintf("- **%s** (%d명 / 평균 %.1f): ", mk.SubRoleName, subroleCount[mk.MainRoleName][mk.SubRoleName], subroleAverageLevel[mk.MainRoleName][mk.SubRoleName]))
			sb.WriteString(mk.Mention + " ")
		} else if currentSubRole != mk.SubRoleName {
			currentSubRole = mk.SubRoleName
			sb.WriteString(fmt.Sprintf("\n- **%s** (%d명 / 평균 %.1f): ", mk.SubRoleName, subroleCount[mk.MainRoleName][mk.SubRoleName], subroleAverageLevel[mk.MainRoleName][mk.SubRoleName]))
			sb.WriteString(mk.Mention + " ")
		} else {
			sb.WriteString(mk.Mention + " ")
		}
		memberCount++
	}

	sb.WriteString(fmt.Sprintf("\n\n**[총 인원: %d명]**\n", memberCount))

	// split the message by carriage return if it exceeds 2000 characters
	if sb.Len() > 2000 {
		// find last carriage return before 2000 characters
		lastCR := strings.LastIndex(sb.String()[:2000], "\n")
		if lastCR == -1 {
			lastCR = 2000 // if no carriage return, split at 2000 characters
		}

		// split the message
		firstPart := sb.String()[:lastCR]
		secondPart := sb.String()[lastCR:]

		// edit the first message
		if len(messageIDs) > 1 {
			_, err := s.ChannelMessageEdit(channelID, messageIDs[0], firstPart)
			if err != nil {
				return fmt.Errorf("failed to edit first message: %w", err)
			}

			_, err = s.ChannelMessageEdit(channelID, messageIDs[1], secondPart)
			if err != nil {
				return fmt.Errorf("failed to edit second message: %w", err)
			}
		} else {
			// return error if no message IDs are provided
			return fmt.Errorf("no message IDs provided")
		}
	} else {
		// edit the message
		_, err := s.ChannelMessageEdit(channelID, messageIDs[0], sb.String())
		if err != nil {
			return fmt.Errorf("failed to edit message: %w", err)
		}
	}

	return nil
}

func UpdateMessagesWithLevels(s *discordgo.Session, channelID string, messageIDs []string) error {
	members := cache.ListAllMembers()

	var ms []model.MemberInfo
	for _, member := range members {
		m, err := GetMemberInfoFromMember(member)
		if err != nil {
			return fmt.Errorf("failed to get member info: %w", err)
		}
		if m == nil {
			return fmt.Errorf("member info is nil")
		}
		ms = append(ms, *m)
	}

	// sort by level
	sort.Slice(ms, func(i, j int) bool {
		return ms[i].Level > ms[j].Level
	})

	var sb strings.Builder
	loc, _ := time.LoadLocation("Asia/Seoul")
	sb.WriteString(fmt.Sprintf("**[⚒️ 레벨 별 길드원 분포 ⚒️]** (%s 기준)\n", time.Now().In(loc).Format("2006-01-02 15:04:05")))

	// level distribution by 10
	levelDist := make(map[int]int)
	for _, m := range ms {
		levelDist[m.Level/10*10]++
	}

	// averaging level
	avgLevel := 0.0
	for _, m := range ms {
		avgLevel += float64(m.Level)
	}
	avgLevel /= float64(len(ms))

	// median level
	medianLevel := 0
	if len(ms)%2 == 0 {
		medianLevel = (ms[len(ms)/2-1].Level + ms[len(ms)/2].Level) / 2
	} else {
		medianLevel = ms[len(ms)/2].Level
	}

	for i := 200; i > 0; i -= 10 {
		if levelDist[i] == 0 {
			continue
		}
		sb.WriteString(fmt.Sprintf("\n**%d ~ %d** (%d명): ", i, i+9, levelDist[i]))
		for _, m := range ms {
			if m.Level >= i && m.Level <= i+9 {
				sb.WriteString(m.Mention + " ")
			}
		}
		sb.WriteString("\n")
	}
	sb.WriteString(fmt.Sprintf("\n**[평균 레벨: %.1f, 중앙값 레벨: %d]**\n", avgLevel, medianLevel))

	// split the message by carriage return if it exceeds 2000 characters
	if sb.Len() > 2000 {
		// find last carriage return before 2000 characters
		lastCR := strings.LastIndex(sb.String()[:2000], "\n")
		if lastCR == -1 {
			lastCR = 2000 // if no carriage return, split at 2000 characters
		}

		// split the message
		firstPart := sb.String()[:lastCR]
		secondPart := sb.String()[lastCR:]

		// edit the first message
		if len(messageIDs) > 1 {
			_, err := s.ChannelMessageEdit(channelID, messageIDs[0], firstPart)
			if err != nil {
				return fmt.Errorf("failed to edit first message: %w", err)
			}

			_, err = s.ChannelMessageEdit(channelID, messageIDs[1], secondPart)
			if err != nil {
				return fmt.Errorf("failed to edit second message: %w", err)
			}
		} else {
			// return error if no message IDs are provided
			return fmt.Errorf("no message IDs provided")
		}
	} else {
		// edit the message
		_, err := s.ChannelMessageEdit(channelID, messageIDs[0], sb.String())
		if err != nil {
			return fmt.Errorf("failed to edit message: %w", err)
		}
	}

	return nil
}

func GetMemberInfoFromMember(member *discordgo.Member) (*model.MemberInfo, error) {
	// get username
	username := member.Nick
	lv, nickname := cache.ExtractLevelAndNickname(username)
	if lv == 0 {
		// cannot separate level and nickname
		// do nothing, but log error
		return nil, fmt.Errorf("cannot separate level and nickname: %s", username)
	}

	// get mainrole
	mainRole := ""
	for mr, sr := range mainRoleList {
		for _, dmr := range member.Roles {
			if slices.Contains(sr, cache.GetRoleNameByID(dmr)) {
				mainRole = mr
				break
			}
		}
	}
	if mainRole == "" {
		// cannot find main role
		// do nothing, but log error
		return nil, fmt.Errorf("cannot find main role: %s", username)
	}

	flatSrSlice := []string{}
	for _, sr := range mainRoleList[mainRole] {
		flatSrSlice = append(flatSrSlice, sr)
	}
	subRole := ""
	for _, dsr := range member.Roles {
		if slices.Contains(flatSrSlice, cache.GetRoleNameByID(dsr)) {
			subRole = cache.GetRoleNameByID(dsr)
			break
		}
	}
	if subRole == "" {
		// cannot find sub role
		// do nothing, but log error
		return nil, fmt.Errorf("cannot find sub role: %s", username)
	}

	return &model.MemberInfo{
		MainRoleName: mainRole,
		SubRoleName:  subRole,
		Level:        lv,
		Nickname:     nickname,
		Mention:      fmt.Sprintf("<@%s>", member.User.ID),
	}, nil
}
