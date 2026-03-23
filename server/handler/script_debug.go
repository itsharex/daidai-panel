package handler

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"daidai-panel/model"
	"daidai-panel/pkg/response"
	"daidai-panel/service"

	"github.com/gin-gonic/gin"
)

func (h *ScriptHandler) DebugRun(c *gin.Context) {
	var req struct {
		Path string `json:"path" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}

	full, err := safePath(req.Path, true)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	ext := strings.ToLower(filepath.Ext(full))
	cmdParts, err := scriptCommandParts(ext, full)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	workDir := filepath.Dir(full)
	env, envMap := buildScriptExecEnv(workDir)
	cmd := newScriptCommand(cmdParts, workDir, env)

	run := newDebugRun()
	pipeWriter, scanDone, err := startTrackedCommand(cmd, run)
	if err != nil {
		response.InternalError(c, fmt.Sprintf("启动失败: %s", err))
		return
	}

	runID := fmt.Sprintf("%d_%s", time.Now().UnixMilli(), filepath.Base(req.Path))
	h.storeRun(runID, run)

	startTime := time.Now()

	go func() {
		waitErr := waitTrackedCommand(cmd, pipeWriter, scanDone)
		elapsed := time.Since(startTime).Seconds()
		exitCode := resolveExitCode(waitErr)

		if exitCode != 0 && model.GetRegisteredConfigBool("auto_install_deps") {
			candidate := detectAutoInstallCandidate(ext, run.logOutput(), workDir)
			if candidate != nil {
				run.appendLog(fmt.Sprintf("[检测到缺失依赖: %s，正在自动安装...]", candidate.DisplayName))

				installResult := installDepForDebug(candidate, envMap)
				if installResult.Success {
					run.appendLog(fmt.Sprintf("[安装成功: %s，自动重试执行]", candidate.DisplayName))

					retryCmd := newScriptCommand(cmdParts, workDir, env)
					service.SetPgid(retryCmd)

					retryPipeWriter, retryScanDone, startErr := startTrackedCommand(retryCmd, run)
					if startErr == nil {
						waitErr = waitTrackedCommand(retryCmd, retryPipeWriter, retryScanDone)
						elapsed = time.Since(startTime).Seconds()
						exitCode = resolveExitCode(waitErr)
					} else {
						run.appendLog(fmt.Sprintf("[重试启动失败: %s]", startErr))
					}
				} else {
					failureReason := strings.TrimSpace(installResult.Error)
					if failureReason == "" {
						failureReason = candidate.DisplayName
					}
					run.appendLog(fmt.Sprintf("[安装失败: %s]", failureReason))
				}
			}
		}

		run.finish(exitCode, waitErr, elapsed)
	}()

	response.Created(c, gin.H{"message": "脚本已启动", "run_id": runID})
}

func (h *ScriptHandler) DebugLogs(c *gin.Context) {
	runID := c.Param("run_id")

	run, exists := h.loadRun(runID)
	if !exists {
		response.NotFound(c, "运行记录不存在")
		return
	}

	logs, done, exitCode, status := run.snapshot()
	response.Success(c, gin.H{
		"data": gin.H{
			"logs":      logs,
			"done":      done,
			"exit_code": exitCode,
			"status":    status,
		},
	})
}

func (h *ScriptHandler) DebugStop(c *gin.Context) {
	runID := c.Param("run_id")

	run, exists := h.loadRun(runID)
	if !exists {
		response.NotFound(c, "运行记录不存在")
		return
	}

	run.stop()
	response.Success(c, gin.H{"message": "已停止"})
}

func (h *ScriptHandler) DebugClear(c *gin.Context) {
	runID := c.Param("run_id")

	run, exists := h.deleteRun(runID)
	if exists {
		run.killIfRunning()
	}

	response.Success(c, gin.H{"message": "已清除"})
}
