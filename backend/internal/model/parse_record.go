package model

import "time"

type ParseRecord struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"index;not null" json:"user_id"`
	RequestIP string    `gorm:"size:64;index" json:"request_ip"`
	Provider  string    `gorm:"size:32;index;not null" json:"provider"`
	SourceURL string    `gorm:"type:text;not null" json:"source_url"`
	ResultURL string    `gorm:"type:text" json:"result_url"`
	Quality   string    `gorm:"size:32" json:"quality"`
	CacheHit  bool      `gorm:"not null;default:false" json:"cache_hit"`
	Status    string    `gorm:"size:20;not null" json:"status"`
	CreatedAt time.Time `gorm:"index" json:"created_at"`
}
