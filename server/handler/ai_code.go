package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"daidai-panel/middleware"
	"daidai-panel/pkg/response"
	"daidai-panel/service"

	"github.com/gin-gonic/gin"
)

type AICodeHandler struct{}

func NewAICodeHandler() *AICodeHandler {
	return &AICodeHandler{}
}

func (h *AICodeHandler) Config(c *gin.Context) {
	response.Success(c, gin.H{
		"data": gin.H{
			"enabled":          service.AICodeFeatureEnabled(),
			"default_provider": service.DefaultAICodeProvider(),
			"providers":        service.ListAICodeProviders(),
		},
	})
}

func (h *AICodeHandler) GenerateStream(c *gin.Context) {
	var req service.AICodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		response.InternalError(c, "当前环境不支持流式输出")
		return
	}

	writeEvent := func(event string, payload interface{}) error {
		body, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(c.Writer, "event: %s\ndata: %s\n\n", event, body); err != nil {
			return err
		}
		flusher.Flush()
		return nil
	}

	if err := writeEvent("start", gin.H{"status": "started"}); err != nil {
		return
	}

	result, err := service.GenerateAICodeStream(c.Request.Context(), req, func(text string) error {
		if err := c.Request.Context().Err(); err != nil {
			return err
		}
		return writeEvent("delta", gin.H{"text": text})
	})
	if err != nil {
		if c.Request.Context().Err() != nil {
			return
		}
		_ = writeEvent("error", gin.H{"error": err.Error()})
		return
	}

	_ = writeEvent("done", gin.H{"data": result})
}

func (h *AICodeHandler) TestConnection(c *gin.Context) {
	var req service.AICodeProviderTestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}

	result, err := service.TestAICodeProviderConnection(req)
	if err != nil {
		classifyAIError(c, err)
		return
	}

	response.Success(c, gin.H{"data": result})
}

func classifyAIError(c *gin.Context, err error) {
	var upstream *service.AIUpstreamError
	if errors.As(err, &upstream) {
		response.Error(c, http.StatusBadGateway, err.Error())
		return
	}
	response.BadRequest(c, err.Error())
}

func (h *AICodeHandler) RegisterRoutes(r *gin.RouterGroup) {
	ai := r.Group("/ai-code", middleware.JWTAuth(), middleware.RequireRole("operator"), middleware.RateLimit(10, time.Minute))
	{
		ai.GET("/config", h.Config)
		ai.POST("/generate-stream", h.GenerateStream)
	}

	aiAdmin := r.Group("/ai-code", middleware.JWTAuth(), middleware.RequireAdmin())
	{
		aiAdmin.POST("/test", h.TestConnection)
	}
}
