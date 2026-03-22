package service

import (
	"strings"
	"testing"
	"time"

	"daidai-panel/model"
)

func TestBuildTaskExecutionNotificationIncludesFailureExcerpt(t *testing.T) {
	task := &model.Task{ID: 9, Name: "签到任务"}
	endedAt := time.Date(2026, 3, 22, 12, 34, 56, 789000000, time.Local)

	title, content, context := buildTaskExecutionNotification(
		task,
		42,
		false,
		7,
		3.4,
		endedAt,
		"第一行错误\n第二行错误\n第三行错误",
	)

	if title != "任务执行失败" {
		t.Fatalf("unexpected title: %q", title)
	}
	if !strings.Contains(content, "日志ID: 42") {
		t.Fatalf("expected content to include task log id, got %q", content)
	}
	if !strings.Contains(content, "失败原因:") {
		t.Fatalf("expected content to include failure excerpt, got %q", content)
	}
	if got := context["task_name"]; got != "签到任务" {
		t.Fatalf("expected task_name context, got %q", got)
	}
	if got := context["task_log_id"]; got != "42" {
		t.Fatalf("expected task_log_id context, got %q", got)
	}
	if got := context["error_log"]; got == "" {
		t.Fatal("expected error_log context to be populated")
	}
}

func TestSummarizeTaskFailureOutputKeepsRecentLines(t *testing.T) {
	output := strings.Join([]string{
		"=== 开始执行 [2026-03-22 12:00:00] ===",
		"准备中",
		"请求接口失败",
		"HTTP 500",
		"token expired",
		"=== 执行结束 [2026-03-22 12:00:01] 耗时 1.00 秒 退出码 1 ===",
	}, "\n")

	summary := summarizeTaskFailureOutput(output)
	if strings.Contains(summary, "=== 开始执行") {
		t.Fatalf("expected summary to drop banner lines, got %q", summary)
	}
	if !strings.Contains(summary, "token expired") {
		t.Fatalf("expected summary to keep recent failure details, got %q", summary)
	}
}
