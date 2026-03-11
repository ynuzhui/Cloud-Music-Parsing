package service

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const geetestValidateHost = "gcaptcha4.geetest.com"

type CaptchaPayload struct {
	Provider             string `json:"provider"`
	GeetestLotNumber     string `json:"geetest_lot_number"`
	GeetestCaptchaOutput string `json:"geetest_captcha_output"`
	GeetestPassToken     string `json:"geetest_pass_token"`
	GeetestGenTime       string `json:"geetest_gen_time"`
	CloudflareToken      string `json:"cloudflare_token"`
}

type CaptchaService struct {
	settingService *SettingService
	httpClient     *http.Client
}

func NewCaptchaService(settingSvc *SettingService) *CaptchaService {
	return &CaptchaService{
		settingService: settingSvc,
		httpClient: &http.Client{
			Timeout: 8 * time.Second,
		},
	}
}

func (s *CaptchaService) VerifyLogin(payload *CaptchaPayload, clientIP string) error {
	return s.verify(payload, clientIP)
}

func (s *CaptchaService) VerifyRegister(payload *CaptchaPayload, clientIP string) error {
	return s.verify(payload, clientIP)
}

func (s *CaptchaService) verify(payload *CaptchaPayload, clientIP string) error {
	settings, err := s.settingService.Load()
	if err != nil {
		return errors.New("加载验证码配置失败")
	}
	cfg := normalizeCaptchaSettings(settings.Captcha)
	if !cfg.Enabled {
		return nil
	}
	if payload == nil {
		return errors.New("请先完成验证码校验")
	}

	provider := normalizeCaptchaProvider(cfg.Provider)
	if rawProvider := strings.TrimSpace(payload.Provider); rawProvider != "" {
		if normalizeCaptchaProvider(rawProvider) != provider {
			return errors.New("验证码类型不匹配")
		}
	}

	switch provider {
	case "cloudflare":
		return s.verifyCloudflare(cfg, payload, clientIP)
	default:
		return s.verifyGeetest(cfg, payload)
	}
}

func (s *CaptchaService) verifyGeetest(cfg CaptchaSettings, payload *CaptchaPayload) error {
	captchaID := strings.TrimSpace(cfg.GeetestCaptchaID)
	privateKey := strings.TrimSpace(cfg.GeetestCaptchaKey)
	if captchaID == "" || privateKey == "" {
		return errors.New("系统未配置极验验证码参数")
	}

	lotNumber := strings.TrimSpace(payload.GeetestLotNumber)
	captchaOutput := strings.TrimSpace(payload.GeetestCaptchaOutput)
	passToken := strings.TrimSpace(payload.GeetestPassToken)
	genTime := strings.TrimSpace(payload.GeetestGenTime)
	if lotNumber == "" || captchaOutput == "" || passToken == "" || genTime == "" {
		return errors.New("请先完成极验验证码校验")
	}

	signToken := hmacSHA256Hex(privateKey, lotNumber)
	form := url.Values{}
	form.Set("lot_number", lotNumber)
	form.Set("captcha_output", captchaOutput)
	form.Set("pass_token", passToken)
	form.Set("gen_time", genTime)
	form.Set("sign_token", signToken)

	endpoint := fmt.Sprintf("https://%s/validate?captcha_id=%s", geetestValidateHost, url.QueryEscape(captchaID))
	req, err := http.NewRequest(http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return errors.New("创建极验请求失败")
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return errors.New("请求极验验证失败")
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("极验验证异常（HTTP %d）", resp.StatusCode)
	}

	var result struct {
		Result string `json:"result"`
		Reason string `json:"reason"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return errors.New("解析极验返回结果失败")
	}
	if strings.ToLower(strings.TrimSpace(result.Result)) != "success" {
		reason := strings.TrimSpace(result.Reason)
		if reason == "" {
			reason = "极验验证未通过"
		}
		return errors.New(reason)
	}
	return nil
}

func (s *CaptchaService) verifyCloudflare(cfg CaptchaSettings, payload *CaptchaPayload, clientIP string) error {
	siteKey := strings.TrimSpace(cfg.CloudflareSiteKey)
	secretKey := strings.TrimSpace(cfg.CloudflareSecretKey)
	if siteKey == "" || secretKey == "" {
		return errors.New("系统未配置 Cloudflare 验证码参数")
	}

	token := strings.TrimSpace(payload.CloudflareToken)
	if token == "" {
		return errors.New("请先完成 Cloudflare 验证码校验")
	}

	form := url.Values{}
	form.Set("secret", secretKey)
	form.Set("response", token)
	if ip := strings.TrimSpace(clientIP); ip != "" {
		form.Set("remoteip", ip)
	}

	req, err := http.NewRequest(
		http.MethodPost,
		"https://challenges.cloudflare.com/turnstile/v0/siteverify",
		strings.NewReader(form.Encode()),
	)
	if err != nil {
		return errors.New("创建 Cloudflare 请求失败")
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return errors.New("请求 Cloudflare 验证失败")
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Cloudflare 验证异常（HTTP %d）", resp.StatusCode)
	}

	var result struct {
		Success    bool     `json:"success"`
		ErrorCodes []string `json:"error-codes"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return errors.New("解析 Cloudflare 返回结果失败")
	}
	if !result.Success {
		codeText := strings.Join(result.ErrorCodes, ",")
		if codeText == "" {
			return errors.New("Cloudflare 验证未通过")
		}
		return fmt.Errorf("Cloudflare 验证未通过: %s", codeText)
	}
	return nil
}

func hmacSHA256Hex(key, msg string) string {
	mac := hmac.New(sha256.New, []byte(key))
	_, _ = mac.Write([]byte(msg))
	return hex.EncodeToString(mac.Sum(nil))
}
