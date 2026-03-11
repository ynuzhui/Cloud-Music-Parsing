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

const (
	superAdminUserGroupName = "超级管理员组"
	superAdminUserGroupDesc = "超级管理员用户组"
)

func NewUserAdminHandler(db *gorm.DB) *UserAdminHandler {
	return &UserAdminHandler{db: db}
}

func (h *UserAdminHandler) ListUsers(c *gin.Context) {
	page := parseIntWithRange(c.Query("page"), 1, 1, 1000000)
	pageSize := parseIntWithRange(c.Query("page_size"), 20, 1, 100)
	keyword := strings.TrimSpace(c.Query("keyword"))
	if keyword == "" {
		keyword = strings.TrimSpace(c.Query("username"))
	}
	role := strings.TrimSpace(c.Query("role"))
	status := strings.TrimSpace(c.Query("status"))

	query := h.db.Model(&model.User{})
	if keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("username LIKE ?", like)
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

	groupMetaMap := h.groupMetaMap(users)
	items := make([]gin.H, 0, len(users))
	for _, user := range users {
		meta := groupMetaMap[groupIDValue(user.GroupID)]
		items = append(items, gin.H{
			"id":                user.ID,
			"username":          user.Username,
			"email":             user.Email,
			"role":              user.Role,
			"status":            user.Status,
			"group_id":          user.GroupID,
			"group_name":        meta.Name,
			"group_unlimited":   meta.Unlimited,
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
		util.Err(c, http.StatusBadRequest, "请求参数格式错误")
		return
	}

	req.Username = strings.TrimSpace(req.Username)
	req.Email = strings.TrimSpace(req.Email)
	if !util.IsValidUsername(req.Username) {
		util.Err(c, http.StatusBadRequest, "用户名需以中文或英文开头，长度 2-32，可包含数字、下划线和短横线")
		return
	}
	if !util.IsValidEmail(req.Email) || len(req.Password) < 8 {
		util.Err(c, http.StatusBadRequest, "邮箱或密码格式不正确")
		return
	}

	role, err := normalizeRole(req.Role, "user")
	if err != nil {
		util.Err(c, http.StatusBadRequest, err.Error())
		return
	}
	if role == "super_admin" {
		util.Err(c, http.StatusBadRequest, "超级管理员固定为 ID=1，不可新增或转让")
		return
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
	if err := h.applyGroupQuotaInheritance(&user, true); err != nil {
		util.Err(c, http.StatusInternalServerError, err.Error())
		return
	}
	if err := user.SetPassword(req.Password); err != nil {
		util.Err(c, http.StatusInternalServerError, "设置密码失败")
		return
	}
	if err := h.db.Create(&user).Error; err != nil {
		util.Err(c, http.StatusConflict, "用户名或邮箱已存在")
		return
	}

	util.OK(c, gin.H{"id": user.ID})
}

func (h *UserAdminHandler) UpdateUser(c *gin.Context) {
	actor, ok := getActorClaims(c)
	if !ok {
		util.Err(c, http.StatusUnauthorized, "缺少登录信息")
		return
	}

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		util.Err(c, http.StatusBadRequest, "用户 ID 无效")
		return
	}
	var user model.User
	if err := h.db.First(&user, id).Error; err != nil {
		util.Err(c, http.StatusNotFound, "用户不存在")
		return
	}
	if user.Role == "super_admin" && actor.Role != "super_admin" {
		util.Err(c, http.StatusForbidden, "仅超级管理员可修改该用户")
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
		util.Err(c, http.StatusBadRequest, "请求参数格式错误")
		return
	}

	if req.Username != nil {
		username := strings.TrimSpace(*req.Username)
		if !util.IsValidUsername(username) {
			util.Err(c, http.StatusBadRequest, "用户名需以中文或英文开头，长度 2-32，可包含数字、下划线和短横线")
			return
		}
		user.Username = username
	}
	if req.Email != nil {
		email := strings.TrimSpace(*req.Email)
		if !util.IsValidEmail(email) {
			util.Err(c, http.StatusBadRequest, "邮箱格式不正确")
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
	if user.ID == 1 || user.Role == "super_admin" {
		superGroupID, groupErr := h.ensureSuperAdminGroup()
		if groupErr != nil {
			util.Err(c, http.StatusInternalServerError, groupErr.Error())
			return
		}
		user.GroupID = superGroupID
	}
	if err := h.applyGroupQuotaInheritance(&user, false); err != nil {
		util.Err(c, http.StatusInternalServerError, err.Error())
		return
	}

	if err := h.db.Save(&user).Error; err != nil {
		util.Err(c, http.StatusConflict, "用户名或邮箱已存在")
		return
	}
	util.OK(c, gin.H{"updated": true})
}

func (h *UserAdminHandler) UpdateUserStatus(c *gin.Context) {
	actor, ok := getActorClaims(c)
	if !ok {
		util.Err(c, http.StatusUnauthorized, "缺少登录信息")
		return
	}

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		util.Err(c, http.StatusBadRequest, "用户 ID 无效")
		return
	}
	var user model.User
	if err := h.db.First(&user, id).Error; err != nil {
		util.Err(c, http.StatusNotFound, "用户不存在")
		return
	}
	if user.Role == "super_admin" && actor.Role != "super_admin" {
		util.Err(c, http.StatusForbidden, "仅超级管理员可修改该用户")
		return
	}

	var req struct {
		Active bool `json:"active"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		util.Err(c, http.StatusBadRequest, "请求参数格式错误")
		return
	}
	nextStatus := "disabled"
	if req.Active {
		nextStatus = "active"
	}
	if user.ID == 1 && nextStatus != "active" {
		util.Err(c, http.StatusBadRequest, "超级管理员（ID=1）不可被禁用")
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
		util.Err(c, http.StatusUnauthorized, "缺少登录信息")
		return
	}

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		util.Err(c, http.StatusBadRequest, "用户 ID 无效")
		return
	}
	var target model.User
	if err := h.db.First(&target, id).Error; err != nil {
		util.Err(c, http.StatusNotFound, "用户不存在")
		return
	}

	var req struct {
		Role string `json:"role"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		util.Err(c, http.StatusBadRequest, "请求参数格式错误")
		return
	}
	role, err := normalizeRole(req.Role, "")
	if err != nil {
		util.Err(c, http.StatusBadRequest, err.Error())
		return
	}

	if target.ID == 1 || target.Role == "super_admin" {
		util.Err(c, http.StatusBadRequest, "超级管理员固定为 ID=1，不可转让或降级")
		return
	}
	if role == "super_admin" {
		util.Err(c, http.StatusBadRequest, "超级管理员固定为 ID=1，不可转让")
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
		util.Err(c, http.StatusUnauthorized, "缺少登录信息")
		return
	}

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		util.Err(c, http.StatusBadRequest, "用户 ID 无效")
		return
	}
	var target model.User
	if err := h.db.First(&target, id).Error; err != nil {
		util.Err(c, http.StatusNotFound, "用户不存在")
		return
	}
	if target.Role == "super_admin" && actor.Role != "super_admin" {
		util.Err(c, http.StatusForbidden, "仅超级管理员可修改该用户")
		return
	}

	var req struct {
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		util.Err(c, http.StatusBadRequest, "请求参数格式错误")
		return
	}
	if len(req.Password) < 8 {
		util.Err(c, http.StatusBadRequest, "密码至少 8 位")
		return
	}
	if err := target.SetPassword(req.Password); err != nil {
		util.Err(c, http.StatusInternalServerError, "设置密码失败")
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
			"unlimited_parse":   group.Unlimited,
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
		UnlimitedParse   bool   `json:"unlimited_parse"`
		IsDefault        bool   `json:"is_default"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		util.Err(c, http.StatusBadRequest, "请求参数格式错误")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		util.Err(c, http.StatusBadRequest, "用户组名称不能为空")
		return
	}

	group := model.UserGroup{
		Name:        req.Name,
		Description: strings.TrimSpace(req.Description),
		DailyLimit:  maxInt(req.DailyLimit, 0),
		Concurrency: maxInt(req.ConcurrencyLimit, 0),
		Unlimited:   req.UnlimitedParse,
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
		util.Err(c, http.StatusConflict, "用户组名称已存在")
		return
	}
	util.OK(c, gin.H{"id": group.ID})
}

func (h *UserAdminHandler) UpdateUserGroup(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		util.Err(c, http.StatusBadRequest, "用户组 ID 无效")
		return
	}
	var group model.UserGroup
	if err := h.db.First(&group, id).Error; err != nil {
		util.Err(c, http.StatusNotFound, "用户组不存在")
		return
	}

	var req struct {
		Name             *string `json:"name"`
		Description      *string `json:"description"`
		DailyLimit       *int    `json:"daily_limit"`
		ConcurrencyLimit *int    `json:"concurrency_limit"`
		UnlimitedParse   *bool   `json:"unlimited_parse"`
		IsDefault        *bool   `json:"is_default"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		util.Err(c, http.StatusBadRequest, "请求参数格式错误")
		return
	}

	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			util.Err(c, http.StatusBadRequest, "用户组名称不能为空")
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
	if req.UnlimitedParse != nil {
		group.Unlimited = *req.UnlimitedParse
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
		util.Err(c, http.StatusConflict, "用户组名称已存在")
		return
	}
	util.OK(c, gin.H{"updated": true})
}

func (h *UserAdminHandler) DeleteUserGroup(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		util.Err(c, http.StatusBadRequest, "用户组 ID 无效")
		return
	}
	var group model.UserGroup
	if err := h.db.First(&group, id).Error; err != nil {
		util.Err(c, http.StatusNotFound, "用户组不存在")
		return
	}
	if group.IsDefault {
		util.Err(c, http.StatusBadRequest, "默认用户组不可删除")
		return
	}

	var count int64
	if err := h.db.Model(&model.User{}).Where("group_id = ?", group.ID).Count(&count).Error; err != nil {
		util.Err(c, http.StatusInternalServerError, err.Error())
		return
	}
	if count > 0 {
		util.Err(c, http.StatusBadRequest, "该用户组下仍有用户，无法删除")
		return
	}
	if err := h.db.Delete(&model.UserGroup{}, group.ID).Error; err != nil {
		util.Err(c, http.StatusInternalServerError, err.Error())
		return
	}
	util.OK(c, gin.H{"deleted": true})
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
		return nil, errors.New("用户组不存在")
	}
	return groupID, nil
}

