package service

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"daidai-panel/config"
	"daidai-panel/database"
	"daidai-panel/model"
	"daidai-panel/pkg/cron"

	"gorm.io/gorm"
)

type PullCallback func(line string)

func PullSubscription(sub *model.Subscription) (string, error) {
	return PullSubscriptionWithCallback(sub, nil)
}

func PullSubscriptionWithCallback(sub *model.Subscription, onOutput PullCallback) (string, error) {
	startTime := time.Now()

	var sshKeyPath string
	if sub.SSHKeyID != nil {
		var sshKey model.SSHKey
		if err := database.DB.First(&sshKey, *sub.SSHKeyID).Error; err == nil {
			tmpFile, err := writeTempSSHKey(sshKey.PrivateKey)
			if err != nil {
				return "", fmt.Errorf("写入 SSH 密钥失败: %w", err)
			}
			defer os.Remove(tmpFile)
			sshKeyPath = tmpFile
		}
	}

	var fullLog strings.Builder
	emit := func(line string) {
		fullLog.WriteString(line)
		fullLog.WriteString("\n")
		if onOutput != nil {
			onOutput(line)
		}
	}

	emit(fmt.Sprintf("[开始拉取] %s (%s)", sub.Name, sub.Type))

	var output string
	var pullErr error

	switch sub.Type {
	case model.SubTypeSingleFile:
		output, pullErr = pullSingleFileWithCallback(sub, sshKeyPath, emit)
	default:
		output, pullErr = pullGitRepoWithCallback(sub, sshKeyPath, emit)
	}

	duration := time.Since(startTime).Seconds()

	status := 0
	if pullErr != nil {
		status = 1
		emit(fmt.Sprintf("[错误] %s", pullErr.Error()))
	}

	emit(fmt.Sprintf("[完成] 耗时 %.2f 秒, 状态: %s", duration, map[int]string{0: "成功", 1: "失败"}[status]))

	if status == 0 {
		syncSubscriptionTasks(sub, emit)
	}

	subLog := model.SubLog{
		SubscriptionID: sub.ID,
		Status:         status,
		Content:        fullLog.String(),
		Duration:       duration,
	}
	database.DB.Create(&subLog)

	now := time.Now()
	database.DB.Model(sub).Updates(map[string]interface{}{
		"last_pull_at": &now,
		"status":       status,
	})

	return output, pullErr
}

func runCmdWithCallback(cmd *exec.Cmd, emit PullCallback) (string, error) {
	pipe, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	cmd.Stderr = cmd.Stdout

	if err := cmd.Start(); err != nil {
		return "", err
	}

	var buf strings.Builder
	scanner := bufio.NewScanner(pipe)
	scanner.Buffer(make([]byte, 64*1024), 256*1024)
	for scanner.Scan() {
		line := scanner.Text()
		buf.WriteString(line)
		buf.WriteString("\n")
		emit(line)
	}

	err = cmd.Wait()
	return buf.String(), err
}

func pullGitRepoWithCallback(sub *model.Subscription, sshKeyPath string, emit PullCallback) (string, error) {
	saveDir := sub.SaveDir
	if saveDir == "" {
		saveDir = sub.Alias
		if saveDir == "" {
			parts := strings.Split(sub.URL, "/")
			saveDir = strings.TrimSuffix(parts[len(parts)-1], ".git")
		}
	}

	destDir := filepath.Join(config.C.Data.ScriptsDir, saveDir)

	env, err := appendGitSSHEnv(os.Environ(), sshKeyPath)
	if err != nil {
		return "", err
	}

	if IsGitRepo(destDir) {
		emit("[git reset --hard]")
		GitReset(destDir)

		emit("[git pull]")
		cmd := exec.Command("git", "pull")
		cmd.Dir = destDir
		cmd.Env = env
		return runCmdWithCallback(cmd, emit)
	}

	emit(fmt.Sprintf("[git clone] %s -> %s", sub.URL, saveDir))
	os.MkdirAll(destDir, 0755)
	args := []string{"clone", "--depth", "1"}
	if sub.Branch != "" {
		args = append(args, "-b", sub.Branch)
	}
	args = append(args, sub.URL, destDir)
	cmd := exec.Command("git", args...)
	cmd.Dir = config.C.Data.ScriptsDir
	cmd.Env = env
	return runCmdWithCallback(cmd, emit)
}

func pullSingleFileWithCallback(sub *model.Subscription, _ string, emit PullCallback) (string, error) {
	saveDir := sub.SaveDir
	if saveDir == "" {
		saveDir = "downloads"
	}

	parts := strings.Split(sub.URL, "/")
	filename := parts[len(parts)-1]
	if sub.Alias != "" {
		filename = sub.Alias
	}

	destPath := filepath.Join(config.C.Data.ScriptsDir, saveDir, filename)
	emit(fmt.Sprintf("[下载] %s -> %s/%s", sub.URL, saveDir, filename))
	output, err := DownloadFile(sub.URL, destPath)
	if output != "" {
		emit(output)
	}
	return output, err
}

