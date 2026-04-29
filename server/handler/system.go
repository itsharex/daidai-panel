package handler

import (
	"bufio"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"daidai-panel/config"
	"daidai-panel/database"
	"daidai-panel/middleware"
	"daidai-panel/model"
	"daidai-panel/pkg/response"
	"daidai-panel/service"

	"github.com/gin-gonic/gin"
)

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

const (
	systemHealthLastCheckedAtKey = "system_health_last_checked_at"
	systemHealthLastResultKey    = "system_health_last_result_json"
)

type systemHealthCheckItem struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

type systemHealthSnapshot struct {
	Items         []systemHealthCheckItem `json:"items"`
	LastCheckedAt string                  `json:"last_checked_at,omitempty"`
}

var (
	// systemHealthCheckHTTPClient 默认为 nil，运行时按需用 service.NewHTTPClient 构建，
	// 这样能实时读取 system_configs.proxy_url，让 "网络代理" 配置变更立即对健康检查生效。
	// 单元测试可直接赋值非 nil 客户端进行 mock。
	systemHealthCheckHTTPClient *http.Client
	systemHealthCheckURL        = "https://www.baidu.com"
)

func resolveSystemHealthCheckClient() *http.Client {
	if systemHealthCheckHTTPClient != nil {
		return systemHealthCheckHTTPClient
	}
	return service.NewHTTPClient(3 * time.Second)
}

type SystemHandler struct{}

func NewSystemHandler() *SystemHandler {
	return &SystemHandler{}
}

func (h *SystemHandler) Info(c *gin.Context) {
	info := service.GetResourceInfo()
	response.Success(c, gin.H{"data": info})
}

// MachineCode 单独返回面板机器码，便于外部工具通过接口直接获取（无需解析完整系统信息）。
func (h *SystemHandler) MachineCode(c *gin.Context) {
	code := service.EnsureMachineCode()
	response.Success(c, gin.H{"data": gin.H{"machine_code": code}})
}

func (h *SystemHandler) Dashboard(c *gin.Context) {
	var taskCount int64
	database.DB.Model(&model.Task{}).Count(&taskCount)

	var enabledTasks int64
	database.DB.Model(&model.Task{}).Where("status = ?", 1).Count(&enabledTasks)

	var runningTasks int64
	database.DB.Model(&model.Task{}).Where("status = ?", 2).Count(&runningTasks)

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	var todayLogs int64
	database.DB.Model(&model.TaskLog{}).Where("created_at >= ?", today).Count(&todayLogs)

	var successLogs int64
	database.DB.Model(&model.TaskLog{}).Where("created_at >= ? AND status = 0", today).Count(&successLogs)

	var failedLogs int64
	database.DB.Model(&model.TaskLog{}).Where("created_at >= ? AND status = 1", today).Count(&failedLogs)

	var envCount int64
	database.DB.Model(&model.EnvVar{}).Count(&envCount)

	var subCount int64
	database.DB.Model(&model.Subscription{}).Count(&subCount)

	var prevTaskCount int64
	database.DB.Model(&model.Task{}).Where("created_at < ?", today).Count(&prevTaskCount)

	yesterday := today.AddDate(0, 0, -1)
	var yesterdayLogs int64
	database.DB.Model(&model.TaskLog{}).Where("created_at >= ? AND created_at < ?", yesterday, today).Count(&yesterdayLogs)
	var yesterdaySuccess int64
	database.DB.Model(&model.TaskLog{}).Where("created_at >= ? AND created_at < ? AND status = 0", yesterday, today).Count(&yesterdaySuccess)

	var recentLogs []model.TaskLog
	database.DB.Preload("Task").Order("created_at DESC").Limit(10).Find(&recentLogs)

	recentData := make([]map[string]interface{}, len(recentLogs))
	for i, l := range recentLogs {
		recentData[i] = l.ToDict()
	}

	rangeDays := 7
	if r := c.Query("range"); r != "" {
		if n, err := strconv.Atoi(r); err == nil && n > 0 && n <= 90 {
			rangeDays = n
		}
	}

	type DailyStat struct {
		Date    string `json:"date"`
		Success int64  `json:"success"`
		Failed  int64  `json:"failed"`
	}

	var dailyStats []DailyStat
	for i := rangeDays - 1; i >= 0; i-- {
		day := today.AddDate(0, 0, -i)
		nextDay := day.Add(24 * time.Hour)
		date := day.Format("01-02")

		var s, f int64
		database.DB.Model(&model.TaskLog{}).Where("created_at >= ? AND created_at < ? AND status = 0", day, nextDay).Count(&s)
		database.DB.Model(&model.TaskLog{}).Where("created_at >= ? AND created_at < ? AND status = 1", day, nextDay).Count(&f)
		dailyStats = append(dailyStats, DailyStat{Date: date, Success: s, Failed: f})
	}

	response.Success(c, gin.H{
		"data": gin.H{
			"task_count":        taskCount,
			"enabled_tasks":     enabledTasks,
			"running_tasks":     runningTasks,
			"today_logs":        todayLogs,
			"success_logs":      successLogs,
			"failed_logs":       failedLogs,
			"env_count":         envCount,
			"sub_count":         subCount,
			"prev_task_count":   prevTaskCount,
			"yesterday_logs":    yesterdayLogs,
			"yesterday_success": yesterdaySuccess,
			"recent_logs":       recentData,
			"daily_stats":       dailyStats,
			"range_days":        rangeDays,
		},
	})
}

