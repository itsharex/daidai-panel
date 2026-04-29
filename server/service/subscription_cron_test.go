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

func TestResolveSubscriptionTaskNamePrefersNewEnvTitle(t *testing.T) {
	root := t.TempDir()
	scriptPath := filepath.Join(root, "main.py")
	content := "\"\"\"\nnew Env('华星电信999答题');\ncron: 1 1 1 1 1\n\"\"\"\nprint('hello')\n"
	if err := os.WriteFile(scriptPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write script: %v", err)
	}

	got := resolveSubscriptionTaskName(scriptPath, "main")
	if got != "华星电信999答题" {
		t.Fatalf("expected task name from new Env title, got %q", got)
	}
}

func TestResolveSubscriptionTaskNameFallsBackToFilenameWhenNoNewEnvTitle(t *testing.T) {
	root := t.TempDir()
	scriptPath := filepath.Join(root, "main.py")
	content := "print('hello')\n"
	if err := os.WriteFile(scriptPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write script: %v", err)
	}

	got := resolveSubscriptionTaskName(scriptPath, "main")
	if got != "main" {
		t.Fatalf("expected fallback task name, got %q", got)
	}
}

// 覆盖 JS 块注释 `/* ... */` 中 `<cron> <filename>` 形式（jd_OnceApply.js 风格）。
func TestResolveCronForSubscriptionTaskSupportsBlockCommentCronFilenameHeader(t *testing.T) {
	root := t.TempDir()
	scriptPath := filepath.Join(root, "jd_OnceApply.js")
	content := "/*\n价格保护\n55 11 * * * jd_OnceApply.js\n */\nconst $ = new Env('一键价保');\n"
	if err := os.WriteFile(scriptPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write script: %v", err)
	}

	got := resolveCronForSubscriptionTask(scriptPath, "")
	if got != "55 11 * * *" {
		t.Fatalf("expected cron from block comment header, got %q", got)
	}
}

// 覆盖 Python docstring 中 `<cron> <filename>` 形式（jd_beans_7days.py 风格）。
func TestResolveCronForSubscriptionTaskSupportsPythonDocstringCronFilenameHeader(t *testing.T) {
	root := t.TempDir()
	scriptPath := filepath.Join(root, "jd_beans_7days.py")
	content := "# !/usr/bin/env python3\n# -*- coding: utf-8 -*-\n'''\nnew Env('豆子7天统计');\n8 8 29 2 * jd_beans_7days.py\n'''\nprint('hello')\n"
	if err := os.WriteFile(scriptPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write script: %v", err)
	}

	got := resolveCronForSubscriptionTask(scriptPath, "")
	if got != "8 8 29 2 *" {
		t.Fatalf("expected cron from python docstring header, got %q", got)
	}
}

// 覆盖青龙单行声明 `cron "EXPR" filename, tag:xxx`（jd_CheckCK.js 风格）。
func TestResolveCronForSubscriptionTaskSupportsCronDirectiveLine(t *testing.T) {
	root := t.TempDir()
	scriptPath := filepath.Join(root, "jd_CheckCK.js")
	content := "/*\ncron \"6 6 6 6 *\" jd_CheckCK.js, tag:京东CK检测by-ccwav\n */\nconsole.log('hi');\n"
	if err := os.WriteFile(scriptPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write script: %v", err)
	}

	got := resolveCronForSubscriptionTask(scriptPath, "")
	if got != "6 6 6 6 *" {
		t.Fatalf("expected cron from cron directive line, got %q", got)
	}
}

// 青龙单行声明的 cron 与脚本文件名不一致时应忽略，避免误抓邻接脚本的声明。
func TestResolveCronForSubscriptionTaskCronDirectiveIgnoresOtherFile(t *testing.T) {
	root := t.TempDir()
	scriptPath := filepath.Join(root, "jd_OnceApply.js")
	content := "/*\ncron \"6 6 6 6 *\" jd_CheckCK.js, tag:京东CK检测\n */\n"
	if err := os.WriteFile(scriptPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write script: %v", err)
	}

	got := resolveCronForSubscriptionTask(scriptPath, "0 0 * * *")
	if got != "0 0 * * *" {
		t.Fatalf("expected fallback cron when directive points to other file, got %q", got)
	}
}

// 真实场景：B 站 cookie 脚本，docstring 中含 cron 行 + 多行中文说明 + 含 = 的代码片段，
// 不应被中文说明 / 含 = 的代码行误识别为 cron。
func TestResolveCronForSubscriptionTaskBilibiliDocstringScenario(t *testing.T) {
	root := t.TempDir()
	scriptPath := filepath.Join(root, "bili_task_get_cookie.py")
	content := `'''
1 9 11 11 1 bili_task_get_cookie.py
手动运行，查看日志，并使用手机B站app扫描日志中二维码，注意，只能修改第一个cookie
如果产生错误，重新运行并用手机扫描二维码
有可能识别不出来二维码，我测试了几次都能识别

默认环境变量存放位置为/ql/data/config/env.sh
可以自己通过docker命令进入容器查找这个文件位置。docker exec -it qinglong /bin/bash,进入青龙容器，然后查找一下这个文件位置
filename = '../config/env.sh'
'''
print('hello')
`
	if err := os.WriteFile(scriptPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write script: %v", err)
	}

	got := resolveCronForSubscriptionTask(scriptPath, "")
	if got != "1 9 11 11 1" {
		t.Fatalf("expected cron from docstring header, got %q", got)
	}
}
