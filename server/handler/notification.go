package handler

import (
	"strconv"

	"daidai-panel/database"
	"daidai-panel/middleware"
	"daidai-panel/model"
	"daidai-panel/pkg/response"
	"daidai-panel/service"

	"github.com/gin-gonic/gin"
)

type NotificationHandler struct{}

func NewNotificationHandler() *NotificationHandler {
	return &NotificationHandler{}
}

func (h *NotificationHandler) List(c *gin.Context) {
	var channels []model.NotifyChannel
	database.DB.Order("created_at DESC").Find(&channels)

	data := make([]map[string]interface{}, len(channels))
	for i, ch := range channels {
		data[i] = ch.ToDict()
	}

	response.Success(c, gin.H{"data": data})
}

func (h *NotificationHandler) Create(c *gin.Context) {
	var req struct {
		Name   string `json:"name" binding:"required"`
		Type   string `json:"type" binding:"required"`
		Config string `json:"config"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}

	if req.Config == "" {
		req.Config = "{}"
	}

	ch := model.NotifyChannel{
		Name:    req.Name,
		Type:    req.Type,
		Config:  req.Config,
		Enabled: true,
	}

	if err := database.DB.Create(&ch).Error; err != nil {
		response.InternalError(c, "创建通知渠道失败")
		return
	}

	response.Created(c, gin.H{"message": "创建成功", "data": ch.ToDict()})
}

func (h *NotificationHandler) Update(c *gin.Context) {
	chID, _ := strconv.ParseUint(c.Param("id"), 10, 32)

	var ch model.NotifyChannel
	if err := database.DB.First(&ch, chID).Error; err != nil {
		response.NotFound(c, "通知渠道不存在")
		return
	}

	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}

	allowed := map[string]bool{"name": true, "type": true, "config": true}
	updates := make(map[string]interface{})
	for k, v := range req {
		if allowed[k] {
			updates[k] = v
		}
	}

	if len(updates) > 0 {
		database.DB.Model(&ch).Updates(updates)
	}

	database.DB.First(&ch, chID)
	response.Success(c, gin.H{"message": "更新成功", "data": ch.ToDict()})
}

func (h *NotificationHandler) Delete(c *gin.Context) {
	chID, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	database.DB.Where("id = ?", chID).Delete(&model.NotifyChannel{})
	response.Success(c, gin.H{"message": "删除成功"})
}

func (h *NotificationHandler) Enable(c *gin.Context) {
	chID, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	var ch model.NotifyChannel
	if err := database.DB.First(&ch, chID).Error; err != nil {
		response.NotFound(c, "通知渠道不存在")
		return
	}
	database.DB.Model(&ch).Update("enabled", true)
	ch.Enabled = true
	response.Success(c, gin.H{"message": "已启用", "data": ch.ToDict()})
}

func (h *NotificationHandler) Disable(c *gin.Context) {
	chID, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	var ch model.NotifyChannel
	if err := database.DB.First(&ch, chID).Error; err != nil {
		response.NotFound(c, "通知渠道不存在")
		return
	}
	database.DB.Model(&ch).Update("enabled", false)
	ch.Enabled = false
	response.Success(c, gin.H{"message": "已禁用", "data": ch.ToDict()})
}

func (h *NotificationHandler) Test(c *gin.Context) {
	chID, _ := strconv.ParseUint(c.Param("id"), 10, 32)

	var ch model.NotifyChannel
	if err := database.DB.First(&ch, chID).Error; err != nil {
		response.NotFound(c, "通知渠道不存在")
		return
	}

	err := service.SendNotificationToChannel(&ch, "呆呆面板测试通知", "这是一条测试通知消息，如果您收到此消息，说明通知渠道配置正确。")
	if err != nil {
		response.BadRequest(c, "发送失败: "+err.Error())
		return
	}

	response.Success(c, gin.H{"message": "测试通知发送成功"})
}

func (h *NotificationHandler) Types(c *gin.Context) {
	types := []map[string]string{
		{"type": "webhook", "name": "Webhook"},
		{"type": "email", "name": "邮件"},
		{"type": "telegram", "name": "Telegram"},
		{"type": "dingtalk", "name": "钉钉"},
		{"type": "wecom", "name": "企业微信机器人"},
		{"type": "wecom_app", "name": "企业微信应用"},
		{"type": "bark", "name": "Bark"},
		{"type": "pushplus", "name": "PushPlus"},
		{"type": "serverchan", "name": "Server酱"},
		{"type": "feishu", "name": "飞书"},
		{"type": "gotify", "name": "Gotify"},
		{"type": "pushdeer", "name": "PushDeer"},
		{"type": "pushme", "name": "PushMe"},
		{"type": "chanify", "name": "Chanify"},
		{"type": "igot", "name": "iGot"},
		{"type": "qmsg", "name": "Qmsg"},
		{"type": "pushover", "name": "Pushover"},
		{"type": "discord", "name": "Discord"},
		{"type": "slack", "name": "Slack"},
		{"type": "ntfy", "name": "ntfy"},
		{"type": "wxpusher", "name": "WxPusher"},
		{"type": "custom", "name": "自定义"},
	}
	response.Success(c, gin.H{"data": types})
}

func (h *NotificationHandler) RegisterRoutes(r *gin.RouterGroup) {
	notify := r.Group("/notifications", middleware.JWTAuth(), middleware.RequireUserToken(), middleware.RequireAdmin())
	{
		notify.GET("", h.List)
		notify.POST("", h.Create)
		notify.PUT("/:id", h.Update)
		notify.DELETE("/:id", h.Delete)
		notify.PUT("/:id/enable", h.Enable)
		notify.PUT("/:id/disable", h.Disable)
		notify.POST("/:id/test", h.Test)
		notify.GET("/types", h.Types)
	}
}
