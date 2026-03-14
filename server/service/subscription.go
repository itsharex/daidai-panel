package service

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"daidai-panel/config"
	"daidai-panel/database"
	"daidai-panel/model"
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

	emit := func(line string) {
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
	content := output
	if pullErr != nil {
		status = 1
		content = fmt.Sprintf("%s\nError: %s", output, pullErr.Error())
		emit(fmt.Sprintf("[错误] %s", pullErr.Error()))
	}

	emit(fmt.Sprintf("[完成] 耗时 %.2f 秒, 状态: %s", duration, map[int]string{0: "成功", 1: "失败"}[status]))

	subLog := model.SubLog{
		SubscriptionID: sub.ID,
		Status:         status,
		Content:        content,
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

	env := os.Environ()
	if sshKeyPath != "" {
		sshCmd := fmt.Sprintf("ssh -i %s -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null", sshKeyPath)
		env = append(env, "GIT_SSH_COMMAND="+sshCmd)
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
