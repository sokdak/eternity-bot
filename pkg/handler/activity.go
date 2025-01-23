package handler

import (
	"errors"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/sokdak/eternity-bot/pkg/cache"
	"github.com/sokdak/eternity-bot/pkg/environment"
	"github.com/sokdak/eternity-bot/pkg/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"sync"
	"time"
)

var adb *gorm.DB

var lastGuildActivity = make(map[string]time.Time)
var lock = &sync.Mutex{}

type MemberInfoPersist struct {
	gorm.Model
	model.MemberInfo
	LastActivityTime time.Time
}

func ActivityInit(dg *discordgo.Session) error {
	var err error
	adb, err = gorm.Open(sqlite.Open(environment.ActivitySQLiteDBPath), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	err = adb.AutoMigrate(&MemberInfoPersist{})
	if err != nil {
		return fmt.Errorf("failed to migrate database: %w", err)
	}

	// add watchers
	dg.AddHandler(onMessageCreate)
	dg.AddHandler(onMessageUpdate)
	dg.AddHandler(onReactionAdd)
	dg.AddHandler(onReactionRemove)
	dg.AddHandler(onVoiceStateUpdate)
	dg.AddHandler(onTypingStart)

	return nil
}

func ActivityFinalize() {
	sqlDB, err := adb.DB()
	if err != nil {
		fmt.Println("failed to get db connection for close: %w", err)
		return
	}
	_ = sqlDB.Close()
}

func HandlePersistLastActivityTime() error {
	lock.Lock()
	defer lock.Unlock()

	p := []MemberInfoPersist{}
	for userID, lastActivityTime := range lastGuildActivity {
		m := cache.GetGuildMember(userID)
		if m == nil {
			return fmt.Errorf("failed to get guild member")
		}
		info, err := GetMemberInfoFromMember(m)
		if err != nil {
			return fmt.Errorf("failed to get member info")
		}
		if info == nil {
			return fmt.Errorf("not found member info")
		}

		var memberInfo MemberInfoPersist
		result := adb.First(&memberInfo, "nickname = ?", info.Nickname)
		if result.Error != nil {
			if errors.Is(result.Error, gorm.ErrRecordNotFound) {
				memberInfo = MemberInfoPersist{
					MemberInfo:       *info,
					LastActivityTime: lastActivityTime,
				}
				adb.Create(&memberInfo)
			} else {
				return fmt.Errorf("failed to query database: %w", result.Error)
			}
		} else {
			// upgrade for subrolename 4th job
			if memberInfo.SubRoleName != info.SubRoleName {
				memberInfo.SubRoleName = info.SubRoleName
			}
			if memberInfo.Level != info.Level {
				memberInfo.Level = info.Level
			}
			memberInfo.LastActivityTime = lastActivityTime
			p = append(p, memberInfo)
		}
	}

	// batch update
	if len(p) > 0 {
		adb.Save(&p)
	}

	return nil
}

func updateGuildActivity(userID string) {
	lock.Lock()
	defer lock.Unlock()
	lastGuildActivity[userID] = time.Now()
}

func onMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	if !m.Author.Bot {
		updateGuildActivity(m.Author.ID)
	}
}

func onMessageUpdate(s *discordgo.Session, m *discordgo.MessageUpdate) {
	if !m.Author.Bot {
		updateGuildActivity(m.Author.ID)
	}
}

func onReactionAdd(s *discordgo.Session, r *discordgo.MessageReactionAdd) {
	updateGuildActivity(r.UserID)
}

func onReactionRemove(s *discordgo.Session, r *discordgo.MessageReactionRemove) {
	updateGuildActivity(r.UserID)
}

func onVoiceStateUpdate(s *discordgo.Session, vs *discordgo.VoiceStateUpdate) {
	// vs.BeforeUpdate와 vs.AfterUpdate를 비교하거나,
	// 간단히 vs.VoiceState.ChannelID != "" 로 입장/이동 시점 체크
	updateGuildActivity(vs.UserID)
}

func onTypingStart(s *discordgo.Session, t *discordgo.TypingStart) {
	updateGuildActivity(t.UserID)
}
