package model

import "time"

type Cookie struct {
	ID             uint       `gorm:"primaryKey" json:"id"`
	Provider       string     `gorm:"size:32;index;not null" json:"provider"`
	Label          string     `gorm:"size:64;not null" json:"label"`
	ValueEncrypted string     `gorm:"type:text;not null" json:"-"`
	Active         bool       `gorm:"not null;default:true" json:"active"`
	Status         string     `gorm:"size:16;index;not null;default:unknown" json:"status"`
	Nickname       string     `gorm:"size:80" json:"nickname"`
	VipType        int        `gorm:"not null;default:0" json:"vip_type"`
	VipLevel       int        `gorm:"not null;default:0" json:"vip_level"`
	RedVipLevel    int        `gorm:"not null;default:0" json:"red_vip_level"`
	LastChecked    *time.Time `json:"last_checked"`
	CallCount      int64      `gorm:"not null;default:0" json:"call_count"`
	LastUsedAt     *time.Time `json:"last_used_at"`
	FailCount      int        `gorm:"not null;default:0" json:"fail_count"`
	LastError      string     `gorm:"size:255" json:"last_error"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}