type userGroupMeta struct {
	Name      string
	Unlimited bool
}

func (h *UserAdminHandler) groupMetaMap(users []model.User) map[uint]userGroupMeta {
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
		return map[uint]userGroupMeta{}
	}
	var groups []model.UserGroup
	if err := h.db.Select("id", "name", "unlimited").Where("id IN ?", ids).Find(&groups).Error; err != nil {
		return map[uint]userGroupMeta{}
	}
	out := make(map[uint]userGroupMeta, len(groups))
	for _, group := range groups {
		out[group.ID] = userGroupMeta{
			Name:      group.Name,
			Unlimited: group.Unlimited,
		}
	}
	return out
}

func (h *UserAdminHandler) applyGroupQuotaInheritance(user *model.User, force bool) error {
	if user.GroupID == nil || *user.GroupID == 0 {
		return nil
	}
	var group model.UserGroup
	if err := h.db.Select("id", "daily_limit", "concurrency", "unlimited").First(&group, *user.GroupID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	if group.Unlimited {
		user.DailyLimit = 0
		user.Concurrency = 0
		return nil
	}
	if force {
		user.DailyLimit = maxInt(group.DailyLimit, 0)
		user.Concurrency = maxInt(group.Concurrency, 0)
	}
	return nil
}

func (h *UserAdminHandler) ensureSuperAdminGroup() (*uint, error) {
	return h.ensureSuperAdminGroupWithDB(h.db)
}

func (h *UserAdminHandler) ensureSuperAdminGroupWithDB(db *gorm.DB) (*uint, error) {
	var group model.UserGroup
	if err := db.Where("name = ?", superAdminUserGroupName).Order("id asc").First(&group).Error; err == nil {
		id := group.ID
		return &id, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	group = model.UserGroup{
		Name:        superAdminUserGroupName,
		Description: superAdminUserGroupDesc,
		DailyLimit:  0,
		Concurrency: 0,
		Unlimited:   true,
		IsDefault:   false,
	}
	if err := db.Create(&group).Error; err != nil {
		return nil, err
	}
	id := group.ID
	return &id, nil
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
		return "", errors.New("角色无效")
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
		return "", errors.New("状态无效")
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
