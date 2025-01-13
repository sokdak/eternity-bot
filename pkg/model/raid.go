package model

import (
	"gorm.io/gorm"
	"time"
)

type Raid struct {
	gorm.Model
	RaidName    string
	Type        string
	Description string
	Manager     string
}

type RaidSchedule struct {
	gorm.Model
	RaidID              uint
	Raid                Raid `gorm:"foreignKey:RaidID"`
	TryCount            int
	SubscriptionEndTime time.Time
	StartTime           time.Time
	MessageID           string
}

type RaidAttend struct {
	gorm.Model
	MemberInfo
	Canceled       bool
	RaidScheduleID uint
	RaidSchedule   RaidSchedule `gorm:"foreignKey:RaidScheduleID"`
}

type RaidInfo struct {
	gorm.Model
	EntranceTime       time.Time
	StartTime          time.Time
	EndTime            time.Time
	MessageID          string
	RaidScheduleID     uint
	RaidSchedule       RaidSchedule `gorm:"foreignKey:RaidScheduleID"`
	DistributionRuleID uint
	DistributionRule   DistributionRule `gorm:"foreignKey:DistributionRuleID"`
	RaidPartyInfo      []RaidPartyInfo  `gorm:"foreignKey:RaidInfoID"`
}

type DistributionRule struct {
	gorm.Model
	Name              string
	Reason            string
	TargetPartyRole   string
	TargetLevel       int
	TargetAttackPower int
	DistributionType  string
	Amount            float32
}

type RaidPartyInfo struct {
	gorm.Model
	RaidInfoID uint
	RaidInfo   RaidInfo `gorm:"foreignKey:RaidInfoID"`

	Order     int
	PartyRole string
	Members   []RaidPartyMemberInfo `gorm:"foreignKey:RaidPartyInfoID"`
}

type RaidPartyMemberInfo struct {
	gorm.Model
	RaidPartyInfoID uint
	RaidPartyInfo   RaidPartyInfo `gorm:"foreignKey:RaidPartyInfoID"`

	MemberInfo
	Role string
}
