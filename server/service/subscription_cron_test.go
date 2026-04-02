package service

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveCronForSubscriptionTaskSupportsDocstringCronFilenameHeader(t *testing.T) {
	root := t.TempDir()
	scriptPath := filepath.Join(root, "bili_task_get_cookie.py")
	content := "'''\n1 9 11 11 1 bili_task_get_cookie.py\n手动运行，查看日志\n'''\nprint('hello')\n"
	if err := os.WriteFile(scriptPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write script: %v", err)
	}

	got := resolveCronForSubscriptionTask(scriptPath, "")
	if got != "1 9 11 11 1" {
		t.Fatalf("expected cron from docstring header, got %q", got)
	}
}

func TestResolveCronForSubscriptionTaskIgnoresDocstringCronForOtherFile(t *testing.T) {
	root := t.TempDir()
	scriptPath := filepath.Join(root, "actual_task.py")
	content := "'''\n1 9 11 11 1 other_task.py\n手动运行，查看日志\n'''\nprint('hello')\n"
	if err := os.WriteFile(scriptPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write script: %v", err)
	}

	got := resolveCronForSubscriptionTask(scriptPath, "0 0 * * *")
	if got != "0 0 * * *" {
		t.Fatalf("expected fallback cron for mismatched filename, got %q", got)
	}
}
