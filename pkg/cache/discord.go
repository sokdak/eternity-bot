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

var roleList = []string{"용기사", "크루세이더", "나이트", "레인저", "저격수", "썬콜", "불독", "프리스트", "허밋", "시프마스터"}

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
	if n, ok := roleNameIDCache[roleName]; ok {
		return n
	}
	roleCacheLock.Unlock()
	return ""
}

func GetRoleNameByID(roleID string) string {
	roleCacheLock.Lock()
	for roleName, id := range roleNameIDCache {
		if id == roleID {
			roleCacheLock.Unlock()
			return roleName
		}
	}
	roleCacheLock.Unlock()
	return ""
}

func ListAllRoles() map[string]string {
	roleCacheLock.Lock()
	roles := roleNameIDCache
	roleCacheLock.Unlock()
	return roles
}

func GetGuildMember(memberID string) *discordgo.Member {
	memberCacheLock.Lock()
	if member, ok := guildMembersCache[memberID]; ok {
		return member
	}
	memberCacheLock.Unlock()
	return nil
}

func ListAllMembers() []*discordgo.Member {
	memberCacheLock.Lock()
	// flattening slice
	members := []*discordgo.Member{}
	for _, v := range guildMembersCache {
		members = append(members, v)
	}
	memberCacheLock.Unlock()
	return members
}
