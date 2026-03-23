package service

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"daidai-panel/config"
)

func appendGitSSHEnv(baseEnv []string, sshKeyPath string) ([]string, error) {
	env := AppendProxyEnv(baseEnv)
	sshKeyPath = strings.TrimSpace(sshKeyPath)
	if sshKeyPath == "" {
		return env, nil
	}

	knownHostsPath, err := ensureGitKnownHostsFile()
	if err != nil {
		return nil, err
	}

	env = append(env, "GIT_SSH_COMMAND="+buildGitSSHCommand(sshKeyPath, knownHostsPath))
	return env, nil
}

func ensureGitKnownHostsFile() (string, error) {
	if config.C == nil {
		return "", fmt.Errorf("配置未初始化")
	}

	sshDir := filepath.Join(config.C.Data.Dir, "ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		return "", fmt.Errorf("创建 SSH 配置目录失败: %w", err)
	}

	knownHostsPath := filepath.Join(sshDir, "known_hosts")
	if _, err := os.Stat(knownHostsPath); os.IsNotExist(err) {
		if err := os.WriteFile(knownHostsPath, []byte{}, 0o600); err != nil {
			return "", fmt.Errorf("创建 known_hosts 失败: %w", err)
		}
	} else if err != nil {
		return "", fmt.Errorf("读取 known_hosts 失败: %w", err)
	}

	return knownHostsPath, nil
}

func buildGitSSHCommand(sshKeyPath, knownHostsPath string) string {
	return fmt.Sprintf(
		"ssh -i %s -o IdentitiesOnly=yes -o StrictHostKeyChecking=accept-new -o UserKnownHostsFile=%s",
		shellEscapeSSHArg(sshKeyPath),
		shellEscapeSSHArg(knownHostsPath),
	)
}

func shellEscapeSSHArg(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}
