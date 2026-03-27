package handler

import (
	"fmt"
	"os"
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
		Path     string `json:"path"`
		Content  string `json:"content"`
		Language string `json:"language"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}

	req.Path = strings.TrimSpace(req.Path)
	req.Language = strings.TrimSpace(req.Language)
	hasInlineContent := strings.TrimSpace(req.Content) != "" && req.Language != ""

	if req.Path == "" && !hasInlineContent {
		response.BadRequest(c, "缺少脚本路径或调试内容")
		return
	}

	var (
		full      string
		ext       string
		workDir   string
		cleanupFn = func() {}
	)

	if hasInlineContent {
		ext = strings.ToLower(scriptLanguageExtMap[strings.ToLower(strings.TrimSpace(req.Language))])
		if ext == "" {
			response.BadRequest(c, "不支持的语言类型")
			return
		}

		tmpDir := filepath.Join(os.TempDir(), "daidai-debug")
		if err := os.MkdirAll(tmpDir, 0o755); err != nil {
			response.InternalError(c, "创建调试目录失败")
			return
		}

		full = filepath.Join(tmpDir, fmt.Sprintf("debug_%d%s", time.Now().UnixMilli(), ext))
		content := req.Content
		if ext == ".sh" {
			content = string(service.NormalizeShellLineEndings([]byte(content)))
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			response.InternalError(c, "创建调试文件失败")
			return
		}
		workDir = tmpDir
		cleanupFn = func() {
			_ = os.Remove(full)
		}
	} else {
		resolvedPath, err := safePath(req.Path, true)
		if err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		full = resolvedPath
		ext = strings.ToLower(filepath.Ext(full))
		workDir = filepath.Dir(full)
	}

	if ext == ".sh" {
		if err := service.NormalizeShellScriptFile(full); err != nil {
			cleanupFn()
			response.InternalError(c, fmt.Sprintf("脚本换行规范化失败: %s", err))
			return
		}
	}
	interpreter, err := scriptRuntimeInterpreter(ext)
	if err != nil {
		cleanupFn()
		response.BadRequest(c, err.Error())
		return
	}

	envMap := buildScriptExecEnv(workDir)
	cmd, cleanup, err := newScriptCommand(interpreter, full, nil, workDir, envMap)
	if err != nil {
		cleanupFn()
		response.InternalError(c, fmt.Sprintf("启动失败: %s", err))
		return
	}

	run := newDebugRun()
	pipeWriter, scanDone, err := startTrackedCommand(cmd, run)
	if err != nil {
		cleanup()
		cleanupFn()
		response.InternalError(c, fmt.Sprintf("启动失败: %s", err))
		return
	}

	runName := filepath.Base(full)
	if req.Path != "" {
		runName = filepath.Base(req.Path)
	}
	runID := fmt.Sprintf("%d_%s", time.Now().UnixMilli(), runName)
	h.storeRun(runID, run)

	startTime := time.Now()

	go func() {
		waitErr := waitTrackedCommand(cmd, pipeWriter, scanDone)
		cleanup()
		cleanupFn()
		elapsed := time.Since(startTime).Seconds()
		exitCode := resolveExitCode(waitErr)

		if run.isStopped() {
			return
		}

		if exitCode != 0 && model.GetRegisteredConfigBool("auto_install_deps") {
			candidate := detectAutoInstallCandidate(ext, run.logOutput(), workDir)
			if candidate != nil {
				run.appendLog(fmt.Sprintf("[检测到缺失依赖: %s，正在自动安装...]", candidate.DisplayName))

				installResult := installDepForDebug(candidate, envMap)
				if run.isStopped() {
					return
				}
				if installResult.Success {
					run.appendLog(fmt.Sprintf("[安装成功: %s，自动重试执行]", candidate.DisplayName))

					retryCmd, retryCleanup, retryPrepareErr := newScriptCommand(interpreter, full, nil, workDir, envMap)
					if retryPrepareErr != nil {
						run.appendLog(fmt.Sprintf("[重试启动失败: %s]", retryPrepareErr))
						run.finish(exitCode, waitErr, elapsed)
						return
					}
					retryPipeWriter, retryScanDone, startErr := startTrackedCommand(retryCmd, run)
					if startErr == nil {
						waitErr = waitTrackedCommand(retryCmd, retryPipeWriter, retryScanDone)
						retryCleanup()
						elapsed = time.Since(startTime).Seconds()
						exitCode = resolveExitCode(waitErr)
						if run.isStopped() {
							return
						}
					} else {
						retryCleanup()
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
