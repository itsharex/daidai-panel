package service

import (
	"log"
	"strings"

	"daidai-panel/database"
	"daidai-panel/model"
)

var dependencyInstalledFunc = DependencyInstalled
var dependencyReinstallBatchFunc = reinstallDependenciesAsync

func ReconcileDependenciesAfterRestart() {
	var installed []model.Dependency
	database.DB.Where("status = ?", model.DepStatusInstalled).Find(&installed)
	resetCount := 0

	for _, dep := range installed {
		if dependencyInstalledFunc(dep.Type, dep.Name) {
			continue
		}
		database.DB.Model(&dep).Updates(map[string]interface{}{
			"status": model.DepStatusFailed,
			"log":    appendDependencyLog(dep.Log, "[启动校验] 依赖未检测到，可能因容器重建而丢失，请重新安装"),
		})
		resetCount++
		log.Printf("dep verify: %s/%s not found, status reset to failed", dep.Type, dep.Name)
	}

	if resetCount > 0 {
		log.Printf("dep verify: %d dependencies reset to failed (not found on system)", resetCount)
	}

	var stale []model.Dependency
	database.DB.Where("status IN ?", []string{model.DepStatusInstalling, model.DepStatusRemoving}).Find(&stale)

	toResume := make([]model.Dependency, 0, len(stale))
	for _, dep := range stale {
		if dependencyInstalledFunc(dep.Type, dep.Name) {
			nextLog := appendDependencyLog(dep.Log, "[启动校验] 检测到依赖已安装，已同步状态为已安装")
			database.DB.Model(&dep).Updates(map[string]interface{}{
				"status": model.DepStatusInstalled,
				"log":    nextLog,
			})
			log.Printf("dep verify: %s/%s was %s, reconciled to installed", dep.Type, dep.Name, dep.Status)
			continue
		}

		if shouldResumeRestoredDependency(dep) {
			nextLog := appendDependencyLog(dep.Log, "[启动校验] 检测到恢复任务未完成，已在重启后继续安装")
			database.DB.Model(&dep).Updates(map[string]interface{}{
				"status": model.DepStatusInstalling,
				"log":    nextLog,
			})
			dep.Log = nextLog
			toResume = append(toResume, dep)
			log.Printf("dep verify: %s/%s was %s, resumed restore install after restart", dep.Type, dep.Name, dep.Status)
			continue
		}

		database.DB.Model(&dep).Updates(map[string]interface{}{
			"status": model.DepStatusFailed,
			"log":    appendDependencyLog(dep.Log, "[启动校验] 操作因服务重启而中断"),
		})
		log.Printf("dep verify: %s/%s was %s, reset to failed", dep.Type, dep.Name, dep.Status)
	}

	if len(toResume) > 0 {
		dependencyReinstallBatchFunc(toResume)
		log.Printf("dep verify: resumed %d restored dependencies after restart", len(toResume))
	}
}

func shouldResumeRestoredDependency(dep model.Dependency) bool {
	return dep.Status == model.DepStatusInstalling && strings.Contains(dep.Log, "[恢复备份]")
}

func appendDependencyLog(existing, line string) string {
	existing = strings.TrimRight(existing, "\n")
	line = strings.TrimSpace(line)
	if line == "" {
		return existing
	}
	if existing == "" {
		return line
	}
	if strings.Contains(existing, line) {
		return existing
	}
	return existing + "\n" + line
}
