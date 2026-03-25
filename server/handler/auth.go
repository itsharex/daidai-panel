package handler

import (
	"fmt"
	"time"

	"daidai-panel/middleware"
	"daidai-panel/model"
	"daidai-panel/pkg/response"
	"daidai-panel/service"

	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	authService  *service.AuthService
	loginLimiter gin.HandlerFunc
}

func NewAuthHandler() *AuthHandler {
	return &AuthHandler{
		authService:  service.NewAuthService(),
		loginLimiter: middleware.RateLimit(5, time.Minute),
	}
}

func (h *AuthHandler) CheckInit(c *gin.Context) {
	response.Success(c, gin.H{"need_init": h.authService.NeedInit()})
}

func (h *AuthHandler) Init(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}

	user, err := h.authService.InitAdmin(req.Username, req.Password)
	if err != nil {
		switch err {
		case service.ErrInvalidUsername:
			response.BadRequest(c, "用户名需 3-32 位，仅支持字母数字下划线")
		case service.ErrPasswordTooShort:
			response.BadRequest(c, "密码长度需 6-128 位")
		default:
			response.BadRequest(c, err.Error())
		}
		return
	}

	response.Success(c, gin.H{
		"message": "初始化成功",
		"user":    user.ToDict(),
	})
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req struct {
		Username string                 `json:"username" binding:"required"`
		Password string                 `json:"password" binding:"required"`
		TOTPCode string                 `json:"totp_code"`
		Captcha  service.CaptchaPayload `json:"captcha"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}

	ip := middleware.ResolveClientIP(c)
	ua := c.GetHeader("User-Agent")
	clientType := service.DetectSessionClientType(
		c.GetHeader("X-Client-Type"),
		c.GetHeader("X-Client-App"),
		ua,
	)

	locked, remaining := service.CheckLoginLock(ip, req.Username)
	if locked {
		service.RecordLoginLog(0, req.Username, ip, ua, 1, "账号已锁定")
		remainSec := int(remaining.Seconds())
		c.JSON(429, gin.H{
			"error":             fmt.Sprintf("账号已锁定，请 %.0f 分钟后重试", remaining.Minutes()),
			"locked":            true,
			"remaining_seconds": remainSec,
		})
		return
	}

	if !service.IsIPWhitelisted(ip) {
		service.RecordLoginLog(0, req.Username, ip, ua, 1, "IP 不在白名单")
		response.Forbidden(c, "当前 IP 不在登录白名单中")
		return
	}

	captchaCfg := service.GetCaptchaRuntimeConfig()
	if service.IsCaptchaRequired(ip, req.Username) {
		verifyResult, err := service.VerifyLoginCaptcha(req.Captcha)
		if err != nil {
			switch err {
			case service.ErrCaptchaRequired:
				c.JSON(401, gin.H{
					"error":                  err.Error(),
					"captcha_required":       true,
					"captcha_id":             captchaCfg.CaptchaID,
					"captcha_threshold":      captchaCfg.RequireAfterFailures,
					"require_after_failures": captchaCfg.RequireAfterFailures,
				})
			default:
				response.InternalError(c, "验证码校验失败，请稍后重试")
			}
			return
		}
		if !verifyResult.Passed {
			if verifyResult.UpstreamError {
				c.JSON(503, gin.H{
					"error":                       "验证码服务暂时不可用，请稍后重试",
					"captcha_required":            true,
					"captcha_service_unavailable": true,
					"captcha_id":                  captchaCfg.CaptchaID,
					"captcha_reason":              verifyResult.Reason,
					"captcha_fail_mode":           captchaCfg.FailMode,
					"captcha_threshold":           captchaCfg.RequireAfterFailures,
					"require_after_failures":      captchaCfg.RequireAfterFailures,
				})
				return
			}

			c.JSON(401, gin.H{
				"error":                  "验证码校验失败，请重新完成人机验证",
				"captcha_required":       true,
				"captcha_invalid":        true,
				"captcha_id":             captchaCfg.CaptchaID,
				"captcha_reason":         verifyResult.Reason,
				"captcha_threshold":      captchaCfg.RequireAfterFailures,
				"require_after_failures": captchaCfg.RequireAfterFailures,
			})
			return
		}
	}

	user, accessToken, refreshToken, accessInfo, refreshInfo, err := h.authService.Login(req.Username, req.Password, req.TOTPCode)
	if err != nil {
		switch err {
		case service.ErrUserNotFound, service.ErrInvalidPassword:
			failedAttempts := service.RecordFailedLogin(ip, req.Username)
			service.RecordLoginLog(0, req.Username, ip, ua, 1, "登录失败")
			c.JSON(401, gin.H{
				"error":                  "用户名或密码错误",
				"failed_attempts":        failedAttempts,
				"captcha_required":       captchaCfg.Enabled && service.ShouldRequireCaptchaByAttempts(failedAttempts),
				"captcha_id":             captchaCfg.CaptchaID,
				"captcha_threshold":      captchaCfg.RequireAfterFailures,
				"require_after_failures": captchaCfg.RequireAfterFailures,
			})
		case service.ErrUserDisabled:
			service.RecordLoginLog(0, req.Username, ip, ua, 1, "登录失败")
			response.Forbidden(c, "账号已被禁用")
		case service.ErrTOTPRequired:
			service.RecordLoginLog(0, req.Username, ip, ua, 1, "登录失败")
			c.JSON(401, gin.H{
				"error":               "请输入两步验证码",
				"two_factor_required": true,
			})
		case service.ErrInvalidTOTP:
			service.RecordLoginLog(0, req.Username, ip, ua, 1, "登录失败")
			c.JSON(401, gin.H{
				"error":               "两步验证码错误",
				"two_factor_required": true,
			})
		default:
			service.RecordFailedLogin(ip, req.Username)
			service.RecordLoginLog(0, req.Username, ip, ua, 1, "登录失败")
			response.InternalError(c, "登录失败")
		}
		return
	}

	service.ClearLoginAttempts(ip, req.Username)
	service.RecordLoginLog(user.ID, user.Username, ip, ua, 0, "登录成功")
	if model.GetRegisteredConfigBool("notify_on_login") {
		go service.SendNotification(
			"登录成功通知",
			fmt.Sprintf("用户 %s 于 %s 从 IP %s 登录成功。",
				user.Username, time.Now().Format("2006-01-02 15:04:05"), ip),
		)
	}
	service.CreateSessionWithRefresh(user.ID, user.Username, accessInfo.JTI, refreshInfo.JTI, clientType, ip, ua, accessInfo.ExpiresAt, refreshInfo.ExpiresAt)

	response.Success(c, gin.H{
		"message":       "登录成功",
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"user":          user.ToDict(),
	})
}

func (h *AuthHandler) Logout(c *gin.Context) {
	jti, _ := c.Get("jti")
	service.RevokeSession(jti.(string))
	response.Success(c, gin.H{"message": "已退出登录"})
}

func (h *AuthHandler) Refresh(c *gin.Context) {
	tokenStr := middleware.ExtractBearerToken(c.GetHeader("Authorization"))

	newToken, err := h.authService.RefreshToken(tokenStr)
	if err != nil {
		response.Unauthorized(c, "令牌无效或已过期")
		return
	}

	response.Success(c, gin.H{"access_token": newToken})
}

func (h *AuthHandler) GetUser(c *gin.Context) {
	username, _ := c.Get("username")
	user, err := h.authService.GetUser(username.(string))
	if err != nil {
		response.NotFound(c, "用户不存在")
		return
	}
	response.Success(c, gin.H{"user": user.ToDict()})
}

func (h *AuthHandler) ChangePassword(c *gin.Context) {
	var req struct {
		OldPassword string `json:"old_password" binding:"required"`
		NewPassword string `json:"new_password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}

	username, _ := c.Get("username")
	if err := h.authService.ChangePassword(username.(string), req.OldPassword, req.NewPassword); err != nil {
		switch err {
		case service.ErrInvalidPassword:
			response.BadRequest(c, "当前密码不正确")
		case service.ErrPasswordTooShort:
			response.BadRequest(c, "新密码长度需 6-128 位")
		default:
			response.InternalError(c, "修改密码失败")
		}
		return
	}

	user, _ := h.authService.GetUser(username.(string))
	if user != nil {
		service.RevokeAllUserSessions(user.ID)
	}

	response.Success(c, gin.H{"message": "密码修改成功，请重新登录"})
}