func writeTempSSHKey(privateKey string) (string, error) {
	tmpFile, err := os.CreateTemp("", "ssh_key_*")
	if err != nil {
		return "", err
	}
	defer tmpFile.Close()

	if _, err := tmpFile.WriteString(privateKey); err != nil {
		os.Remove(tmpFile.Name())
		return "", err
	}

	os.Chmod(tmpFile.Name(), 0600)
	return tmpFile.Name(), nil
}

var cronCommentRe = regexp.MustCompile(`(?i)#\s*cron\s*[:：]\s*(.+)`)

type subscriptionTaskSyncOptions struct {
	autoAdd     bool
	autoDelete  bool
	defaultCron string
	allowedExts map[string]bool
}

type subscriptionTaskCandidate struct {
	Name           string
	Command        string
	CronExpression string
}

func subscriptionTaskLabel(subID uint) string {
	return fmt.Sprintf("subscription:%d", subID)
}

func hasLabel(labels []string, target string) bool {
	for _, item := range labels {
		if item == target {
			return true
		}
	}
	return false
}

func withLabel(labels []string, target string) []string {
	if hasLabel(labels, target) {
		return labels
	}
	return append(labels, target)
}

func subscriptionSaveDir(sub *model.Subscription) string {
	saveDir := sub.SaveDir
	if saveDir == "" {
		saveDir = sub.Alias
		if saveDir == "" {
			parts := strings.Split(sub.URL, "/")
			saveDir = strings.TrimSuffix(parts[len(parts)-1], ".git")
		}
	}
	return saveDir
}

func matchesSubscriptionFilters(sub *model.Subscription, filename string) bool {
	if sub.Whitelist != "" {
		matched := false
		for _, pattern := range strings.Split(sub.Whitelist, ",") {
			pattern = strings.TrimSpace(pattern)
			if pattern != "" && strings.Contains(filename, pattern) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	if sub.Blacklist != "" {
		for _, pattern := range strings.Split(sub.Blacklist, ",") {
			pattern = strings.TrimSpace(pattern)
			if pattern != "" && strings.Contains(filename, pattern) {
				return false
			}
		}
	}
	return true
}

func syncSubscriptionTasks(sub *model.Subscription, emit PullCallback) {
	options := getSubscriptionTaskSyncOptions(sub)
	if !options.autoAdd && !options.autoDelete {
		return
	}

	candidates := collectSubscriptionTaskCandidates(sub, options)
	label := subscriptionTaskLabel(sub.ID)

	var managedTasks []model.Task
	queryTasksByLabel(label).Find(&managedTasks)
	managedByCommand := make(map[string]*model.Task, len(managedTasks))
	for i := range managedTasks {
		managedByCommand[strings.TrimSpace(managedTasks[i].Command)] = &managedTasks[i]
	}

	created := 0
	updated := 0
	deleted := 0
	adopted := 0

	if options.autoAdd {
		for command, candidate := range candidates {
			if existing, ok := managedByCommand[command]; ok {
				changes := map[string]interface{}{}
				if existing.Name != candidate.Name {
					changes["name"] = candidate.Name
					existing.Name = candidate.Name
				}
				if existing.CronExpression != candidate.CronExpression {
					changes["cron_expression"] = candidate.CronExpression
					existing.CronExpression = candidate.CronExpression
				}
				if len(changes) > 0 {
					database.DB.Model(existing).Updates(changes)
					GetSchedulerV2().UpdateJob(existing)
					updated++
					emit(fmt.Sprintf("[自动更新任务] %s (cron: %s)", candidate.Name, candidate.CronExpression))
				}
				continue
			}

			var existing model.Task
			if err := database.DB.Where("command = ?", command).First(&existing).Error; err == nil {
				labels := withLabel(existing.GetLabels(), label)
				existing.SetLabelsFromSlice(labels)
				database.DB.Model(&existing).Update("labels", existing.Labels)
				managedByCommand[command] = &existing
				adopted++
				emit(fmt.Sprintf("[关联已有任务] %s", existing.Name))
				continue
			}

			task := model.Task{
				Name:            candidate.Name,
				Command:         candidate.Command,
				CronExpression:  candidate.CronExpression,
				TaskType:        model.TaskTypeCron,
				Status:          model.TaskStatusEnabled,
				Timeout:         86400,
				NotifyOnFailure: true,
			}
			task.SetLabelsFromSlice([]string{label})
			if database.DB.Select("*").Create(&task).Error == nil {
				GetSchedulerV2().AddJob(&task)
				managedByCommand[command] = &task
				created++
				emit(fmt.Sprintf("[自动添加任务] %s (cron: %s)", candidate.Name, candidate.CronExpression))
			}
		}
	}

	if options.autoDelete {
		for _, task := range managedTasks {
			command := strings.TrimSpace(task.Command)
			if !strings.HasPrefix(command, "task ") {
				continue
			}
			if _, ok := candidates[command]; ok {
				continue
			}

			GetSchedulerV2().RemoveJob(task.ID)
			database.DB.Where("task_id = ?", task.ID).Delete(&model.TaskLog{})
			database.DB.Delete(&task)
			deleted++
			emit(fmt.Sprintf("[自动删除任务] %s", task.Name))
		}
	}

	if created > 0 {
		emit(fmt.Sprintf("[共自动添加 %d 个定时任务]", created))
	}
	if updated > 0 {
		emit(fmt.Sprintf("[共自动更新 %d 个定时任务]", updated))
	}
	if adopted > 0 {
		emit(fmt.Sprintf("[共关联 %d 个已有任务]", adopted))
	}
	if deleted > 0 {
		emit(fmt.Sprintf("[共自动删除 %d 个失效任务]", deleted))
	}
}

func getSubscriptionTaskSyncOptions(sub *model.Subscription) subscriptionTaskSyncOptions {
	defaultCron := strings.TrimSpace(model.GetRegisteredConfig("default_cron_rule"))
	if defaultCron != "" && !cron.Parse(defaultCron).Valid {
		defaultCron = ""
	}

	return subscriptionTaskSyncOptions{
		autoAdd:     sub.AutoAddTask || isConfigEnabled("auto_add_cron", true),
		autoDelete:  sub.AutoDelTask || isConfigEnabled("auto_del_cron", true),
		defaultCron: defaultCron,
		allowedExts: getSubscriptionAllowedExtensions(model.GetRegisteredConfig("repo_file_extensions")),
	}
}

func isConfigEnabled(key string, defaultValue bool) bool {
	if _, exists := model.GetSystemConfigDefinition(key); exists {
		return model.GetRegisteredConfigBool(key)
	}
	return model.GetConfigBool(key, defaultValue)
}

func getSubscriptionAllowedExtensions(raw string) map[string]bool {
	exts := make(map[string]bool)
	for _, token := range strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n' || r == '\r' || r == '\t' || r == ' '
	}) {
		token = strings.TrimSpace(strings.ToLower(token))
		token = strings.TrimPrefix(token, "*")
		if token == "" {
			continue
		}
		if !strings.HasPrefix(token, ".") {
			token = "." + token
		}
		exts[token] = true
	}
	if len(exts) > 0 {
		return exts
	}

	return map[string]bool{
		".js": true,
		".ts": true,
		".py": true,
		".sh": true,
	}
}

