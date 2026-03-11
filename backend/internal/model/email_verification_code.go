package model

import "time"

type EmailVerificationCode struct {
	ID        uint       `gorm:"primaryKey" json:"id"`
	Email     string     `gorm:"size:120;index;not null" json:"email"`
	Scene     string     `gorm:"size:32;index;not null" json:"scene"`
	Code      string     `gorm:"size:16;not null" json:"-"`
	ExpiresAt time.Time  `gorm:"index;not null" json:"expires_at"`
	UsedAt    *time.Time `json:"used_at"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

