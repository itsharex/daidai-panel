package handler

import (
	"strconv"

	"daidai-panel/database"
	"daidai-panel/model"
	"daidai-panel/pkg/response"

	"github.com/gin-gonic/gin"
)

func (h *TaskHandler) ListViews(c *gin.Context) {
	var views []model.TaskView
	database.DB.Order("id asc").Find(&views)
	response.Success(c, views)
}

func (h *TaskHandler) CreateView(c *gin.Context) {
	var view model.TaskView
	if err := c.ShouldBindJSON(&view); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	if view.Name == "" {
		response.BadRequest(c, "视图名称不能为空")
		return
	}
	if view.Filters == "" {
		view.Filters = "[]"
	}
	if view.SortRules == "" {
		view.SortRules = "[]"
	}
	database.DB.Create(&view)
	response.Success(c, view)
}

func (h *TaskHandler) UpdateView(c *gin.Context) {
	viewID, _ := strconv.ParseUint(c.Param("viewId"), 10, 32)
	var view model.TaskView
	if err := database.DB.First(&view, viewID).Error; err != nil {
		response.NotFound(c, "视图不存在")
		return
	}
	var input model.TaskView
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	updates := map[string]interface{}{}
	if input.Name != "" {
		updates["name"] = input.Name
	}
	if input.Filters != "" {
		updates["filters"] = input.Filters
	}
	if input.SortRules != "" {
		updates["sort_rules"] = input.SortRules
	}
	database.DB.Model(&view).Updates(updates)
	database.DB.First(&view, viewID)
	response.Success(c, view)
}

func (h *TaskHandler) DeleteView(c *gin.Context) {
	viewID, _ := strconv.ParseUint(c.Param("viewId"), 10, 32)
	var view model.TaskView
	if err := database.DB.First(&view, viewID).Error; err != nil {
		response.NotFound(c, "视图不存在")
		return
	}
	database.DB.Delete(&view)
	response.Success(c, gin.H{"message": "已删除"})
}