func (h *AuthHandler) CaptchaConfig(c *gin.Context) {
	cfg := service.GetCaptchaRuntimeConfig()
	username := c.Query("username")

	response.Success(c, gin.H{
		"enabled":                cfg.Enabled,
		"captcha_id":             cfg.CaptchaID,
		"configured":             cfg.Configured,
		"implemented":            true,
		"required":               service.IsCaptchaRequired(middleware.ResolveClientIP(c), username),
		"captcha_fail_mode":      cfg.FailMode,
		"require_after_failures": cfg.RequireAfterFailures,
		"message":                service.BuildCaptchaStatusMessage(cfg),
	})
}

func (h *AuthHandler) RegisterRoutes(r *gin.RouterGroup) {
	auth := r.Group("/auth")
	{
		auth.GET("/check-init", h.CheckInit)
		auth.POST("/init", h.Init)
		auth.POST("/login", h.loginLimiter, h.Login)
		auth.POST("/logout", middleware.JWTAuth(), h.Logout)
		auth.POST("/refresh", h.Refresh)
		auth.GET("/user", middleware.JWTAuth(), h.GetUser)
		auth.PUT("/password", middleware.JWTAuth(), h.ChangePassword)
		auth.GET("/captcha-config", h.CaptchaConfig)
	}
}
