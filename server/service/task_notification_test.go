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
	if !strings.Contains(content, "定时任务「签到任务」执行失败") {
		t.Fatalf("expected unified failure summary line, got %q", content)
	}
	if !strings.Contains(content, "完成时间: 2026-03-22 12:34:56.789") {
		t.Fatalf("expected content to include completed time, got %q", content)
	}
	if !strings.Contains(content, "日志ID: 42") {
		t.Fatalf("expected content to include task log id, got %q", content)
	}
	if !strings.Contains(content, "退出码: 7") {
		t.Fatalf("expected content to include exit code, got %q", content)
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
	if got := context["result_summary"]; got != "定时任务「签到任务」执行失败" {
		t.Fatalf("expected result_summary context, got %q", got)
	}
	if got := context["error_log"]; got == "" {
		t.Fatal("expected error_log context to be populated")
	}
	if got := context["failure_reason"]; got == "" {
		t.Fatal("expected failure_reason context to be populated")
	}
}

func TestBuildTaskExecutionNotificationUsesUnifiedSuccessLayout(t *testing.T) {
	task := &model.Task{ID: 10, Name: "电信签到"}
	endedAt := time.Date(2026, 3, 23, 0, 0, 20, 759000000, time.Local)

	title, content, context := buildTaskExecutionNotification(
		task,
		34,
		true,
		0,
		20.7,
		endedAt,
		"",
	)

	if title != "任务执行成功" {
		t.Fatalf("unexpected title: %q", title)
	}
	if !strings.Contains(content, "定时任务「电信签到」执行成功") {
		t.Fatalf("expected unified success summary line, got %q", content)
	}
	if strings.Contains(content, "退出码") {
		t.Fatalf("did not expect exit code in success content, got %q", content)
	}
	if !strings.Contains(content, "完成时间: 2026-03-23 00:00:20.759") {
		t.Fatalf("expected content to include completed time, got %q", content)
	}
	if got := context["status_text"]; got != "成功" {
		t.Fatalf("expected success status_text, got %q", got)
	}
	if got := context["result_summary"]; got != "定时任务「电信签到」执行成功" {
		t.Fatalf("expected success result_summary, got %q", got)
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

func TestSummarizeTaskFailureOutputCondensesPythonTraceback(t *testing.T) {
	output := strings.Join([]string{
		"=== 开始执行 [2026-03-23 00:00:00] ===",
		"Traceback (most recent call last):",
		`  File "/usr/lib/python3.11/asyncio/runners.py", line 190, in run`,
		"    return runner.run(main)",
		"    ^^^^^^^^^^^^^^^^^^^^^^^",
		`  File "/app/Dumb-Panel/scripts/电信营业厅/电信.py", line 1118, in main`,
		"    sign, accId = await getSign(ticket, session)",
		"    ^^^^^^^^^^^",
		"TypeError: cannot unpack non-iterable NoneType object",
	}, "\n")

	summary := summarizeTaskFailureOutput(output)
	if strings.Contains(summary, "asyncio/runners.py") {
		t.Fatalf("expected runtime traceback frames to be removed, got %q", summary)
	}
	if strings.Contains(summary, "^^^^^^^^") {
		t.Fatalf("expected caret indicator lines to be removed, got %q", summary)
	}
	if !strings.Contains(summary, "TypeError: cannot unpack non-iterable NoneType object") {
		t.Fatalf("expected summary to keep final exception, got %q", summary)
	}
	if !strings.Contains(summary, "电信营业厅/电信.py:1118") {
		t.Fatalf("expected summary to keep relevant script frame, got %q", summary)
	}
}
