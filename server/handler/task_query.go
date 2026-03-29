package handler

import (
	"sort"
	"strconv"
	"strings"

	"daidai-panel/database"
	"daidai-panel/model"
	panelcron "daidai-panel/pkg/cron"
	"daidai-panel/pkg/response"
	"daidai-panel/service"

	"github.com/gin-gonic/gin"
)

func (h *TaskHandler) List(c *gin.Context) {
	keyword := c.Query("keyword")
	statusStr := c.Query("status")
	label := c.Query("label")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	query := database.DB.Model(&model.Task{})

	if keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("name LIKE ? OR command LIKE ?", like, like)
	}
	if statusStr != "" {
		status, err := strconv.ParseFloat(statusStr, 64)
		if err == nil {
			query = query.Where("status = ?", status)
		}
	}
	if label != "" {
		query = query.Where("labels LIKE ?", "%"+label+"%")
	}

	var total int64
	query.Count(&total)

	var tasks []model.Task
	query.Find(&tasks)

	sort.SliceStable(tasks, func(i, j int) bool {
		left := tasks[i]
		right := tasks[j]

		leftGroup := taskSortGroup(left.Status)
		rightGroup := taskSortGroup(right.Status)
		if leftGroup != rightGroup {
			return leftGroup < rightGroup
		}
		if left.IsPinned != right.IsPinned {
			return left.IsPinned
		}
		if left.SortOrder != right.SortOrder {
			return left.SortOrder < right.SortOrder
		}
		return left.CreatedAt.After(right.CreatedAt)
	})

	start := (page - 1) * pageSize
	if start > len(tasks) {
		start = len(tasks)
	}
	end := start + pageSize
	if end > len(tasks) {
		end = len(tasks)
	}
	tasks = tasks[start:end]
	subscriptionNames := loadSubscriptionNameMap(tasks)
	notificationChannels := loadTaskNotificationChannelMap(tasks)

	data := make([]map[string]interface{}, len(tasks))
	for i, task := range tasks {
		item := task.ToDict()
		item["display_labels"] = buildTaskDisplayLabels(task.GetLabels(), subscriptionNames)
		if task.NotificationChannelID != nil {
			if channel, exists := notificationChannels[*task.NotificationChannelID]; exists {
				item["notification_channel_name"] = channel.Name
				item["notification_channel_enabled"] = channel.Enabled
			}
		}
		if task.Status != model.TaskStatusDisabled && task.UsesCronSchedule() && task.CronExpression != "" {
			nextTimes := panelcron.NextRunTimesForExpressions(task.CronExpression, 1)
			if len(nextTimes) > 0 {
				item["next_run_at"] = nextTimes[0]
			}
		}
		data[i] = item
	}

	response.Paginated(c, data, total, page, pageSize)
}

func taskSortGroup(status float64) int {
	switch status {
	case model.TaskStatusEnabled, model.TaskStatusQueued, model.TaskStatusRunning:
		return 0
	case model.TaskStatusDisabled:
		return 1
	default:
		return 2
	}
}

func loadSubscriptionNameMap(tasks []model.Task) map[uint]string {
	subscriptionIDs := make(map[uint]struct{})
	for _, task := range tasks {
		for _, label := range task.GetLabels() {
			if !strings.HasPrefix(label, "subscription:") {
				continue
			}
			rawID := strings.TrimSpace(strings.TrimPrefix(label, "subscription:"))
			subID, err := strconv.ParseUint(rawID, 10, 32)
			if err != nil {
				continue
			}
			subscriptionIDs[uint(subID)] = struct{}{}
		}
	}

	if len(subscriptionIDs) == 0 {
		return map[uint]string{}
	}

	ids := make([]uint, 0, len(subscriptionIDs))
	for id := range subscriptionIDs {
		ids = append(ids, id)
	}

	var subscriptions []model.Subscription
	database.DB.Model(&model.Subscription{}).
		Where("id IN ?", ids).
		Find(&subscriptions)

	result := make(map[uint]string, len(subscriptions))
	for _, sub := range subscriptions {
		result[sub.ID] = strings.TrimSpace(sub.Name)
	}
	return result
}

func buildTaskDisplayLabels(labels []string, subscriptionNames map[uint]string) []string {
	displayLabels := make([]string, 0, len(labels))
	seen := make(map[string]struct{})

	addLabel := func(label string) {
		label = strings.TrimSpace(label)
		if label == "" {
			return
		}
		if _, exists := seen[label]; exists {
			return
		}
		seen[label] = struct{}{}
		displayLabels = append(displayLabels, label)
	}

	for _, label := range labels {
		if !strings.HasPrefix(label, "subscription:") {
			addLabel(label)
			continue
		}

		rawID := strings.TrimSpace(strings.TrimPrefix(label, "subscription:"))
		subID, err := strconv.ParseUint(rawID, 10, 32)
		if err != nil {
			continue
		}

		if subName := subscriptionNames[uint(subID)]; subName != "" {
			addLabel(subName)
			continue
		}

		addLabel("订阅任务")
	}

	return displayLabels
}

func (h *TaskHandler) Stats(c *gin.Context) {
	taskID, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	daysStr := c.DefaultQuery("days", "7")
	days, _ := strconv.Atoi(daysStr)
	if days < 1 {
		days = 7
	}

	stats := service.GetTaskStats(uint(taskID), days)
	if stats == nil {
		response.NotFound(c, "任务不存在")
		return
	}
	response.Success(c, stats)
}
