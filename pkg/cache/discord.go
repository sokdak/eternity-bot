package cache

import (
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/sokdak/eternity-bot/pkg/environment"
	"slices"
	"sync"
	"time"
)

var roleNameIDCache = make(map[string]string)
var guildMembersCache = make(map[string]*discordgo.Member)
var roleCacheLock = sync.Mutex{}
var memberCacheLock = sync.Mutex{}

var roleList = []string{"다크나이트", "히어로", "팔라딘", "보우마스터", "신궁", "아크메이지(썬,콜)", "아크메이지(불,독)", "비숍", "나이트로드", "섀도어"}

func RunDiscordCacheEvictionPolicy(s *discordgo.Session, roleCachingPeriod time.Duration, memberCachingPeriod time.Duration) {
	rcp := time.NewTicker(roleCachingPeriod)
	mcp := time.NewTicker(memberCachingPeriod)

	renewRoleMap(s)
	renewMemberMap(s)

	go func() {
		for {
			select {
			case <-rcp.C:
				roleCacheLock.Lock()
				renewRoleMap(s)
				roleCacheLock.Unlock()
			case <-mcp.C:
				memberCacheLock.Lock()
				renewMemberMap(s)
				memberCacheLock.Unlock()
			}
		}
	}()
}

func renewRoleMap(s *discordgo.Session) {
	newRoleMap := make(map[string]string)
	roles, err := s.GuildRoles(environment.DiscordGuildID)
	if err != nil {
		fmt.Printf("Error fetching guild roles while updating role cache: %v\n", err)
		return
	}
	for _, role := range roles {
		newRoleMap[role.Name] = role.ID
	}
	roleNameIDCache = newRoleMap
}

func renewMemberMap(s *discordgo.Session) {
	newMemberMap := make(map[string]*discordgo.Member)
	members, err := s.GuildMembers(environment.DiscordGuildID, "", 1000)
	if err != nil {
		fmt.Printf("Error fetching guild members while updating member cache: %v\n", err)
		return
	}
	for _, member := range members {
		gamer := false
		for _, role := range member.Roles {
			if slices.Contains(roleList, GetRoleNameByID(role)) {
				gamer = true
				break
			}
		}
		if gamer == false {
			continue
		}

		lv, nick := ExtractLevelAndNickname(member.Nick)
		if lv == 0 {
			continue
		}
		newMemberMap[nick] = member
	}
	guildMembersCache = newMemberMap
}

func GetRoleID(roleName string) string {
	roleCacheLock.Lock()
	defer roleCacheLock.Unlock()
	if n, ok := roleNameIDCache[roleName]; ok {
		return n
	}
	return ""
}

func GetRoleNameByID(roleID string) string {
	roleCacheLock.Lock()
	defer roleCacheLock.Unlock()
	for roleName, id := range roleNameIDCache {
		if id == roleID {
			return roleName
		}
	}
	return ""
}

func ListAllRoles() map[string]string {
	roleCacheLock.Lock()
	defer roleCacheLock.Unlock()
	roles := roleNameIDCache
	return roles
}

func GetGuildMember(memberID string) *discordgo.Member {
	memberCacheLock.Lock()
	defer memberCacheLock.Unlock()
	for _, v := range guildMembersCache {
		if v.User.ID == memberID {
			return v
		}
	}
	return nil
}

func ListAllMembers() []*discordgo.Member {
	memberCacheLock.Lock()
	defer memberCacheLock.Unlock()
	// flattening slice
	members := []*discordgo.Member{}
	for _, v := range guildMembersCache {
		members = append(members, v)
	}
	return members
}

func ListAllMembersNicknameMap() map[string]*discordgo.Member {
	memberCacheLock.Lock()
	defer memberCacheLock.Unlock()
	members := guildMembersCache
	return members
}
