package service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"daidai-panel/config"
	"daidai-panel/testutil"
)

func TestAppendGitSSHEnvUsesPersistentKnownHosts(t *testing.T) {
	testutil.SetupTestEnv(t)

	sshKeyPath := filepath.Join(t.TempDir(), "deploy key")
	if err := os.WriteFile(sshKeyPath, []byte("dummy"), 0o600); err != nil {
		t.Fatalf("write ssh key: %v", err)
	}

	env, err := appendGitSSHEnv([]string{"BASE=1"}, sshKeyPath)
	if err != nil {
		t.Fatalf("append git ssh env: %v", err)
	}

	var sshCommand string
	for _, entry := range env {
		if strings.HasPrefix(entry, "GIT_SSH_COMMAND=") {
			sshCommand = strings.TrimPrefix(entry, "GIT_SSH_COMMAND=")
			break
		}
	}
	if sshCommand == "" {
		t.Fatalf("expected GIT_SSH_COMMAND to be set, env=%v", env)
	}

	if strings.Contains(sshCommand, "StrictHostKeyChecking=no") {
		t.Fatalf("expected host key checking to stay enabled, got %q", sshCommand)
	}
	if strings.Contains(sshCommand, "/dev/null") {
		t.Fatalf("expected persistent known_hosts instead of /dev/null, got %q", sshCommand)
	}
	if !strings.Contains(sshCommand, "StrictHostKeyChecking=accept-new") {
		t.Fatalf("expected accept-new host key policy, got %q", sshCommand)
	}

	knownHostsPath := filepath.Join(config.C.Data.Dir, "ssh", "known_hosts")
	if _, err := os.Stat(knownHostsPath); err != nil {
		t.Fatalf("expected known_hosts file to exist: %v", err)
	}
	if !strings.Contains(sshCommand, shellEscapeSSHArg(knownHostsPath)) {
		t.Fatalf("expected ssh command to reference known_hosts %q, got %q", knownHostsPath, sshCommand)
	}
}