func shouldManageSubscriptionFile(sub *model.Subscription, filename string, allowedExts map[string]bool) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	if !allowedExts[ext] {
		return false
	}
	return matchesSubscriptionFilters(sub, filename)
}

func collectSubscriptionTaskCandidates(sub *model.Subscription, options subscriptionTaskSyncOptions) map[string]subscriptionTaskCandidate {
	candidates := make(map[string]subscriptionTaskCandidate)
	saveDir := subscriptionSaveDir(sub)
	scriptsDir := filepath.Join(config.C.Data.ScriptsDir, saveDir)

	if _, err := os.Stat(scriptsDir); err != nil {
		return candidates
	}

	filepath.Walk(scriptsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			switch strings.ToLower(info.Name()) {
			case ".git", "node_modules", "__pycache__":
				return filepath.SkipDir
			}
			return nil
		}

		if !shouldManageSubscriptionFile(sub, info.Name(), options.allowedExts) {
			return nil
		}

		cronExpr := resolveCronForSubscriptionTask(path, options.defaultCron)
		if cronExpr == "" {
			return nil
		}

		relPath, err := filepath.Rel(config.C.Data.ScriptsDir, path)
		if err != nil {
			return nil
		}
		command := "task " + relPath
		taskName := strings.TrimSuffix(info.Name(), filepath.Ext(info.Name()))
		candidates[command] = subscriptionTaskCandidate{
			Name:           taskName,
			Command:        command,
			CronExpression: cronExpr,
		}
		return nil
	})

	return candidates
}

func queryTasksByLabel(label string) *gorm.DB {
	return database.DB.Where(
		"labels = ? OR labels LIKE ? OR labels LIKE ? OR labels LIKE ?",
		label,
		label+",%",
		"%,"+label,
		"%,"+label+",%",
	)
}

func resolveCronForSubscriptionTask(path string, defaultCron string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	lineCount := 0
	for scanner.Scan() {
		lineCount++
		if lineCount > 50 {
			break
		}
		line := scanner.Text()
		matches := cronCommentRe.FindStringSubmatch(line)
		if len(matches) > 1 {
			expr := strings.TrimSpace(matches[1])
			result := cron.Parse(expr)
			if result.Valid {
				return expr
			}
		}
	}
	return strings.TrimSpace(defaultCron)
}
