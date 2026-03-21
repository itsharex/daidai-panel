package handler

import (
	"fmt"
	"strconv"
	"strings"

	"daidai-panel/database"
	"daidai-panel/model"
	panelcron "daidai-panel/pkg/cron"
	"daidai-panel/pkg/response"
	"daidai-panel/service"

	"github.com/gin-gonic/gin"
)

func (h *TaskHandler) Create(c *gin.Context) {
	var req struct {
		Name                   string   `json:"name" binding:"required"`
		Command                string   `json:"command" binding:"required"`
		CronExpression         string   `json:"cron_expression" binding:"required"`
		Timeout                *int     `json:"timeout"`
		MaxRetries             *int     `json:"max_retries"`
		RetryInterval          *int     `json:"retry_interval"`
		NotifyOnFailure        *bool    `json:"notify_on_failure"`
		NotifyOnSuccess        *bool    `json:"notify_on_success"`
		Labels                 []string `json:"labels"`
		DependsOn              *uint    `json:"depends_on"`
		TaskBefore             *string  `json:"task_before"`
		TaskAfter              *string  `json:"task_after"`
		AllowMultipleInstances *bool    `json:"allow_multiple_instances"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}

	result := panelcron.Parse(req.CronExpression)
	if !result.Valid {
		response.BadRequest(c, "无效的 cron 表达式")
		return
	}

	task := model.Task{
		Name:            req.Name,
		Command:         req.Command,
		CronExpression:  req.CronExpression,
		Status:          model.TaskStatusEnabled,
		Timeout:         86400,
		RetryInterval:   60,
		NotifyOnFailure: true,
	}

	if req.Timeout != nil {
		task.Timeout = *req.Timeout
	}
	if req.MaxRetries != nil {
		task.MaxRetries = *req.MaxRetries
	}
	if req.RetryInterval != nil {
		task.RetryInterval = *req.RetryInterval
	}
	if req.NotifyOnFailure != nil {
		task.NotifyOnFailure = *req.NotifyOnFailure
	}
	if req.NotifyOnSuccess != nil {
		task.NotifyOnSuccess = *req.NotifyOnSuccess
	}
	if req.Labels != nil {
		task.SetLabelsFromSlice(req.Labels)
	}
	if req.DependsOn != nil {
		task.DependsOn = req.DependsOn
	}
	if req.TaskBefore != nil {
		task.TaskBefore = req.TaskBefore
	}
	if req.TaskAfter != nil {
		task.TaskAfter = req.TaskAfter
	}
	if req.AllowMultipleInstances != nil {
		task.AllowMultipleInstances = *req.AllowMultipleInstances
	}

	if err := database.DB.Select("*").Create(&task).Error; err != nil {
		response.InternalError(c, "创建任务失败")
		return
	}

	if scheduler := service.GetSchedulerV2(); scheduler != nil {
		scheduler.AddJob(&task)
	}

	response.Created(c, gin.H{
		"message": "创建成功",
		"data":    task.ToDict(),
	})
}

func (h *TaskHandler) Update(c *gin.Context) {
	taskID, _ := strconv.ParseUint(c.Param("id"), 10, 32)

	var task model.Task
	if err := database.DB.First(&task, taskID).Error; err != nil {
		response.NotFound(c, "任务不存在")
		return
	}

	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}

	if labels, ok := req["labels"].([]interface{}); ok {
		values := make([]string, len(labels))
		for i, label := range labels {
			values[i] = fmt.Sprintf("%v", label)
		}
		req["labels"] = strings.Join(values, ",")
	}

	if cronExpr, ok := req["cron_expression"].(string); ok {
		result := panelcron.Parse(cronExpr)
		if !result.Valid {
			response.BadRequest(c, "无效的 cron 表达式")
			return
		}
	}

	allowedFields := map[string]bool{
		"name": true, "command": true, "cron_expression": true,
		"timeout": true, "max_retries": true, "retry_interval": true,
		"notify_on_failure": true, "notify_on_success": true, "labels": true, "depends_on": true,
		"sort_order": true, "task_before": true, "task_after": true,
		"allow_multiple_instances": true,
	}

	updates := make(map[string]interface{})
	for key, value := range req {
		if allowedFields[key] {
			updates[key] = value
		}
	}

	if len(updates) > 0 {
		database.DB.Model(&task).Updates(updates)
	}

	database.DB.First(&task, taskID)
	if scheduler := service.GetSchedulerV2(); scheduler != nil {
		scheduler.UpdateJob(&task)
	}

	response.Success(c, gin.H{
		"message": "task updated",
		"data":    task.ToDict(),
	})
}

func (h *TaskHandler) Delete(c *gin.Context) {
	taskID, _ := strconv.ParseUint(c.Param("id"), 10, 32)

	var task model.Task
	if err := database.DB.First(&task, taskID).Error; err != nil {
		response.NotFound(c, "任务不存在")
		return
	}

	if scheduler := service.GetSchedulerV2(); scheduler != nil {
		scheduler.RemoveJob(uint(taskID))
	}
	database.DB.Where("task_id = ?", taskID).Delete(&model.TaskLog{})
	database.DB.Delete(&task)

	response.Success(c, gin.H{"message": "删除成功"})
}

func (h *TaskHandler) Pin(c *gin.Context) {
	taskID, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	database.DB.Model(&model.Task{}).Where("id = ?", taskID).Update("is_pinned", true)
	response.Success(c, gin.H{"message": "已置顶"})
}

func (h *TaskHandler) Unpin(c *gin.Context) {
	taskID, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	database.DB.Model(&model.Task{}).Where("id = ?", taskID).Update("is_pinned", false)
	response.Success(c, gin.H{"message": "已取消置顶"})
}

func (h *TaskHandler) Copy(c *gin.Context) {
	taskID, _ := strconv.ParseUint(c.Param("id"), 10, 32)

	var task model.Task
	if err := database.DB.First(&task, taskID).Error; err != nil {
		response.NotFound(c, "任务不存在")
		return
	}

	newTask := model.Task{
		Name:                   task.Name + " (副本)",
		Command:                task.Command,
		CronExpression:         task.CronExpression,
		Status:                 model.TaskStatusDisabled,
		Labels:                 task.Labels,
		Timeout:                task.Timeout,
		MaxRetries:             task.MaxRetries,
		RetryInterval:          task.RetryInterval,
		NotifyOnFailure:        task.NotifyOnFailure,
		NotifyOnSuccess:        task.NotifyOnSuccess,
		DependsOn:              task.DependsOn,
		TaskBefore:             task.TaskBefore,
		TaskAfter:              task.TaskAfter,
		AllowMultipleInstances: task.AllowMultipleInstances,
	}
	database.DB.Select("*").Create(&newTask)
	response.Created(c, gin.H{"message": "复制成功", "data": newTask.ToDict()})
}
