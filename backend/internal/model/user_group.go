package model

import "time"

type UserGroup struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Name        string    `gorm:"size:64;uniqueIndex;not null" json:"name"`
	Description string    `gorm:"size:255" json:"description"`
	DailyLimit  int       `gorm:"not null;default:0" json:"daily_limit"`
	Concurrency int       `gorm:"not null;default:0" json:"concurrency_limit"`
	IsDefault   bool      `gorm:"index;not null;default:false" json:"is_default"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
