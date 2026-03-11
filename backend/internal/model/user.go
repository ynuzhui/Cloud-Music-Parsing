package model

import (
	"time"

	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID           uint       `gorm:"primaryKey" json:"id"`
	Username     string     `gorm:"size:64;uniqueIndex;not null" json:"username"`
	Email        string     `gorm:"size:120;uniqueIndex;not null" json:"email"`
	PasswordHash string     `gorm:"size:255;not null" json:"-"`
	Role         string     `gorm:"size:16;index;not null;default:user" json:"role"`
	Status       string     `gorm:"size:16;index;not null;default:active" json:"status"`
	TokenVersion int        `gorm:"not null;default:1" json:"token_version"`
	GroupID      *uint      `gorm:"index" json:"group_id"`
	DailyLimit   int        `gorm:"not null;default:0" json:"daily_limit"`
	Concurrency  int        `gorm:"not null;default:0" json:"concurrency_limit"`
	LastLoginAt  *time.Time `json:"last_login_at"`
	LastLoginIP  string     `gorm:"size:64" json:"last_login_ip"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

func (u *User) SetPassword(raw string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(raw), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u.PasswordHash = string(hash)
	return nil
}

func (u *User) CheckPassword(raw string) bool {
	return bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(raw)) == nil
}
