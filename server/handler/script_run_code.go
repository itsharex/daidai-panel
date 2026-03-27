package handler

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"daidai-panel/pkg/response"
	"daidai-panel/service"

	"github.com/gin-gonic/gin"
)

var (
	debugNodeModuleRe = regexp.MustCompile(`(?:Cannot find module|Error \[ERR_MODULE_NOT_FOUND\].*)\s*'([^']+)'`)
	debugPyModuleRe   = regexp.MustCompile(`(?:ModuleNotFoundError|ImportError):\s*No module named\s+'([^']+)'`)
)

func (h *ScriptHandler) RunCode(c *gin.Context) {
	var req struct {
		Code     string `json:"code" binding:"required"`
		Language string `json:"language" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}

	ext, ok := scriptLanguageExtMap[req.Language]
	if !ok {
		response.BadRequest(c, "不支持的语言类型")
		return
	}

	tmpDir := filepath.Join(os.TempDir(), "daidai-debug")
	os.MkdirAll(tmpDir, 0755)

	tmpFile := filepath.Join(tmpDir, fmt.Sprintf("code_%d%s", time.Now().UnixMilli(), ext))
	if err := os.WriteFile(tmpFile, []byte(req.Code), 0644); err != nil {
		response.InternalError(c, "创建临时文件失败")
		return
	}

	interpreter, err := scriptRuntimeInterpreter(ext)
	if err != nil {
		os.Remove(tmpFile)
		response.BadRequest(c, err.Error())
		return
	}
	envMap := buildScriptExecEnv(tmpDir)
	cmd, cleanup, err := newScriptCommand(interpreter, tmpFile, nil, tmpDir, envMap)
	if err != nil {
		os.Remove(tmpFile)
		response.InternalError(c, fmt.Sprintf("启动失败: %s", err))
		return
	}

	run := newDebugRun()
	pipeWriter, scanDone, err := startTrackedCommand(cmd, run)
	if err != nil {
		cleanup()
		os.Remove(tmpFile)
		response.InternalError(c, fmt.Sprintf("启动失败: %s", err))
		return
	}

	runID := fmt.Sprintf("code_%d", time.Now().UnixMilli())
	h.storeRun(runID, run)

	startTime := time.Now()

	go func() {
		waitErr := waitTrackedCommand(cmd, pipeWriter, scanDone)
		cleanup()
		os.Remove(tmpFile)
		run.finish(resolveExitCode(waitErr), waitErr, time.Since(startTime).Seconds())
	}()

	response.Created(c, gin.H{"message": "代码已启动", "run_id": runID})
}

func detectMissingDep(output string) string {
	if matches := debugNodeModuleRe.FindStringSubmatch(output); len(matches) > 1 {
		mod := matches[1]
		if !strings.HasPrefix(mod, ".") && !strings.HasPrefix(mod, "/") {
			return mod
		}
	}
	if matches := debugPyModuleRe.FindStringSubmatch(output); len(matches) > 1 {
		return strings.Split(matches[1], ".")[0]
	}
	return ""
}

func detectAutoInstallCandidate(ext, output, workDir string) *service.AutoInstallCandidate {
	return service.DetectAutoInstallCandidate(ext, output, workDir)
}

func installDepForDebug(candidate *service.AutoInstallCandidate, envMap map[string]string) service.AutoInstallResult {
	return service.InstallAutoDependency(candidate, envMap)
}
