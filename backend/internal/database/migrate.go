package database

import (
	"go-music-aggregator/backend/internal/model"

	"gorm.io/gorm"
)

func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&model.User{},
		&model.Setting{},
		&model.Cookie{},
		&model.ParseRecord{},
		&model.AuditLog{},
	)
}
