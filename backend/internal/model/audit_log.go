package model

import "time"

type AuditLog struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Path        string    `gorm:"size:255;index;not null" json:"path"`
	Method      string    `gorm:"size:10;index;not null" json:"method"`
	IP          string    `gorm:"size:64;index;not null" json:"ip"`
	StatusCode  int       `gorm:"index;not null" json:"status_code"`
	LatencyMS   int64     `gorm:"not null" json:"latency_ms"`
	RequestBody string    `gorm:"type:text" json:"request_body"`
	CreatedAt   time.Time `json:"created_at"`
}
