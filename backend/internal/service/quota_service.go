package service

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"go-music-aggregator/backend/internal/model"
	"go-music-aggregator/backend/internal/util"

	"gorm.io/gorm"
)

type QuotaSnapshot struct {
	Timezone         string `json:"timezone"`
	Date             string `json:"date"`
	DailyLimit       int    `json:"daily_limit"`
	Used             int64  `json:"used"`
	Remaining        int64  `json:"remaining"`
	ConcurrencyLimit int    `json:"concurrency_limit"`
	InFlight         int    `json:"in_flight"`
}

type QuotaService struct {
	db             *gorm.DB
	settingService *SettingService
	mu             sync.Mutex
	inFlight       map[string]int
}

func NewQuotaService(db *gorm.DB, settingSvc *SettingService) *QuotaService {
	return &QuotaService{
		db:             db,
		settingService: settingSvc,
		inFlight:       make(map[string]int),
	}
}

// AcquireParseQuota checks daily quota and concurrent quota then acquires one in-flight slot.
// Caller must call returned release function once request is done.
func (s *QuotaService) AcquireParseQuota(userID uint, requestIP string) (func(), *QuotaSnapshot, error) {
	effective, err := s.resolveEffectiveLimit(userID)
	if err != nil {
		return nil, nil, err
	}

	used, day, err := s.todayUsage(userID, requestIP)
	if err != nil {
		return nil, nil, err
	}
	if effective.DailyLimit > 0 && used >= int64(effective.DailyLimit) {
		return nil, nil, errors.New("已达到每日解析次数上限")
	}

	// Atomically check concurrency and acquire slot under a single lock
	key := s.concurrentKey(userID, requestIP)
	s.mu.Lock()
	current := s.inFlight[key]
	if effective.ConcurrencyLimit > 0 && current >= effective.ConcurrencyLimit {
		s.mu.Unlock()
		return nil, nil, errors.New("已达到并发上限")
	}
	s.inFlight[key] = current + 1
	s.mu.Unlock()

	released := false
	release := func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		if released {
			return
		}
		released = true
		next := s.inFlight[key] - 1
		if next <= 0 {
			delete(s.inFlight, key)
			return
		}
		s.inFlight[key] = next
	}

	snapshot := &QuotaSnapshot{
		Timezone:         util.BeijingTimezone,
		Date:             day.Format("2006-01-02"),
		DailyLimit:       effective.DailyLimit,
		Used:             used,
		Remaining:        remain(used, effective.DailyLimit),
		ConcurrencyLimit: effective.ConcurrencyLimit,
		InFlight:         current + 1,
	}
	return release, snapshot, nil
}

func (s *QuotaService) Today(userID uint, requestIP string) (*QuotaSnapshot, error) {
	effective, err := s.resolveEffectiveLimit(userID)
	if err != nil {
		return nil, err
	}
	used, day, err := s.todayUsage(userID, requestIP)
	if err != nil {
		return nil, err
	}
	inFlight := s.getInFlight(s.concurrentKey(userID, requestIP))
	return &QuotaSnapshot{
		Timezone:         util.BeijingTimezone,
		Date:             day.Format("2006-01-02"),
		DailyLimit:       effective.DailyLimit,
		Used:             used,
		Remaining:        remain(used, effective.DailyLimit),
		ConcurrencyLimit: effective.ConcurrencyLimit,
		InFlight:         inFlight,
	}, nil
}

func (s *QuotaService) Trend(userID uint, days int) ([]DayCount, error) {
	if userID == 0 {
		return nil, errors.New("请先登录")
	}
	if days <= 0 || days > 31 {
		days = 7
	}

	now := util.NowBeijing()
	start, _ := util.BeijingDayRange(now.AddDate(0, 0, -(days - 1)))
	end := now.Add(24 * time.Hour)

	var rows []DayCount
	if err := s.db.Model(&model.ParseRecord{}).
		Select("date(created_at) as day, count(*) as count").
		Where("user_id = ? AND created_at >= ? AND created_at < ?", userID, start, end).
		Group("date(created_at)").
		Order("date(created_at) asc").
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	countMap := make(map[string]int64, len(rows))
	for _, row := range rows {
		countMap[row.Day] = row.Count
	}
	out := make([]DayCount, 0, days)
	for i := days - 1; i >= 0; i-- {
		day := now.AddDate(0, 0, -i).Format("2006-01-02")
		out = append(out, DayCount{
			Day:   day,
			Count: countMap[day],
		})
	}
	return out, nil
}

type effectiveLimit struct {
	DailyLimit       int
	ConcurrencyLimit int
}

func (s *QuotaService) resolveEffectiveLimit(userID uint) (*effectiveLimit, error) {
	settings, err := s.settingService.Load()
	if err != nil {
		return nil, err
	}
	limit := &effectiveLimit{
		DailyLimit:       maxInt(settings.Feature.DefaultDailyLimit, 0),
		ConcurrencyLimit: maxInt(settings.Feature.DefaultConcurrency, 0),
	}
	if userID == 0 {
		return limit, nil
	}

	var user model.User
	if err := s.db.Select("id", "group_id", "daily_limit", "concurrency").First(&user, userID).Error; err != nil {
		return nil, err
	}
	if user.GroupID != nil {
		var group model.UserGroup
		if err := s.db.Select("id", "daily_limit", "concurrency", "unlimited").First(&group, *user.GroupID).Error; err == nil {
			if group.Unlimited {
				return &effectiveLimit{
					DailyLimit:       0,
					ConcurrencyLimit: 0,
				}, nil
			}
			if group.DailyLimit > 0 {
				limit.DailyLimit = group.DailyLimit
			}
			if group.Concurrency > 0 {
				limit.ConcurrencyLimit = group.Concurrency
			}
		}
	}
	if user.DailyLimit > 0 {
		limit.DailyLimit = user.DailyLimit
	}
	if user.Concurrency > 0 {
		limit.ConcurrencyLimit = user.Concurrency
	}
	return limit, nil
}

func (s *QuotaService) todayUsage(userID uint, requestIP string) (int64, time.Time, error) {
	start, end := util.BeijingDayRange(util.NowBeijing())
	var count int64
	query := s.db.Model(&model.ParseRecord{}).Where("created_at >= ? AND created_at < ?", start, end)
	if userID > 0 {
		query = query.Where("user_id = ?", userID)
	} else {
		ip := strings.TrimSpace(requestIP)
		if ip == "" {
			ip = "unknown"
		}
		query = query.Where("user_id = ? AND request_ip = ?", 0, ip)
	}
	if err := query.Count(&count).Error; err != nil {
		return 0, time.Time{}, err
	}
	return count, start, nil
}

func (s *QuotaService) concurrentKey(userID uint, requestIP string) string {
	if userID > 0 {
		return fmt.Sprintf("u:%d", userID)
	}
	ip := strings.TrimSpace(requestIP)
	if ip == "" {
		ip = "unknown"
	}
	return "ip:" + ip
}

func (s *QuotaService) getInFlight(key string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.inFlight[key]
}

func (s *QuotaService) incInFlight(key string, delta int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.inFlight[key] += delta
}

func remain(used int64, limit int) int64 {
	if limit <= 0 {
		return -1
	}
	left := int64(limit) - used
	if left < 0 {
		return 0
	}
	return left
}

func maxInt(v int, floor int) int {
	if v < floor {
		return floor
	}
	return v
}
