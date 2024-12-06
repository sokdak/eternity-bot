package handler

import (
	"fmt"
	"slices"
	"sort"
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

	for roleName, ms := range roleMembers {
		// sort by level
		sort.Slice(ms, func(i, j int) bool {
			return ms[i].Level > ms[j].Level
		})
		roleMembers[roleName] = ms
	}

	// flattening the map
	var ms []MemberInfo
	for _, v := range roleMembers {
		ms = append(ms, v...)
	}

	// sort by main role
	// sort order by 전사, 궁수, 마법사, 도적
	roleOrder := map[string]int{
		"크루세이더": 1,
		"나이트":   2,
		"용기사":   3,
		"레인저":   4,
		"저격수":   5,
		"썬콜":    6,
		"불독":    7,
		"프리스트":  8,
		"허밋":    9,
		"시프마스터": 10,
	}
	sort.Slice(ms, func(i, j int) bool {
		return roleOrder[ms[i].RoleName] < roleOrder[ms[j].RoleName]
	})

	sort.SliceStable(ms, func(i, j int) bool {
		if roleOrder[ms[i].RoleName] == roleOrder[ms[j].RoleName] {
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
		subroleAverageLevel[m.MainRoleName][m.RoleName] += float64(m.Level)
		subroleCount[m.MainRoleName][m.RoleName]++
	}
	for k, _ := range subroleAverageLevel {
		for kk, _ := range subroleAverageLevel[k] {
			subroleAverageLevel[k][kk] /= float64(subroleCount[k][kk])
		}
	}

	var sb strings.Builder
	loc, _ := time.LoadLocation("Asia/Seoul")
	sb.WriteString(fmt.Sprintf("**[⚒️ 직업 별 길드원 분포 ⚒️]** (%s 기준)\n", time.Now().In(loc).Format("2006-01-02 15:04:05")))
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
			currentSubRole = mk.RoleName

			sb.WriteString(fmt.Sprintf("\n**%s** (%d명 / 평렙 %.1f)\n", mk.MainRoleName, mainroleCount[mk.MainRoleName], mainroleAverageLevel[mk.MainRoleName]))
			sb.WriteString(fmt.Sprintf("- **%s** (%d명 / 평렙 %.1f): ", mk.RoleName, subroleCount[mk.MainRoleName][mk.RoleName], subroleAverageLevel[mk.MainRoleName][mk.RoleName]))
			sb.WriteString(mk.Mention + " ")
		} else if currentSubRole != mk.RoleName {
			currentSubRole = mk.RoleName
			sb.WriteString(fmt.Sprintf("\n- **%s** (%d명 / 평렙 %.1f): ", mk.RoleName, subroleCount[mk.MainRoleName][mk.RoleName], subroleAverageLevel[mk.MainRoleName][mk.RoleName]))
			sb.WriteString(mk.Mention + " ")
		} else {
			sb.WriteString(mk.Mention + " ")
		}
		memberCount++
	}

	sb.WriteString(fmt.Sprintf("\n\n**[총 인원: %d명]**\n", memberCount))

	_, err = s.ChannelMessageEdit(channelID, messageID, sb.String())
	if err != nil {
		return fmt.Errorf("failed to edit message: %w", err)
	}

	return nil
}

type MemberInfo struct {
	RoleName     string
	MainRoleName string
	Level        int
	Nickname     string
	Mention      string
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

	// flattening the map
	var ms []MemberInfo
	for _, v := range roleMembers {
		ms = append(ms, v...)
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

	// averaging level
	avgLevel := 0.0
	for _, m := range ms {
		avgLevel += float64(m.Level)
	}
	avgLevel /= float64(len(ms))
	sb.WriteString(fmt.Sprintf("\n**[평균 레벨: %.1f, ", avgLevel))

	// median level
	medianLevel := 0
	if len(ms)%2 == 0 {
		medianLevel = (ms[len(ms)/2-1].Level + ms[len(ms)/2].Level) / 2
	} else {
		medianLevel = ms[len(ms)/2].Level
	}
	sb.WriteString(fmt.Sprintf("중앙값 레벨: %d]**\n", medianLevel))

	_, err = s.ChannelMessageEdit(channelID, messageID, sb.String())
	if err != nil {
		return fmt.Errorf("failed to edit message: %w", err)
	}

	return nil
}

var mainRoleList = map[string][]string{
	"전사": {
		"크루세이더",
		"나이트",
		"용기사",
	},
	"궁수": {
		"레인저",
		"저격수",
	},
	"마법사": {
		"썬콜",
		"불독",
		"프리스트",
	},
	"도적": {
		"허밋",
		"시프마스터",
	},
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
	if !strings.HasPrefix(newUsername, "lv") && !strings.HasPrefix(newUsername, "Lv") &&
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

	// get mainrole
	mainRole := ""
	for mr, roles := range mainRoleList {
		if slices.Contains(roles, role) {
			mainRole = mr
			break
		}
	}

	return &MemberInfo{
		RoleName:     role,
		MainRoleName: mainRole,
		Level:        lv,
		Nickname:     nickname,
		Mention:      fmt.Sprintf("<@%s>", member.User.ID),
	}, nil
}