func (h *SystemHandler) Stats(c *gin.Context) {
	var taskCount, enabledTasks, disabledTasks, runningTasks int64
	database.DB.Model(&model.Task{}).Count(&taskCount)
	database.DB.Model(&model.Task{}).Where("status = ?", 1).Count(&enabledTasks)
	database.DB.Model(&model.Task{}).Where("status = ?", 0).Count(&disabledTasks)
	database.DB.Model(&model.Task{}).Where("status = ?", 2).Count(&runningTasks)

	var totalLogs, successLogs, failedLogs int64
	database.DB.Model(&model.TaskLog{}).Count(&totalLogs)
	database.DB.Model(&model.TaskLog{}).Where("status = 0").Count(&successLogs)
	database.DB.Model(&model.TaskLog{}).Where("status = 1").Count(&failedLogs)

	successRate := 0.0
	if totalLogs > 0 {
		successRate = float64(successLogs) / float64(totalLogs) * 100
	}

	scriptCount := service.CountScriptFiles(config.C.Data.ScriptsDir)

	response.Success(c, gin.H{
		"data": gin.H{
			"tasks": gin.H{
				"total":    taskCount,
				"enabled":  enabledTasks,
				"disabled": disabledTasks,
				"running":  runningTasks,
			},
			"logs": gin.H{
				"total":        totalLogs,
				"success":      successLogs,
				"failed":       failedLogs,
				"success_rate": successRate,
			},
			"scripts": gin.H{
				"total": scriptCount,
			},
		},
	})
}

func (h *SystemHandler) Backup(c *gin.Context) {
	var req struct {
		Password  string                  `json:"password"`
		Name      string                  `json:"name"`
		Selection service.BackupSelection `json:"selection"`
	}
	c.ShouldBindJSON(&req)

	filePath, err := service.CreateBackup(service.BackupCreateOptions{
		Password:  req.Password,
		Name:      req.Name,
		Selection: req.Selection.NormalizeDefaults(),
	})
	if err != nil {
		response.InternalError(c, "备份失败: "+err.Error())
		return
	}
	response.Success(c, gin.H{"message": "备份成功", "data": gin.H{"path": filePath}})
}

func (h *SystemHandler) BackupList(c *gin.Context) {
	backups, err := service.ListBackups()
	if err != nil {
		response.InternalError(c, "获取备份列表失败")
		return
	}
	response.Success(c, gin.H{"data": backups})
}

