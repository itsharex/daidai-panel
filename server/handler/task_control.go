package handler

import (
	"net/http"
	"strconv"
	"time"

	"daidai-panel/database"
	"daidai-panel/model"
	panelcron "daidai-panel/pkg/cron"
	"daidai-panel/pkg/response"
	"daidai-panel/service"

	"github.com/gin-gonic/gin"
)

func (h *TaskHandler) Run(c *gin.Context) {
	taskID, _ := strconv.ParseUint(c.Param("id"), 10, 32)

	var task model.Task
	if err := database.DB.First(&task, taskID).Error; err != nil {
		response.NotFound(c, "任务不存在")
		return
	}

	if task.Status == model.TaskStatusRunning {
		response.BadRequest(c, "任务正在运行中")
		return
	}

	if err := service.GetSchedulerV2().RunNow(uint(taskID)); err != nil {
		response.Error(c, http.StatusServiceUnavailable, "任务入队失败: "+err.Error())
		return
	}
	response.Success(c, gin.H{"message": "任务已启动"})
}

func (h *TaskHandler) Stop(c *gin.Context) {
	taskID, _ := strconv.ParseUint(c.Param("id"), 10, 32)

	var task model.Task
	if err := database.DB.First(&task, taskID).Error; err != nil {
		response.NotFound(c, "任务不存在")
		return
	}

	stopped := service.GetTaskExecutor().StopTask(uint(taskID))
	if !stopped {
		if scheduler := service.GetScheduler(); scheduler != nil {
			stopped = scheduler.StopRunningTask(uint(taskID))
		}
	}

	if task.PID != nil && *task.PID > 0 {
		service.KillProcessByPid(*task.PID)
	}

	inactiveStatus := service.ResolveTaskInactiveStatus(&task)
	database.DB.Model(&task).Updates(map[string]interface{}{
		"status":   inactiveStatus,
		"pid":      nil,
		"log_path": nil,
	})

	var runningLog model.TaskLog
	if err := database.DB.Where("task_id = ? AND status = ?", taskID, model.LogStatusRunning).
		Order("started_at DESC").First(&runningLog).Error; err == nil {
		now := time.Now()
		database.DB.Model(&runningLog).Updates(map[string]interface{}{
			"status":   model.LogStatusFailed,
			"ended_at": now,
		})
	}

	response.Success(c, gin.H{"message": "任务已停止"})
}

func (h *TaskHandler) Enable(c *gin.Context) {
	taskID, _ := strconv.ParseUint(c.Param("id"), 10, 32)

	var task model.Task
	if err := database.DB.First(&task, taskID).Error; err != nil {
		response.NotFound(c, "任务不存在")
		return
	}

	if task.UsesCronSchedule() {
		result := panelcron.Parse(task.CronExpression)
		if !result.Valid {
			response.BadRequest(c, "无效的 cron 表达式，无法启用")
			return
		}
	}

	task.Status = model.TaskStatusEnabled
	database.DB.Save(&task)
	service.GetSchedulerV2().AddJob(&task)
	response.Success(c, gin.H{"message": "已启用", "data": task.ToDict()})
}

func (h *TaskHandler) Disable(c *gin.Context) {
	taskID, _ := strconv.ParseUint(c.Param("id"), 10, 32)

	var task model.Task
	if err := database.DB.First(&task, taskID).Error; err != nil {
		response.NotFound(c, "任务不存在")
		return
	}

	task.Status = model.TaskStatusDisabled
	database.DB.Save(&task)
	service.GetSchedulerV2().RemoveJob(uint(taskID))
	response.Success(c, gin.H{"message": "已禁用", "data": task.ToDict()})
}
