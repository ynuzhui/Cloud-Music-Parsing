package handler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"go-music-aggregator/backend/internal/middleware"
	"go-music-aggregator/backend/internal/model"
	"go-music-aggregator/backend/internal/security"
	"go-music-aggregator/backend/internal/util"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type UserAdminHandler struct {
	db *gorm.DB
}

func NewUserAdminHandler(db *gorm.DB) *UserAdminHandler {
	return &UserAdminHandler{db: db}
}

func (h *UserAdminHandler) ListUsers(c *gin.Context) {
	page := parseIntWithRange(c.Query("page"), 1, 1, 1000000)
	pageSize := parseIntWithRange(c.Query("page_size"), 20, 1, 100)
	keyword := strings.TrimSpace(c.Query("keyword"))
	role := strings.TrimSpace(c.Query("role"))
	status := strings.TrimSpace(c.Query("status"))

	query := h.db.Model(&model.User{})
	if keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("username LIKE ? OR email LIKE ?", like, like)
	}
	if role != "" {
		query = query.Where("role = ?", role)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		util.Err(c, http.StatusInternalServerError, err.Error())
		return
	}

	var users []model.User
	if err := query.Order("id desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&users).Error; err != nil {
		util.Err(c, http.StatusInternalServerError, err.Error())
		return
	}

	groupNameMap := h.groupNameMap(users)
	items := make([]gin.H, 0, len(users))
	for _, user := range users {
		items = append(items, gin.H{
			"id":                user.ID,
			"username":          user.Username,
			"email":             user.Email,
			"role":              user.Role,
			"status":            user.Status,
			"group_id":          user.GroupID,
			"group_name":        groupNameMap[groupIDValue(user.GroupID)],
			"daily_limit":       user.DailyLimit,
			"concurrency_limit": user.Concurrency,
			"last_login_at":     user.LastLoginAt,
			"last_login_ip":     user.LastLoginIP,
			"created_at":        user.CreatedAt,
			"updated_at":        user.UpdatedAt,
		})
	}

	util.OK(c, gin.H{
		"items":     items,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

func (h *UserAdminHandler) CreateUser(c *gin.Context) {
	actor, ok := getActorClaims(c)
	if !ok {
		util.Err(c, http.StatusUnauthorized, "missing auth context")
		return
	}

	var req struct {
		Username         string `json:"username"`
		Email            string `json:"email"`
		Password         string `json:"password"`
		Role             string `json:"role"`
		GroupID          *uint  `json:"group_id"`
		DailyLimit       int    `json:"daily_limit"`
		ConcurrencyLimit int    `json:"concurrency_limit"`
		Status           string `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		util.Err(c, http.StatusBadRequest, "invalid request body")
		return
	}

	req.Username = strings.TrimSpace(req.Username)
	req.Email = strings.TrimSpace(req.Email)
	if !util.IsValidUsername(req.Username) || !util.IsValidEmail(req.Email) || len(req.Password) < 8 {
		util.Err(c, http.StatusBadRequest, "invalid username, email or password")
		return
	}

	role, err := normalizeRole(req.Role, "user")
	if err != nil {
		util.Err(c, http.StatusBadRequest, err.Error())
		return
	}
	if role == "super_admin" && actor.Role != "super_admin" {
		util.Err(c, http.StatusForbidden, "super admin required")
		return
	}
	if role == "super_admin" {
		if err := h.ensureNoOtherSuperAdmin(0); err != nil {
			util.Err(c, http.StatusBadRequest, err.Error())
			return
		}
	}
	status, err := normalizeStatus(req.Status, "active")
	if err != nil {
		util.Err(c, http.StatusBadRequest, err.Error())
		return
	}

	groupID, err := h.normalizeGroupID(req.GroupID, true)
	if err != nil {
		util.Err(c, http.StatusBadRequest, err.Error())
		return
	}

	user := model.User{
		Username:     req.Username,
		Email:        req.Email,
		Role:         role,
		Status:       status,
		TokenVersion: 1,
		GroupID:      groupID,
		DailyLimit:   maxInt(req.DailyLimit, 0),
		Concurrency:  maxInt(req.ConcurrencyLimit, 0),
	}
	if err := user.SetPassword(req.Password); err != nil {
		util.Err(c, http.StatusInternalServerError, "failed to set password")
		return
	}
	if err := h.db.Create(&user).Error; err != nil {
		util.Err(c, http.StatusConflict, "username or email already exists")
		return
	}

	util.OK(c, gin.H{"id": user.ID})
}

func (h *UserAdminHandler) UpdateUser(c *gin.Context) {
	actor, ok := getActorClaims(c)
	if !ok {
		util.Err(c, http.StatusUnauthorized, "missing auth context")
		return
	}

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		util.Err(c, http.StatusBadRequest, "invalid user id")
		return
	}
	var user model.User
	if err := h.db.First(&user, id).Error; err != nil {
		util.Err(c, http.StatusNotFound, "user not found")
		return
	}
	if user.Role == "super_admin" && actor.Role != "super_admin" {
		util.Err(c, http.StatusForbidden, "super admin required")
		return
	}

	var req struct {
		Username         *string `json:"username"`
		Email            *string `json:"email"`
		GroupID          *uint   `json:"group_id"`
		DailyLimit       *int    `json:"daily_limit"`
		ConcurrencyLimit *int    `json:"concurrency_limit"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		util.Err(c, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Username != nil {
		username := strings.TrimSpace(*req.Username)
		if !util.IsValidUsername(username) {
			util.Err(c, http.StatusBadRequest, "invalid username")
			return
		}
		user.Username = username
	}
	if req.Email != nil {
		email := strings.TrimSpace(*req.Email)
		if !util.IsValidEmail(email) {
			util.Err(c, http.StatusBadRequest, "invalid email")
			return
		}
		user.Email = email
	}
	if req.GroupID != nil {
		gid, groupErr := h.normalizeGroupID(req.GroupID, false)
		if groupErr != nil {
			util.Err(c, http.StatusBadRequest, groupErr.Error())
			return
		}
		user.GroupID = gid
	}
	if req.DailyLimit != nil {
		user.DailyLimit = maxInt(*req.DailyLimit, 0)
	}
	if req.ConcurrencyLimit != nil {
		user.Concurrency = maxInt(*req.ConcurrencyLimit, 0)
	}

	if err := h.db.Save(&user).Error; err != nil {
		util.Err(c, http.StatusConflict, "username or email already exists")
		return
	}
	util.OK(c, gin.H{"updated": true})
}

func (h *UserAdminHandler) UpdateUserStatus(c *gin.Context) {
	actor, ok := getActorClaims(c)
	if !ok {
		util.Err(c, http.StatusUnauthorized, "missing auth context")
		return
	}

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		util.Err(c, http.StatusBadRequest, "invalid user id")
		return
	}
	var user model.User
	if err := h.db.First(&user, id).Error; err != nil {
		util.Err(c, http.StatusNotFound, "user not found")
		return
	}
	if user.Role == "super_admin" && actor.Role != "super_admin" {
		util.Err(c, http.StatusForbidden, "super admin required")
		return
	}

	var req struct {
		Active bool `json:"active"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		util.Err(c, http.StatusBadRequest, "invalid request body")
		return
	}
	nextStatus := "disabled"
	if req.Active {
		nextStatus = "active"
	}
	if user.Role == "super_admin" && nextStatus != "active" {
		util.Err(c, http.StatusBadRequest, "super admin cannot be disabled")
		return
	}
	if user.Status != nextStatus {
		user.Status = nextStatus
		user.TokenVersion++
	}
	if err := h.db.Save(&user).Error; err != nil {
		util.Err(c, http.StatusInternalServerError, err.Error())
		return
	}
	util.OK(c, gin.H{"updated": true})
}

func (h *UserAdminHandler) SetUserRole(c *gin.Context) {
	_, ok := getActorClaims(c)
	if !ok {
		util.Err(c, http.StatusUnauthorized, "missing auth context")
		return
	}

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		util.Err(c, http.StatusBadRequest, "invalid user id")
		return
	}
	var target model.User
	if err := h.db.First(&target, id).Error; err != nil {
		util.Err(c, http.StatusNotFound, "user not found")
		return
	}

	var req struct {
		Role string `json:"role"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		util.Err(c, http.StatusBadRequest, "invalid request body")
		return
	}
	role, err := normalizeRole(req.Role, "")
	if err != nil {
		util.Err(c, http.StatusBadRequest, err.Error())
		return
	}

	if role == "super_admin" {
		if err := h.transferSuperAdmin(uint(id)); err != nil {
			util.Err(c, http.StatusBadRequest, err.Error())
			return
		}
		util.OK(c, gin.H{"updated": true})
		return
	}
	if target.Role == "super_admin" && role != "super_admin" {
		util.Err(c, http.StatusBadRequest, "must transfer super admin role instead of demoting directly")
		return
	}
	if target.Role != role {
		target.Role = role
		target.TokenVersion++
		if err := h.db.Save(&target).Error; err != nil {
			util.Err(c, http.StatusInternalServerError, err.Error())
			return
		}
	}
	util.OK(c, gin.H{"updated": true})
}

func (h *UserAdminHandler) ResetUserPassword(c *gin.Context) {
	actor, ok := getActorClaims(c)
	if !ok {
		util.Err(c, http.StatusUnauthorized, "missing auth context")
		return
	}

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		util.Err(c, http.StatusBadRequest, "invalid user id")
		return
	}
	var target model.User
	if err := h.db.First(&target, id).Error; err != nil {
		util.Err(c, http.StatusNotFound, "user not found")
		return
	}
	if target.Role == "super_admin" && actor.Role != "super_admin" {
		util.Err(c, http.StatusForbidden, "super admin required")
		return
	}

	var req struct {
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		util.Err(c, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(req.Password) < 8 {
		util.Err(c, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}
	if err := target.SetPassword(req.Password); err != nil {
		util.Err(c, http.StatusInternalServerError, "failed to set password")
		return
	}
	target.TokenVersion++
	if err := h.db.Save(&target).Error; err != nil {
		util.Err(c, http.StatusInternalServerError, err.Error())
		return
	}
	util.OK(c, gin.H{"updated": true})
}

func (h *UserAdminHandler) ListUserGroups(c *gin.Context) {
	var groups []model.UserGroup
	if err := h.db.Order("id asc").Find(&groups).Error; err != nil {
		util.Err(c, http.StatusInternalServerError, err.Error())
		return
	}

	type memberCountRow struct {
		GroupID uint
		Count   int64
	}
	var rows []memberCountRow
	_ = h.db.Model(&model.User{}).
		Select("group_id, count(*) as count").
		Where("group_id IS NOT NULL").
		Group("group_id").
		Scan(&rows).Error

	countMap := make(map[uint]int64, len(rows))
	for _, row := range rows {
		countMap[row.GroupID] = row.Count
	}

	items := make([]gin.H, 0, len(groups))
	for _, group := range groups {
		items = append(items, gin.H{
			"id":                group.ID,
			"name":              group.Name,
			"description":       group.Description,
			"daily_limit":       group.DailyLimit,
			"concurrency_limit": group.Concurrency,
			"is_default":        group.IsDefault,
			"member_count":      countMap[group.ID],
			"created_at":        group.CreatedAt,
			"updated_at":        group.UpdatedAt,
		})
	}
	util.OK(c, items)
}

func (h *UserAdminHandler) CreateUserGroup(c *gin.Context) {
	var req struct {
		Name             string `json:"name"`
		Description      string `json:"description"`
		DailyLimit       int    `json:"daily_limit"`
		ConcurrencyLimit int    `json:"concurrency_limit"`
		IsDefault        bool   `json:"is_default"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		util.Err(c, http.StatusBadRequest, "invalid request body")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		util.Err(c, http.StatusBadRequest, "name is required")
		return
	}

	group := model.UserGroup{
		Name:        req.Name,
		Description: strings.TrimSpace(req.Description),
		DailyLimit:  maxInt(req.DailyLimit, 0),
		Concurrency: maxInt(req.ConcurrencyLimit, 0),
		IsDefault:   req.IsDefault,
	}
	if err := h.db.Transaction(func(tx *gorm.DB) error {
		if group.IsDefault {
			if err := tx.Model(&model.UserGroup{}).Where("is_default = ?", true).Update("is_default", false).Error; err != nil {
				return err
			}
		}
		return tx.Create(&group).Error
	}); err != nil {
		util.Err(c, http.StatusConflict, "group name already exists")
		return
	}
	util.OK(c, gin.H{"id": group.ID})
}

func (h *UserAdminHandler) UpdateUserGroup(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		util.Err(c, http.StatusBadRequest, "invalid group id")
		return
	}
	var group model.UserGroup
	if err := h.db.First(&group, id).Error; err != nil {
		util.Err(c, http.StatusNotFound, "group not found")
		return
	}

	var req struct {
		Name             *string `json:"name"`
		Description      *string `json:"description"`
		DailyLimit       *int    `json:"daily_limit"`
		ConcurrencyLimit *int    `json:"concurrency_limit"`
		IsDefault        *bool   `json:"is_default"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		util.Err(c, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			util.Err(c, http.StatusBadRequest, "name cannot be empty")
			return
		}
		group.Name = name
	}
	if req.Description != nil {
		group.Description = strings.TrimSpace(*req.Description)
	}
	if req.DailyLimit != nil {
		group.DailyLimit = maxInt(*req.DailyLimit, 0)
	}
	if req.ConcurrencyLimit != nil {
		group.Concurrency = maxInt(*req.ConcurrencyLimit, 0)
	}
	if req.IsDefault != nil {
		group.IsDefault = *req.IsDefault
	}

	if err := h.db.Transaction(func(tx *gorm.DB) error {
		if group.IsDefault {
			if err := tx.Model(&model.UserGroup{}).Where("id <> ? AND is_default = ?", group.ID, true).Update("is_default", false).Error; err != nil {
				return err
			}
		}
		return tx.Save(&group).Error
	}); err != nil {
		util.Err(c, http.StatusConflict, "group name already exists")
		return
	}
	util.OK(c, gin.H{"updated": true})
}

func (h *UserAdminHandler) DeleteUserGroup(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		util.Err(c, http.StatusBadRequest, "invalid group id")
		return
	}
	var group model.UserGroup
	if err := h.db.First(&group, id).Error; err != nil {
		util.Err(c, http.StatusNotFound, "group not found")
		return
	}
	if group.IsDefault {
		util.Err(c, http.StatusBadRequest, "default group cannot be deleted")
		return
	}

	var count int64
	if err := h.db.Model(&model.User{}).Where("group_id = ?", group.ID).Count(&count).Error; err != nil {
		util.Err(c, http.StatusInternalServerError, err.Error())
		return
	}
	if count > 0 {
		util.Err(c, http.StatusBadRequest, "group has users, cannot delete")
		return
	}
	if err := h.db.Delete(&model.UserGroup{}, group.ID).Error; err != nil {
		util.Err(c, http.StatusInternalServerError, err.Error())
		return
	}
	util.OK(c, gin.H{"deleted": true})
}

func (h *UserAdminHandler) transferSuperAdmin(targetID uint) error {
	if targetID == 0 {
		return errors.New("invalid target user")
	}
	return h.db.Transaction(func(tx *gorm.DB) error {
		var target model.User
		if err := tx.First(&target, targetID).Error; err != nil {
			return err
		}
		var current model.User
		err := tx.Where("role = ? AND id <> ?", "super_admin", targetID).First(&current).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err == nil {
			if err := tx.Model(&model.User{}).
				Where("id = ?", current.ID).
				Updates(map[string]any{
					"role":          "admin",
					"token_version": gorm.Expr("token_version + 1"),
				}).Error; err != nil {
				return err
			}
		}
		return tx.Model(&model.User{}).
			Where("id = ?", targetID).
			Updates(map[string]any{
				"role":          "super_admin",
				"status":        "active",
				"token_version": gorm.Expr("token_version + 1"),
			}).Error
	})
}

func (h *UserAdminHandler) ensureNoOtherSuperAdmin(excludeUserID uint) error {
	query := h.db.Model(&model.User{}).Where("role = ?", "super_admin")
	if excludeUserID > 0 {
		query = query.Where("id <> ?", excludeUserID)
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return errors.New("super admin can only be one")
	}
	return nil
}

func (h *UserAdminHandler) normalizeGroupID(groupID *uint, useDefaultWhenNil bool) (*uint, error) {
	if groupID == nil {
		if !useDefaultWhenNil {
			return nil, nil
		}
		var group model.UserGroup
		if err := h.db.Where("is_default = ?", true).Order("id asc").First(&group).Error; err != nil {
			return nil, nil
		}
		id := group.ID
		return &id, nil
	}
	if *groupID == 0 {
		return nil, nil
	}
	var count int64
	if err := h.db.Model(&model.UserGroup{}).Where("id = ?", *groupID).Count(&count).Error; err != nil {
		return nil, err
	}
	if count == 0 {
		return nil, errors.New("group not found")
	}
	return groupID, nil
}

func (h *UserAdminHandler) groupNameMap(users []model.User) map[uint]string {
	ids := make([]uint, 0, len(users))
	seen := make(map[uint]struct{}, len(users))
	for _, user := range users {
		if user.GroupID == nil {
			continue
		}
		if _, ok := seen[*user.GroupID]; ok {
			continue
		}
		seen[*user.GroupID] = struct{}{}
		ids = append(ids, *user.GroupID)
	}
	if len(ids) == 0 {
		return map[uint]string{}
	}
	var groups []model.UserGroup
	if err := h.db.Select("id", "name").Where("id IN ?", ids).Find(&groups).Error; err != nil {
		return map[uint]string{}
	}
	out := make(map[uint]string, len(groups))
	for _, group := range groups {
		out[group.ID] = group.Name
	}
	return out
}

func getActorClaims(c *gin.Context) (*security.Claims, bool) {
	claimsAny, ok := c.Get(middleware.ContextClaimsKey)
	if !ok {
		return nil, false
	}
	claims, ok := claimsAny.(*security.Claims)
	return claims, ok
}

func parseIntWithRange(raw string, fallback, min, max int) int {
	value := strings.TrimSpace(raw)
	if value == "" {
		return fallback
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	if n < min {
		return min
	}
	if n > max {
		return max
	}
	return n
}

func normalizeRole(raw, fallback string) (string, error) {
	role := strings.ToLower(strings.TrimSpace(raw))
	if role == "" {
		role = fallback
	}
	switch role {
	case "user", "admin", "super_admin":
		return role, nil
	default:
		return "", errors.New("invalid role")
	}
}

func normalizeStatus(raw, fallback string) (string, error) {
	status := strings.ToLower(strings.TrimSpace(raw))
	if status == "" {
		status = fallback
	}
	switch status {
	case "active", "disabled":
		return status, nil
	default:
		return "", errors.New("invalid status")
	}
}

func groupIDValue(groupID *uint) uint {
	if groupID == nil {
		return 0
	}
	return *groupID
}

func maxInt(v int, floor int) int {
	if v < floor {
		return floor
	}
	return v
}
