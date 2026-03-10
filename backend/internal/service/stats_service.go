package service

import (
	"time"

	"go-music-aggregator/backend/internal/model"

	"gorm.io/gorm"
)

type DayCount struct {
	Day   string `json:"day"`
	Count int64  `json:"count"`
}

type DashboardStats struct {
	UserCount   int64      `json:"user_count"`
	ParseCount  int64      `json:"parse_count"`
	CookieCount int64      `json:"cookie_count"`
	Trend7Days  []DayCount `json:"trend_7days"`
}

type StatsService struct {
	db *gorm.DB
}

func NewStatsService(db *gorm.DB) *StatsService {
	return &StatsService{db: db}
}

func (s *StatsService) Dashboard() (DashboardStats, error) {
	var out DashboardStats

	if err := s.db.Model(&model.User{}).Count(&out.UserCount).Error; err != nil {
		return out, err
	}
	if err := s.db.Model(&model.ParseRecord{}).Count(&out.ParseCount).Error; err != nil {
		return out, err
	}
	if err := s.db.Model(&model.Cookie{}).Count(&out.CookieCount).Error; err != nil {
		return out, err
	}

	from := time.Now().AddDate(0, 0, -6).Format("2006-01-02")
	var rows []DayCount
	if err := s.db.Model(&model.ParseRecord{}).
		Select("date(created_at) as day, count(*) as count").
		Where("date(created_at) >= ?", from).
		Group("date(created_at)").
		Order("date(created_at) asc").
		Scan(&rows).Error; err != nil {
		return out, err
	}
	out.Trend7Days = rows

	return out, nil
}