func (h *SystemHandler) Restore(c *gin.Context) {
	var req struct {
		Filename string `json:"filename" binding:"required"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}

	if err := service.RestoreBackup(req.Filename, req.Password); err != nil {
		response.InternalError(c, "恢复失败: "+err.Error())
		return
	}
	response.Success(c, gin.H{"message": "恢复成功"})
}

func (h *SystemHandler) RestoreProgress(c *gin.Context) {
	response.Success(c, gin.H{"data": service.CurrentRestoreProgress()})
}

func (h *SystemHandler) DeleteBackup(c *gin.Context) {
	filename := c.Query("filename")
	if filename == "" {
		response.BadRequest(c, "文件名不能为空")
		return
	}
	service.DeleteBackup(filename)
	response.Success(c, gin.H{"message": "删除成功"})
}

func (h *SystemHandler) UploadBackup(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		response.BadRequest(c, "请选择备份文件")
		return
	}

	if file.Size > 512*1024*1024 {
		response.BadRequest(c, "文件过大，最大 512MB")
		return
	}

	filename := filepath.Base(file.Filename)
	lowerName := strings.ToLower(filename)
	if !strings.HasSuffix(lowerName, ".json") &&
		!strings.HasSuffix(lowerName, ".enc") &&
		!strings.HasSuffix(lowerName, ".tgz") &&
		!strings.HasSuffix(lowerName, ".tar.gz") {
		response.BadRequest(c, "仅支持 .json、.enc、.tgz 或 .tar.gz 备份文件")
		return
	}

	backupDir := filepath.Join(config.C.Data.Dir, "backups")
	os.MkdirAll(backupDir, 0755)
	dst := filepath.Join(backupDir, filename)

	if err := c.SaveUploadedFile(file, dst); err != nil {
		response.InternalError(c, "保存文件失败")
		return
	}

	response.Success(c, gin.H{"message": "上传成功", "data": gin.H{"filename": filename}})
}

func (h *SystemHandler) DownloadBackup(c *gin.Context) {
	filename := c.Param("filename")
	if filename == "" {
		response.BadRequest(c, "文件名不能为空")
		return
	}

	backupDir := filepath.Join(config.C.Data.Dir, "backups")
	filePath := filepath.Join(backupDir, filepath.Base(filename))

	c.FileAttachment(filePath, filename)
}

func (h *SystemHandler) Version(c *gin.Context) {
	response.Success(c, gin.H{
		"data": gin.H{
			"version":     Version,
			"api_version": "v1",
			"framework":   "gin",
			"go_version":  service.GetResourceInfo().GoVersion,
		},
	})
}

func (h *SystemHandler) PublicVersion(c *gin.Context) {
	response.Success(c, gin.H{
		"version": Version,
		"data": gin.H{
			"version": Version,
		},
	})
}

func (h *SystemHandler) PanelSettings(c *gin.Context) {
	title := model.GetRegisteredConfig("panel_title")
	icon := model.GetRegisteredConfig("panel_icon")
	editorBackgroundColor := model.GetRegisteredConfig("editor_background_color")
	logBackgroundColor := model.GetRegisteredConfig("log_background_color")
	logBackgroundImage := model.GetRegisteredConfig("log_background_image")
	response.Success(c, gin.H{
		"data": gin.H{
			"panel_title":             title,
			"panel_icon":              icon,
			"editor_background_color": editorBackgroundColor,
			"log_background_color":    logBackgroundColor,
			"log_background_image":    logBackgroundImage,
		},
	})
}

func (h *SystemHandler) CheckUpdate(c *gin.Context) {
	currentVersion := Version

	release, err := fetchLatestPanelRelease()
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	latestVersion := release.version()
	hasUpdate := compareVersions(currentVersion, latestVersion)
	autoUpdateSupported := true
	updateDisabledReason := ""
	updateTarget := gin.H{}

	plan, planErr := buildPanelUpdatePlanForRelease(release)
	if planErr != nil {
		autoUpdateSupported = false
		updateDisabledReason = planErr.Error()
	} else {
		updateTarget = buildPanelUpdateTarget(plan)
	}

	response.Success(c, gin.H{
		"data": gin.H{
			"current":                currentVersion,
			"latest":                 latestVersion,
			"has_update":             hasUpdate,
			"release_name":           release.Name,
			"release_url":            release.HTMLURL,
			"release_notes":          release.Body,
			"published_at":           release.PublishedAt,
			"auto_update_supported":  autoUpdateSupported,
			"update_disabled_reason": updateDisabledReason,
			"update_target":          updateTarget,
		},
	})
}

func (h *SystemHandler) UpdatePanel(c *gin.Context) {
	plan, err := buildPanelUpdatePlan()
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	if err := panelUpdater.begin(plan); err != nil {
		respondUpdateConflict(c, err.Error())
		return
	}

	go executePanelUpdate(plan)

	response.Success(c, gin.H{
		"data": panelUpdater.snapshotCopy(),
	})
}

func (h *SystemHandler) Restart(c *gin.Context) {
	response.Success(c, gin.H{"message": "面板将在 2 秒后重启"})

	go func() {
		time.Sleep(2 * time.Second)
		os.Exit(1)
	}()
}

func (h *SystemHandler) PanelLog(c *gin.Context) {
	linesStr := c.DefaultQuery("lines", "100")
	keyword := c.Query("keyword")

	lines, _ := strconv.Atoi(linesStr)
	if lines <= 0 || lines > 10000 {
		lines = 100
	}

	logFile := filepath.Join(config.C.Data.Dir, "panel.log")
	file, err := os.Open(logFile)
	if err != nil {
		response.Success(c, gin.H{"data": gin.H{"logs": []string{}}})
		return
	}
	defer file.Close()

	var allLines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if keyword == "" || strings.Contains(line, keyword) {
			allLines = append(allLines, line)
		}
	}

	start := len(allLines) - lines
	if start < 0 {
		start = 0
	}

	response.Success(c, gin.H{
		"data": gin.H{
			"logs":  allLines[start:],
			"total": len(allLines),
		},
	})
}

func runSystemHealthChecks() []systemHealthCheckItem {
	items := make([]systemHealthCheckItem, 0, 4)

	if err := database.DB.Exec("SELECT 1").Error; err != nil {
		items = append(items, systemHealthCheckItem{Name: "database", Status: "error", Message: err.Error()})
	} else {
		items = append(items, systemHealthCheckItem{Name: "database", Status: "ok"})
	}

	info := service.GetResourceInfo()
	memThreshold := float64(model.GetRegisteredConfigInt("memory_warn"))
	if memThreshold <= 0 {
		memThreshold = 80
	}
	if info.MemoryUsage > memThreshold {
		items = append(items, systemHealthCheckItem{Name: "memory", Status: "warning", Message: strconv.FormatFloat(info.MemoryUsage, 'f', 1, 64) + "%"})
	} else {
		items = append(items, systemHealthCheckItem{Name: "memory", Status: "ok", Message: strconv.FormatFloat(info.MemoryUsage, 'f', 1, 64) + "%"})
	}

	if sched := service.GetScheduler(); sched != nil {
		items = append(items, systemHealthCheckItem{Name: "scheduler", Status: "ok", Message: "运行中"})
	} else if schedV2 := service.GetSchedulerV2(); schedV2 != nil {
		items = append(items, systemHealthCheckItem{Name: "scheduler", Status: "ok", Message: "运行中"})
	} else {
		items = append(items, systemHealthCheckItem{Name: "scheduler", Status: "ok", Message: "空闲"})
	}

	if resp, err := resolveSystemHealthCheckClient().Get(systemHealthCheckURL); err != nil {
		items = append(items, systemHealthCheckItem{Name: "network", Status: "error", Message: "无法连接外部网络"})
	} else {
		resp.Body.Close()
		if resp.StatusCode >= http.StatusBadRequest {
			items = append(items, systemHealthCheckItem{Name: "network", Status: "error", Message: "网络检查返回状态异常"})
		} else {
			items = append(items, systemHealthCheckItem{Name: "network", Status: "ok"})
		}
	}

	return items
}

func loadSystemHealthSnapshot() systemHealthSnapshot {
	snapshot := systemHealthSnapshot{
		Items:         []systemHealthCheckItem{},
		LastCheckedAt: strings.TrimSpace(model.GetConfig(systemHealthLastCheckedAtKey, "")),
	}

	rawItems := strings.TrimSpace(model.GetConfig(systemHealthLastResultKey, ""))
	if rawItems == "" {
		return snapshot
	}

	if err := json.Unmarshal([]byte(rawItems), &snapshot.Items); err != nil {
		snapshot.Items = []systemHealthCheckItem{}
	}

	return snapshot
}

func saveSystemHealthSnapshot(items []systemHealthCheckItem, checkedAt time.Time) error {
	rawItems, err := json.Marshal(items)
	if err != nil {
		return err
	}

	if err := model.SetConfig(systemHealthLastResultKey, string(rawItems)); err != nil {
		return err
	}
	if err := model.SetConfig(systemHealthLastCheckedAtKey, checkedAt.Format(time.RFC3339)); err != nil {
		return err
	}
	return nil
}

func buildSystemHealthSnapshot(items []systemHealthCheckItem, checkedAt string) gin.H {
	return gin.H{
		"items":           items,
		"last_checked_at": checkedAt,
	}
}

func (h *SystemHandler) HealthCheck(c *gin.Context) {
	snapshot := loadSystemHealthSnapshot()
	response.Success(c, buildSystemHealthSnapshot(snapshot.Items, snapshot.LastCheckedAt))
}

func (h *SystemHandler) RunHealthCheck(c *gin.Context) {
	items := runSystemHealthChecks()
	checkedAt := time.Now()

	if err := saveSystemHealthSnapshot(items, checkedAt); err != nil {
		response.InternalError(c, "保存健康检查结果失败: "+err.Error())
		return
	}

	response.Success(c, buildSystemHealthSnapshot(items, checkedAt.Format(time.RFC3339)))
}

func (h *SystemHandler) RegisterRoutes(r *gin.RouterGroup) {
	r.GET("/system/public-version", h.PublicVersion)
	r.GET("/system/panel-settings", h.PanelSettings)

	sys := r.Group("/system", middleware.JWTAuth())
	{
		sys.GET("/info", middleware.OpenAPIAccess("system"), middleware.RequireRole("viewer"), h.Info)
		sys.GET("/machine-code", middleware.OpenAPIAccess("system"), middleware.RequireRole("viewer"), h.MachineCode)
		sys.GET("/dashboard", middleware.OpenAPIAccess("system"), middleware.RequireRole("viewer"), h.Dashboard)
		sys.GET("/stats", middleware.OpenAPIAccess("system"), middleware.RequireRole("viewer"), h.Stats)
		sys.GET("/version", middleware.OpenAPIAccess("system"), middleware.RequireRole("viewer"), h.Version)
		sys.GET("/check-update", middleware.OpenAPIAccess("system"), middleware.RequireRole("viewer"), h.CheckUpdate)
		sys.GET("/health-check", middleware.RequireRole("viewer"), h.HealthCheck)
		sys.POST("/health-check", middleware.RequireRole("viewer"), h.RunHealthCheck)
		sys.GET("/update-status", middleware.RequireAdmin(), h.UpdateStatus)
		sys.POST("/update", middleware.RequireAdmin(), h.UpdatePanel)
		sys.POST("/restart", middleware.RequireAdmin(), h.Restart)
		sys.GET("/panel-log", middleware.RequireUserToken(), middleware.RequireAdmin(), h.PanelLog)
		sys.POST("/backup", middleware.OpenAPIAccess("backup"), middleware.RequireRole("admin"), h.Backup)
		sys.POST("/backup/upload", middleware.OpenAPIAccess("backup"), middleware.RequireRole("admin"), h.UploadBackup)
		sys.GET("/backups", middleware.OpenAPIAccess("backup"), middleware.RequireRole("admin"), h.BackupList)
		sys.GET("/backup/download/:filename", middleware.OpenAPIAccess("backup"), middleware.RequireRole("admin"), h.DownloadBackup)
		sys.GET("/restore/progress", middleware.OpenAPIAccess("backup"), middleware.RequireRole("admin"), h.RestoreProgress)
		sys.POST("/restore", middleware.OpenAPIAccess("backup"), middleware.RequireRole("admin"), h.Restore)
		sys.DELETE("/backup", middleware.OpenAPIAccess("backup"), middleware.RequireRole("admin"), h.DeleteBackup)
	}
}
