package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"go-music-aggregator/backend/internal/model"
	"go-music-aggregator/backend/internal/util"
)

const (
	CookieStatusUnknown = "unknown"
	CookieStatusValid   = "valid"
	CookieStatusInvalid = "invalid"
)

type CookieVerifyResult struct {
	Valid       bool   `json:"valid"`
	Status      string `json:"status"`
	Nickname    string `json:"nickname"`
	VipType     int    `json:"vip_type"`
	VipLevel    int    `json:"vip_level"`
	RedVipLevel int    `json:"red_vip_level"`
	Error       string `json:"error,omitempty"`
}

func (s *ParseService) ExtractMusicU(raw string) string {
	return extractMusicUFromRaw(raw)
}

func (s *ParseService) VerifyNeteaseCookie(ctx context.Context, rawCookie string) (*CookieVerifyResult, error) {
	musicU := extractMusicUFromRaw(rawCookie)
	if musicU == "" {
		return nil, errors.New("cookie does not contain MUSIC_U")
	}

	result := &CookieVerifyResult{
		Valid:  false,
		Status: CookieStatusInvalid,
	}

	accountResp := struct {
		Code    int `json:"code"`
		Account *struct {
			VipType     int `json:"vipType"`
			RedVipLevel int `json:"redVipLevel"`
		} `json:"account"`
		Profile *struct {
			Nickname    string `json:"nickname"`
			VipType     int    `json:"vipType"`
			RedVipLevel int    `json:"redVipLevel"`
		} `json:"profile"`
	}{}

	if err := s.callNeteaseAPI(ctx, http.MethodPost, "https://music.163.com/api/nuser/account/get", nil, "", musicU, &accountResp); err != nil {
		result.Error = err.Error()
		return result, nil
	}

	if accountResp.Code != 200 || accountResp.Account == nil || accountResp.Profile == nil || strings.TrimSpace(accountResp.Profile.Nickname) == "" {
		result.Error = "cookie is not logged in or has expired"
		return result, nil
	}

	levelResp := struct {
		Code int `json:"code"`
		Data *struct {
			Level int `json:"level"`
		} `json:"data"`
	}{}
	vipLevel := 0
	if err := s.callNeteaseAPI(ctx, http.MethodPost, "https://music.163.com/api/user/level", nil, "", musicU, &levelResp); err == nil {
		if levelResp.Code == 200 && levelResp.Data != nil {
			vipLevel = levelResp.Data.Level
		}
	}

	vipType := accountResp.Profile.VipType
	if vipType == 0 {
		vipType = accountResp.Account.VipType
	}
	redVipLevel := accountResp.Account.RedVipLevel
	if redVipLevel == 0 {
		redVipLevel = accountResp.Profile.RedVipLevel
	}
	result.Valid = true
	result.Status = CookieStatusValid
	result.Nickname = strings.TrimSpace(accountResp.Profile.Nickname)
	result.VipType = vipType
	result.VipLevel = vipLevel
	result.RedVipLevel = redVipLevel
	result.Error = ""
	return result, nil
}

func (s *ParseService) callNeteaseAPI(ctx context.Context, method, endpoint string, query url.Values, formBody string, musicU string, out any) error {
	if query == nil {
		query = url.Values{}
	}
	target := endpoint
	encodedQuery := query.Encode()
	if encodedQuery != "" {
		target += "?" + encodedQuery
	}

	var body io.Reader
	if formBody != "" {
		body = strings.NewReader(formBody)
	}
	req, err := http.NewRequestWithContext(ctx, method, target, body)
	if err != nil {
		return err
	}
	setNeteasePublicHeaders(req)
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if strings.TrimSpace(musicU) != "" {
		req.AddCookie(&http.Cookie{Name: "MUSIC_U", Value: strings.TrimSpace(musicU)})
	}

	client := s.buildHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("netease upstream status: %d", resp.StatusCode)
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return err
	}
	return nil
}

func setNeteasePublicHeaders(req *http.Request) {
	req.Header.Set("Referer", "https://music.163.com")
	req.Header.Set("User-Agent", util.RandomUserAgent())
	spoofIP := util.RandomIPv4()
	req.Header.Set("X-Forwarded-For", spoofIP)
	req.Header.Set("Client-IP", spoofIP)
	req.Header.Set("X-Real-IP", spoofIP)
}

func extractMusicUFromRaw(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if !strings.Contains(raw, "=") {
		return raw
	}
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ';' || r == '\n' || r == '\r'
	})
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		idx := strings.Index(part, "=")
		if idx <= 0 {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(part[:idx]), "MUSIC_U") {
			return strings.Trim(strings.TrimSpace(part[idx+1:]), "\"")
		}
	}
	return ""
}

func (s *ParseService) pickActiveCookie() string {
	rows := make([]model.Cookie, 0)
	if err := s.db.Where("provider = ? AND active = ? AND status = ?", "netease", true, CookieStatusValid).Find(&rows).Error; err != nil {
		return ""
	}
	if len(rows) == 0 {
		if err := s.db.Where("provider = ? AND active = ?", "netease", true).Find(&rows).Error; err != nil || len(rows) == 0 {
			return ""
		}
	}

	row := rows[util.RandomInt(len(rows))]
	plain, err := s.box.Decrypt(row.ValueEncrypted)
	if err != nil {
		return ""
	}
	musicU := extractMusicUFromRaw(plain)
	if musicU == "" {
		musicU = strings.TrimSpace(plain)
	}
	if musicU == "" {
		return ""
	}

	now := time.Now()
	_ = s.db.Model(&model.Cookie{}).Where("id = ?", row.ID).Updates(map[string]any{
		"call_count":   row.CallCount + 1,
		"last_used_at": &now,
	}).Error
	return musicU
}
