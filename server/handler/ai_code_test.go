package handler_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"daidai-panel/model"
	"daidai-panel/testutil"
)

func TestAICodeConfigListsConfiguredProvidersWithoutSecrets(t *testing.T) {
	testutil.SetupTestEnv(t)

	operator := testutil.MustCreateUser(t, "ai-operator", "operator")
	token := testutil.MustCreateAccessToken(t, operator.Username, operator.Role)

	if err := model.SetConfig("ai_enabled", "true"); err != nil {
		t.Fatalf("enable ai: %v", err)
	}
	if err := model.SetConfig("ai_default_provider", "openai"); err != nil {
		t.Fatalf("set default provider: %v", err)
	}
	if err := model.SetConfig("ai_openai_api_key", "top-secret-key"); err != nil {
		t.Fatalf("set openai key: %v", err)
	}
	if err := model.SetConfig("ai_openai_model", "gpt-test"); err != nil {
		t.Fatalf("set openai model: %v", err)
	}

	engine := newProtectedRouter()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/ai-code/config", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "top-secret-key") {
		t.Fatalf("response body should not expose API key: %s", rec.Body.String())
	}

	payload := decodeJSONMap(t, rec)
	data, ok := payload["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected data map, got %T", payload["data"])
	}
	if enabled, _ := data["enabled"].(bool); !enabled {
		t.Fatalf("expected ai enabled in config response")
	}
	if got, _ := data["default_provider"].(string); got != "openai" {
		t.Fatalf("expected default provider openai, got %q", got)
	}

	providers, ok := data["providers"].([]interface{})
	if !ok || len(providers) == 0 {
		t.Fatalf("expected providers array, got %T", data["providers"])
	}

	var foundOpenAI bool
	for _, item := range providers {
		provider, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if provider["id"] == "openai" {
			foundOpenAI = true
			if configured, _ := provider["configured"].(bool); !configured {
				t.Fatalf("expected openai provider configured")
			}
			if modelName, _ := provider["model"].(string); modelName != "gpt-test" {
				t.Fatalf("expected openai model gpt-test, got %q", modelName)
			}
		}
	}
	if !foundOpenAI {
		t.Fatalf("expected openai provider in config list")
	}
}

func TestAICodeGenerateStreamReturnsDeltaAndDoneEvents(t *testing.T) {
	testutil.SetupTestEnv(t)

	operator := testutil.MustCreateUser(t, "ai-stream-operator", "operator")
	token := testutil.MustCreateAccessToken(t, operator.Username, operator.Role)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"{\\\"summary\\\":\\\"流式修复\\\",\\\"content\\\":\\\"print('\"}}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"ok')\\\\n\\\"}\"}}]}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer upstream.Close()

	for key, value := range map[string]string{
		"ai_enabled":                 "true",
		"ai_default_provider":        "openai",
		"ai_openai_base_url":         upstream.URL + "/v1",
		"ai_openai_api_key":          "handler-secret",
		"ai_openai_model":            "gpt-handler-test",
		"ai_request_timeout_seconds": "30",
	} {
		if err := model.SetConfig(key, value); err != nil {
			t.Fatalf("set config %s: %v", key, err)
		}
	}

	engine := newProtectedRouter()
	body := `{"provider":"openai","mode":"fix","response_mode":"full","prompt":"修复这个 Python 脚本","language":"python","current_content":"print('broken')\n"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ai-code/generate-stream", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", rec.Code, rec.Body.String())
	}
	if !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/event-stream") {
		t.Fatalf("expected text/event-stream content type, got %q", rec.Header().Get("Content-Type"))
	}

	bodyText := rec.Body.String()
	if !strings.Contains(bodyText, "event: delta") {
		t.Fatalf("expected delta event in stream, got %q", bodyText)
	}
	if !strings.Contains(bodyText, "event: done") {
		t.Fatalf("expected done event in stream, got %q", bodyText)
	}
	if !strings.Contains(bodyText, "\"summary\":\"流式修复\"") || !strings.Contains(bodyText, "print('ok')") {
		t.Fatalf("expected final stream payload to contain generated result, got %q", bodyText)
	}
}

func TestAICodeTestConnectionUsesPostedCredentialsAndProtocolSettings(t *testing.T) {
	testutil.SetupTestEnv(t)

	admin := testutil.MustCreateUser(t, "ai-admin", "admin")
	token := testutil.MustCreateAccessToken(t, admin.Username, admin.Role)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/relay/messages" {
			t.Fatalf("expected /relay/messages, got %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer posted-key" {
			t.Fatalf("expected posted api key, got %q", got)
		}
		if got := r.Header.Get("anthropic-version"); got != "" {
			t.Fatalf("expected no anthropic-version header for bearer relay, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"content": []map[string]string{
				{"type": "text", "text": "OK"},
			},
		})
	}))
	defer upstream.Close()

	engine := newProtectedRouter()
	body := `{"provider":"anthropic","base_url":"` + upstream.URL + `/relay/messages","api_key":"posted-key","model":"claude-test-connect","api_format":"anthropic","auth_strategy":"bearer","is_full_url":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ai-code/test", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", rec.Code, rec.Body.String())
	}

	payload := decodeJSONMap(t, rec)
	data, ok := payload["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected data map, got %T", payload["data"])
	}
	if got, _ := data["provider"].(string); got != "anthropic" {
		t.Fatalf("expected provider anthropic, got %q", got)
	}
	if got, _ := data["model"].(string); got != "claude-test-connect" {
		t.Fatalf("expected model claude-test-connect, got %q", got)
	}
	if got, _ := data["message"].(string); !strings.Contains(got, "OK") {
		t.Fatalf("expected test message to contain OK, got %q", got)
	}
}
