package service

import (
	"go-music-aggregator/backend/internal/model"
	"go-music-aggregator/backend/internal/util"
	"time"

	"gorm.io/gorm"
)

type DayCount struct {
	Day   string `json:"day"`
	Count int64  `json:"count"`
}

type DashboardStats struct {
	UserCount      int64      `json:"user_count"`
	UserNewToday   int64      `json:"user_new_today"`
	UserNewPrevDay int64      `json:"user_new_prev_day"`
	ParseCount     int64      `json:"parse_count"`
	ParseToday     int64      `json:"parse_today"`
	CookieCount    int64      `json:"cookie_count"`
	CookieNewToday int64      `json:"cookie_new_today"`
	CookieNewPrev  int64      `json:"cookie_new_prev_day"`
	PVTotal        int64      `json:"pv_total"`
	UVTotal        int64      `json:"uv_total"`
	PVToday        int64      `json:"pv_today"`
	UVToday        int64      `json:"uv_today"`
	AvgLatencyMS   float64    `json:"avg_latency_ms"`
	Trend7Days     []DayCount `json:"trend_7days"`
	PVTrend7Days   []DayCount `json:"pv_trend_7days"`
	UVTrend7Days   []DayCount `json:"uv_trend_7days"`
	LatencyTrend7d []DayCount `json:"latency_trend_7days"`
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

	now := util.NowBeijing()
	dayStart, dayEnd := util.BeijingDayRange(now)
	prevDayStart, prevDayEnd := util.BeijingDayRange(now.AddDate(0, 0, -1))

	if err := s.db.Model(&model.ParseRecord{}).
		Where("created_at >= ? AND created_at < ?", dayStart, dayEnd).
		Count(&out.ParseToday).Error; err != nil {
		return out, err
	}
	if err := s.db.Model(&model.User{}).
		Where("created_at >= ? AND created_at < ?", dayStart, dayEnd).
		Count(&out.UserNewToday).Error; err != nil {
		return out, err
	}
	if err := s.db.Model(&model.User{}).
		Where("created_at >= ? AND created_at < ?", prevDayStart, prevDayEnd).
		Count(&out.UserNewPrevDay).Error; err != nil {
		return out, err
	}
	if err := s.db.Model(&model.Cookie{}).
		Where("created_at >= ? AND created_at < ?", dayStart, dayEnd).
		Count(&out.CookieNewToday).Error; err != nil {
		return out, err
	}
	if err := s.db.Model(&model.Cookie{}).
		Where("created_at >= ? AND created_at < ?", prevDayStart, prevDayEnd).
		Count(&out.CookieNewPrev).Error; err != nil {
		return out, err
	}

	if err := s.db.Model(&model.AuditLog{}).Count(&out.PVTotal).Error; err != nil {
		return out, err
	}
	if err := s.db.Model(&model.AuditLog{}).Distinct("ip").Where("ip <> ''").Count(&out.UVTotal).Error; err != nil {
		return out, err
	}
	if err := s.db.Model(&model.AuditLog{}).
		Where("created_at >= ? AND created_at < ?", dayStart, dayEnd).
		Count(&out.PVToday).Error; err != nil {
		return out, err
	}
	if err := s.db.Model(&model.AuditLog{}).
		Distinct("ip").
		Where("created_at >= ? AND created_at < ? AND ip <> ''", dayStart, dayEnd).
		Count(&out.UVToday).Error; err != nil {
		return out, err
	}

	if err := s.db.Model(&model.AuditLog{}).Select("COALESCE(AVG(latency_ms), 0)").Scan(&out.AvgLatencyMS).Error; err != nil {
		return out, err
	}

	from := now.AddDate(0, 0, -6).Format("2006-01-02")

	var parseRows []DayCount
	if err := s.db.Model(&model.ParseRecord{}).
		Select("date(created_at) as day, count(*) as count").
		Where("date(created_at) >= ?", from).
		Group("date(created_at)").
		Order("date(created_at) asc").
		Scan(&parseRows).Error; err != nil {
		return out, err
	}
	out.Trend7Days = fillDaySeries(parseRows, now, 7)

	var pvRows []DayCount
	if err := s.db.Model(&model.AuditLog{}).
		Select("date(created_at) as day, count(*) as count").
		Where("date(created_at) >= ?", from).
		Group("date(created_at)").
		Order("date(created_at) asc").
		Scan(&pvRows).Error; err != nil {
		return out, err
	}
	out.PVTrend7Days = fillDaySeries(pvRows, now, 7)

	var uvRows []DayCount
	if err := s.db.Model(&model.AuditLog{}).
		Select("date(created_at) as day, count(distinct ip) as count").
		Where("date(created_at) >= ? AND ip <> ''", from).
		Group("date(created_at)").
		Order("date(created_at) asc").
		Scan(&uvRows).Error; err != nil {
		return out, err
	}
	out.UVTrend7Days = fillDaySeries(uvRows, now, 7)

	var latencyRawRows []struct {
		Day string  `json:"day"`
		Avg float64 `json:"avg"`
	}
	if err := s.db.Model(&model.AuditLog{}).
		Select("date(created_at) as day, avg(latency_ms) as avg").
		Where("date(created_at) >= ?", from).
		Group("date(created_at)").
		Order("date(created_at) asc").
		Scan(&latencyRawRows).Error; err != nil {
		return out, err
	}
	latencyRows := make([]DayCount, 0, len(latencyRawRows))
	for _, row := range latencyRawRows {
		latencyRows = append(latencyRows, DayCount{
			Day:   row.Day,
			Count: int64(row.Avg + 0.5),
		})
	}
	out.LatencyTrend7d = fillDaySeries(latencyRows, now, 7)

	return out, nil
}

func fillDaySeries(rows []DayCount, end time.Time, days int) []DayCount {
	if days <= 0 {
		return rows
	}
	countMap := make(map[string]int64, len(rows))
	for _, row := range rows {
		countMap[row.Day] = row.Count
	}
	out := make([]DayCount, 0, days)
	for i := days - 1; i >= 0; i-- {
		day := end.AddDate(0, 0, -i).Format("2006-01-02")
		out = append(out, DayCount{
			Day:   day,
			Count: countMap[day],
		})
	}
	return out
}
