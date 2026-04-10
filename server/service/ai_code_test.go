package service_test

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"daidai-panel/model"
	"daidai-panel/service"
	"daidai-panel/testutil"
)

func boolPtr(value bool) *bool {
	return &value
}

func mustSetConfig(t *testing.T, key, value string) {
	t.Helper()
	if err := model.SetConfig(key, value); err != nil {
		t.Fatalf("set config %s: %v", key, err)
	}
}

func enableAI(t *testing.T) {
	t.Helper()
	mustSetConfig(t, "ai_enabled", "true")
	mustSetConfig(t, "ai_request_timeout_seconds", "30")
}

func TestGenerateAICodeOpenAICompatible(t *testing.T) {
	testutil.SetupTestEnv(t)
	enableAI(t)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("expected /v1/chat/completions, got %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer openai-secret" {
			t.Fatalf("expected Authorization header, got %q", got)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]interface{}{
						"content": `{"summary":"已修复脚本","content":"print('fixed')\n","warnings":["建议再次运行调试"]}`,
					},
				},
			},
		})
	}))
	defer upstream.Close()

	mustSetConfig(t, "ai_default_provider", "openai")
	mustSetConfig(t, "ai_openai_base_url", upstream.URL+"/v1")
	mustSetConfig(t, "ai_openai_api_key", "openai-secret")
	mustSetConfig(t, "ai_openai_model", "gpt-test")

	result, err := service.GenerateAICodeStream(context.Background(), service.AICodeRequest{
		Provider:       "openai",
		Mode:           "fix",
		Prompt:         "修复这个脚本的异常",
		Language:       "python",
		CurrentContent: "print('broken')\n",
	}, nil)
	if err != nil {
		t.Fatalf("generate ai code: %v", err)
	}
	if result.Provider != "openai" {
		t.Fatalf("expected provider openai, got %q", result.Provider)
	}
	if result.Model != "gpt-test" {
		t.Fatalf("expected model gpt-test, got %q", result.Model)
	}
	if !strings.Contains(result.Content, "fixed") {
		t.Fatalf("expected fixed content, got %q", result.Content)
	}
	if len(result.Warnings) != 1 || result.Warnings[0] != "建议再次运行调试" {
		t.Fatalf("expected warnings to round-trip, got %#v", result.Warnings)
	}
}

func TestGenerateAICodeAnthropic(t *testing.T) {
	testutil.SetupTestEnv(t)
	enableAI(t)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" {
			t.Fatalf("expected /v1/messages, got %s", r.URL.Path)
		}
		if got := r.Header.Get("x-api-key"); got != "anthropic-secret" {
			t.Fatalf("expected x-api-key header, got %q", got)
		}
		if got := r.Header.Get("anthropic-version"); got != "2023-06-01" {
			t.Fatalf("expected anthropic-version header, got %q", got)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"content": []map[string]string{
				{"type": "text", "text": `{"summary":"已完成 Claude 修改","content":"console.log('claude');\n"}`},
			},
		})
	}))
	defer upstream.Close()

	mustSetConfig(t, "ai_default_provider", "anthropic")
	mustSetConfig(t, "ai_anthropic_base_url", upstream.URL)
	mustSetConfig(t, "ai_anthropic_api_key", "anthropic-secret")
	mustSetConfig(t, "ai_anthropic_model", "claude-test")

	result, err := service.GenerateAICodeStream(context.Background(), service.AICodeRequest{
		Provider:       "anthropic",
		Mode:           "modify",
		Prompt:         "把输出改成 claude",
		Language:       "javascript",
		CurrentContent: "console.log('demo');\n",
	}, nil)
	if err != nil {
		t.Fatalf("generate ai code: %v", err)
	}
	if result.Provider != "anthropic" {
		t.Fatalf("expected provider anthropic, got %q", result.Provider)
	}
	if !strings.Contains(result.Content, "claude") {
		t.Fatalf("expected claude content, got %q", result.Content)
	}
}

func TestGenerateAICodeAnthropicUsesStreamFirst(t *testing.T) {
	testutil.SetupTestEnv(t)
	enableAI(t)

	var requestCount int
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if r.URL.Path != "/v1/messages" {
			t.Fatalf("expected /v1/messages, got %s", r.URL.Path)
		}
		if got := r.Header.Get("x-api-key"); got != "anthropic-stream-secret" {
			t.Fatalf("expected x-api-key header, got %q", got)
		}

		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if stream, _ := payload["stream"].(bool); !stream {
			t.Fatalf("expected anthropic request to use stream first, got payload %#v", payload)
		}

		w.Header().Set("Content-Type", "text/event-stream")
		if flusher, ok := w.(http.Flusher); ok {
			_, _ = io.WriteString(w, "event: content_block_delta\n")
			_, _ = io.WriteString(w, "data: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"{\\\"summary\\\":\\\"已通过 Claude 流式返回\\\",\\\"content\\\":\\\"console.log('claude-stream');\\\\n\\\"}\"}}\n\n")
			flusher.Flush()
			_, _ = io.WriteString(w, "event: message_stop\n")
			_, _ = io.WriteString(w, "data: {\"type\":\"message_stop\"}\n\n")
			flusher.Flush()
			return
		}

		t.Fatal("expected response writer to support flushing")
	}))
	defer upstream.Close()

	mustSetConfig(t, "ai_default_provider", "anthropic")
	mustSetConfig(t, "ai_anthropic_base_url", upstream.URL)
	mustSetConfig(t, "ai_anthropic_api_key", "anthropic-stream-secret")
	mustSetConfig(t, "ai_anthropic_model", "claude-stream-test")

	result, err := service.GenerateAICodeStream(context.Background(), service.AICodeRequest{
		Provider:       "anthropic",
		Mode:           "modify",
		Prompt:         "改成 Claude 流式输出",
		Language:       "javascript",
		CurrentContent: "console.log('demo');\n",
	}, nil)
	if err != nil {
		t.Fatalf("generate ai code with anthropic streaming first: %v", err)
	}
	if requestCount != 1 {
		t.Fatalf("expected exactly one upstream request, got %d", requestCount)
	}
	if !strings.Contains(result.Content, "claude-stream") {
		t.Fatalf("expected anthropic streaming content, got %q", result.Content)
	}
}

func TestGenerateAICodeAnthropicRelayOpenAIChat(t *testing.T) {
	testutil.SetupTestEnv(t)
	enableAI(t)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/relay/chat/completions" {
			t.Fatalf("expected /relay/chat/completions, got %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer anthropic-relay-secret" {
			t.Fatalf("expected bearer authorization header, got %q", got)
		}
		if got := r.Header.Get("x-api-key"); got != "" {
			t.Fatalf("expected no x-api-key header, got %q", got)
		}
		if got := r.Header.Get("anthropic-version"); got != "" {
			t.Fatalf("expected no anthropic-version header for bearer relay, got %q", got)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]interface{}{
						"content": `{"summary":"Claude relay 返回成功","content":"console.log('relay');\n"}`,
					},
				},
			},
		})
	}))
	defer upstream.Close()

	mustSetConfig(t, "ai_default_provider", "anthropic")
	mustSetConfig(t, "ai_anthropic_base_url", upstream.URL+"/relay/chat/completions")
	mustSetConfig(t, "ai_anthropic_api_key", "anthropic-relay-secret")
	mustSetConfig(t, "ai_anthropic_model", "claude-relay-test")
	mustSetConfig(t, "ai_anthropic_api_format", "openai_chat")
	mustSetConfig(t, "ai_anthropic_auth_strategy", "bearer")
	mustSetConfig(t, "ai_anthropic_is_full_url", "true")

	result, err := service.GenerateAICodeStream(context.Background(), service.AICodeRequest{
		Provider:       "anthropic",
		Mode:           "modify",
		Prompt:         "改成 relay 输出",
		Language:       "javascript",
		CurrentContent: "console.log('demo');\n",
	}, nil)
	if err != nil {
		t.Fatalf("generate ai code via claude relay: %v", err)
	}
	if result.Provider != "anthropic" {
		t.Fatalf("expected provider anthropic, got %q", result.Provider)
	}
	if !strings.Contains(result.Content, "relay") {
		t.Fatalf("expected relay content, got %q", result.Content)
	}
}

func TestGenerateAICodeGemini(t *testing.T) {
	testutil.SetupTestEnv(t)
	enableAI(t)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/v1beta/models/gemini-test:generateContent") &&
			!strings.HasSuffix(r.URL.Path, "/v1beta/models/gemini-test:streamGenerateContent") {
			t.Fatalf("unexpected gemini path: %s", r.URL.Path)
		}
		if got := r.Header.Get("x-goog-api-key"); got != "gemini-secret" {
			t.Fatalf("expected x-goog-api-key header, got %q", got)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"candidates": []map[string]interface{}{
				{
					"content": map[string]interface{}{
						"parts": []map[string]string{
							{"text": `{"summary":"已生成 Gemini 脚本","content":"echo gemini\n"}`},
						},
					},
				},
			},
		})
	}))
	defer upstream.Close()

	mustSetConfig(t, "ai_default_provider", "gemini")
	mustSetConfig(t, "ai_gemini_base_url", upstream.URL)
	mustSetConfig(t, "ai_gemini_api_key", "gemini-secret")
	mustSetConfig(t, "ai_gemini_model", "gemini-test")

	result, err := service.GenerateAICodeStream(context.Background(), service.AICodeRequest{
		Provider:   "gemini",
		Mode:       "generate",
		Prompt:     "生成一个输出 gemini 的 shell 脚本",
		Language:   "shell",
		TargetPath: "demo.sh",
	}, nil)
	if err != nil {
		t.Fatalf("generate ai code: %v", err)
	}
	if result.Provider != "gemini" {
		t.Fatalf("expected provider gemini, got %q", result.Provider)
	}
	if !strings.Contains(result.Content, "gemini") {
		t.Fatalf("expected gemini content, got %q", result.Content)
	}
}

func TestGenerateAICodeGeminiUsesStreamFirst(t *testing.T) {
	testutil.SetupTestEnv(t)
	enableAI(t)

	var requestCount int
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if !strings.HasSuffix(r.URL.Path, "/v1beta/models/gemini-stream-test:streamGenerateContent") {
			t.Fatalf("unexpected gemini stream path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("alt"); got != "sse" {
			t.Fatalf("expected alt=sse, got %q", got)
		}
		if got := r.Header.Get("x-goog-api-key"); got != "gemini-stream-secret" {
			t.Fatalf("expected x-goog-api-key header, got %q", got)
		}

		w.Header().Set("Content-Type", "text/event-stream")
		if flusher, ok := w.(http.Flusher); ok {
			_, _ = io.WriteString(w, "data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"{\\\"summary\\\":\\\"已通过 Gemini 流式返回\\\",\\\"content\\\":\\\"echo gemini-stream\\\\n\\\"}\"}]}}]}\n\n")
			flusher.Flush()
			return
		}

		t.Fatal("expected response writer to support flushing")
	}))
	defer upstream.Close()

	mustSetConfig(t, "ai_default_provider", "gemini")
	mustSetConfig(t, "ai_gemini_base_url", upstream.URL)
	mustSetConfig(t, "ai_gemini_api_key", "gemini-stream-secret")
	mustSetConfig(t, "ai_gemini_model", "gemini-stream-test")

	result, err := service.GenerateAICodeStream(context.Background(), service.AICodeRequest{
		Provider:   "gemini",
		Mode:       "generate",
		Prompt:     "生成一个输出 gemini-stream 的 shell 脚本",
		Language:   "shell",
		TargetPath: "demo.sh",
	}, nil)
	if err != nil {
		t.Fatalf("generate ai code with gemini streaming first: %v", err)
	}
	if requestCount != 1 {
		t.Fatalf("expected exactly one upstream request, got %d", requestCount)
	}
	if !strings.Contains(result.Content, "gemini-stream") {
		t.Fatalf("expected gemini streaming content, got %q", result.Content)
	}
}

func TestGenerateAICodeCustomProvider(t *testing.T) {
	testutil.SetupTestEnv(t)
	enableAI(t)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("expected /chat/completions, got %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer custom-secret" {
			t.Fatalf("expected Authorization header, got %q", got)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]interface{}{
						"content": `{"summary":"第三方接口返回成功","content":"print('custom')\n"}`,
					},
				},
			},
		})
	}))
	defer upstream.Close()

	mustSetConfig(t, "ai_default_provider", "custom")
	mustSetConfig(t, "ai_custom_base_url", upstream.URL)
	mustSetConfig(t, "ai_custom_api_key", "custom-secret")
	mustSetConfig(t, "ai_custom_model", "custom-model")

	result, err := service.GenerateAICodeStream(context.Background(), service.AICodeRequest{
		Provider: "custom",
		Mode:     "generate",
		Prompt:   "生成 custom 测试脚本",
		Language: "python",
	}, nil)
	if err != nil {
		t.Fatalf("generate ai code: %v", err)
	}
	if result.Provider != "custom" {
		t.Fatalf("expected provider custom, got %q", result.Provider)
	}
	if !strings.Contains(result.Content, "custom") {
		t.Fatalf("expected custom content, got %q", result.Content)
	}
}

func TestGenerateAICodeCustomProviderSupportsNestedChatContent(t *testing.T) {
	testutil.SetupTestEnv(t)
	enableAI(t)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("expected /chat/completions, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]interface{}{
						"content": []map[string]interface{}{
							{
								"type": "text",
								"text": map[string]interface{}{
									"value": `{"summary":"第三方嵌套文本返回成功","content":"print('nested')\n"}`,
								},
							},
						},
					},
				},
			},
		})
	}))
	defer upstream.Close()

	mustSetConfig(t, "ai_default_provider", "custom")
	mustSetConfig(t, "ai_custom_base_url", upstream.URL)
	mustSetConfig(t, "ai_custom_api_key", "custom-secret")
	mustSetConfig(t, "ai_custom_model", "custom-model")
	mustSetConfig(t, "ai_custom_api_format", "openai_chat")

	result, err := service.GenerateAICodeStream(context.Background(), service.AICodeRequest{
		Provider: "custom",
		Mode:     "generate",
		Prompt:   "生成 nested 测试脚本",
		Language: "python",
	}, nil)
	if err != nil {
		t.Fatalf("generate ai code with nested chat content: %v", err)
	}
	if !strings.Contains(result.Content, "nested") {
		t.Fatalf("expected nested content, got %q", result.Content)
	}
}

func TestGenerateAICodeCustomProviderSupportsWrappedChatPayload(t *testing.T) {
	testutil.SetupTestEnv(t)
	enableAI(t)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("expected /chat/completions, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]interface{}{
				"choices": []map[string]interface{}{
					{
						"message": map[string]interface{}{
							"content": `{"summary":"第三方包装层返回成功","content":"print('wrapped')\n"}`,
						},
					},
				},
			},
		})
	}))
	defer upstream.Close()

	mustSetConfig(t, "ai_default_provider", "custom")
	mustSetConfig(t, "ai_custom_base_url", upstream.URL)
	mustSetConfig(t, "ai_custom_api_key", "custom-secret")
	mustSetConfig(t, "ai_custom_model", "custom-model")
	mustSetConfig(t, "ai_custom_api_format", "openai_chat")

	result, err := service.GenerateAICodeStream(context.Background(), service.AICodeRequest{
		Provider: "custom",
		Mode:     "generate",
		Prompt:   "生成 wrapped 测试脚本",
		Language: "python",
	}, nil)
	if err != nil {
		t.Fatalf("generate ai code with wrapped chat payload: %v", err)
	}
	if !strings.Contains(result.Content, "wrapped") {
		t.Fatalf("expected wrapped content, got %q", result.Content)
	}
}

func TestGenerateAICodeCustomProviderPrefersMessageContentOverReasoningContent(t *testing.T) {
	testutil.SetupTestEnv(t)
	enableAI(t)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("expected /chat/completions, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]interface{}{
						"content":           `{"summary":"第三方 content 优先返回成功","content":"print('content-first')\n"}`,
						"reasoning_content": "这里是模型推理内容，不应替代最终 answer",
					},
				},
			},
		})
	}))
	defer upstream.Close()

	mustSetConfig(t, "ai_default_provider", "custom")
	mustSetConfig(t, "ai_custom_base_url", upstream.URL)
	mustSetConfig(t, "ai_custom_api_key", "custom-secret")
	mustSetConfig(t, "ai_custom_model", "custom-model")
	mustSetConfig(t, "ai_custom_api_format", "openai_chat")

	result, err := service.GenerateAICodeStream(context.Background(), service.AICodeRequest{
		Provider: "custom",
		Mode:     "generate",
		Prompt:   "生成 content 优先测试脚本",
		Language: "python",
	}, nil)
	if err != nil {
		t.Fatalf("generate ai code with content + reasoning_content: %v", err)
	}
	if !strings.Contains(result.Content, "content-first") {
		t.Fatalf("expected final content to come from message.content, got %q", result.Content)
	}
}

func TestGenerateAICodeCustomProviderFallsBackToStreamChatContent(t *testing.T) {
	testutil.SetupTestEnv(t)
	enableAI(t)

	requestCount := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("expected /chat/completions, got %s", r.URL.Path)
		}

		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}

		if stream, _ := payload["stream"].(bool); stream {
			w.Header().Set("Content-Type", "text/event-stream")
			writer := bufio.NewWriter(w)
			_, _ = writer.WriteString("data: {\"choices\":[{\"delta\":{\"content\":\"{\\\"summary\\\":\\\"第三方流式回退成功\\\",\\\"content\\\":\\\"print('stream-fallback')\\\\n\\\"}\"}}]}\n\n")
			_, _ = writer.WriteString("data: [DONE]\n\n")
			_ = writer.Flush()
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"id":      "chatcmpl-empty",
			"object":  "chat.completion",
			"created": 123,
			"model":   "custom-model",
			"choices": []map[string]interface{}{
				{
					"index":         0,
					"finish_reason": "stop",
					"message": map[string]interface{}{
						"content": nil,
					},
				},
			},
			"usage": map[string]interface{}{
				"prompt_tokens":     10,
				"completion_tokens": 0,
				"total_tokens":      10,
			},
		})
	}))
	defer upstream.Close()

	mustSetConfig(t, "ai_default_provider", "custom")
	mustSetConfig(t, "ai_custom_base_url", upstream.URL)
	mustSetConfig(t, "ai_custom_api_key", "custom-secret")
	mustSetConfig(t, "ai_custom_model", "custom-model")
	mustSetConfig(t, "ai_custom_api_format", "openai_chat")

	result, err := service.GenerateAICodeStream(context.Background(), service.AICodeRequest{
		Provider: "custom",
		Mode:     "generate",
		Prompt:   "生成 stream fallback 测试脚本",
		Language: "python",
	}, nil)
	if err != nil {
		t.Fatalf("generate ai code with stream fallback: %v", err)
	}
	if requestCount != 1 {
		t.Fatalf("expected stream-first to succeed in 1 request, got %d requests", requestCount)
	}
	if !strings.Contains(result.Content, "stream-fallback") {
		t.Fatalf("expected stream fallback content, got %q", result.Content)
	}
}

func TestGenerateAICodeOpenAIResponsesFullEndpoint(t *testing.T) {
	testutil.SetupTestEnv(t)
	enableAI(t)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/relay/responses" {
			t.Fatalf("expected /relay/responses, got %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer responses-secret" {
			t.Fatalf("expected Authorization header, got %q", got)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"output_text": `{"summary":"Responses 协议返回成功","content":"print('responses')\n"}`,
		})
	}))
	defer upstream.Close()

	mustSetConfig(t, "ai_default_provider", "openai")
	mustSetConfig(t, "ai_openai_base_url", upstream.URL+"/relay/responses")
	mustSetConfig(t, "ai_openai_api_key", "responses-secret")
	mustSetConfig(t, "ai_openai_model", "gpt-responses-test")
	mustSetConfig(t, "ai_openai_api_format", "openai_responses")
	mustSetConfig(t, "ai_openai_is_full_url", "true")

	result, err := service.GenerateAICodeStream(context.Background(), service.AICodeRequest{
		Provider: "openai",
		Mode:     "generate",
		Prompt:   "生成 responses 测试脚本",
		Language: "python",
	}, nil)
	if err != nil {
		t.Fatalf("generate ai code via responses api: %v", err)
	}
	if result.Provider != "openai" {
		t.Fatalf("expected provider openai, got %q", result.Provider)
	}
	if !strings.Contains(result.Content, "responses") {
		t.Fatalf("expected responses content, got %q", result.Content)
	}
}

func TestGenerateAICodeOpenAIResponsesUsesStreamFirst(t *testing.T) {
	testutil.SetupTestEnv(t)
	enableAI(t)

	var requestCount int
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if r.URL.Path != "/v1/responses" {
			t.Fatalf("expected /v1/responses, got %s", r.URL.Path)
		}

		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if stream, _ := payload["stream"].(bool); !stream {
			t.Fatalf("expected responses request to use stream first, got payload %#v", payload)
		}

		w.Header().Set("Content-Type", "text/event-stream")
		if flusher, ok := w.(http.Flusher); ok {
			_, _ = io.WriteString(w, "data: {\"type\":\"response.output_text.delta\",\"delta\":\"{\\\"summary\\\":\\\"Responses 流式返回成功\\\",\\\"content\\\":\\\"print('responses-stream')\\\\n\\\"}\"}\n\n")
			flusher.Flush()
			_, _ = io.WriteString(w, "data: {\"type\":\"response.completed\"}\n\n")
			flusher.Flush()
			return
		}

		t.Fatal("expected response writer to support flushing")
	}))
	defer upstream.Close()

	mustSetConfig(t, "ai_default_provider", "openai")
	mustSetConfig(t, "ai_openai_base_url", upstream.URL+"/v1")
	mustSetConfig(t, "ai_openai_api_key", "responses-stream-secret")
	mustSetConfig(t, "ai_openai_model", "gpt-responses-stream")
	mustSetConfig(t, "ai_openai_api_format", "openai_responses")

	result, err := service.GenerateAICodeStream(context.Background(), service.AICodeRequest{
		Provider: "openai",
		Mode:     "generate",
		Prompt:   "生成 responses stream 测试脚本",
		Language: "python",
	}, nil)
	if err != nil {
		t.Fatalf("generate ai code via responses stream first: %v", err)
	}
	if requestCount != 1 {
		t.Fatalf("expected exactly one upstream request, got %d", requestCount)
	}
	if !strings.Contains(result.Content, "responses-stream") {
		t.Fatalf("expected responses streaming content, got %q", result.Content)
	}
}

func TestGenerateAICodeOpenAICompatibleUsesStreamFirst(t *testing.T) {
	testutil.SetupTestEnv(t)
	enableAI(t)

	var requestCount int
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("expected /v1/chat/completions, got %s", r.URL.Path)
		}

		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if stream, _ := payload["stream"].(bool); !stream {
			t.Fatalf("expected openai-compatible request to use stream first, got payload %#v", payload)
		}

		w.Header().Set("Content-Type", "text/event-stream")
		if flusher, ok := w.(http.Flusher); ok {
			_, _ = io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"{\\\"summary\\\":\\\"已通过流式返回\\\",\\\"content\\\":\\\"print('stream-large')\\\\n\\\"}\"}}]}\n\n")
			flusher.Flush()
			_, _ = io.WriteString(w, "data: [DONE]\n\n")
			flusher.Flush()
			return
		}

		t.Fatal("expected response writer to support flushing")
	}))
	defer upstream.Close()

	mustSetConfig(t, "ai_default_provider", "openai")
	mustSetConfig(t, "ai_openai_base_url", upstream.URL+"/v1")
	mustSetConfig(t, "ai_openai_api_key", "openai-secret")
	mustSetConfig(t, "ai_openai_model", "gpt-stream-large")

	result, err := service.GenerateAICodeStream(context.Background(), service.AICodeRequest{
		Provider:       "openai",
		Mode:           "modify",
		Prompt:         "补一段异常处理",
		Language:       "python",
		CurrentContent: "print('hello')\n",
	}, nil)
	if err != nil {
		t.Fatalf("generate ai code with streaming first: %v", err)
	}
	if requestCount != 1 {
		t.Fatalf("expected exactly one upstream request, got %d", requestCount)
	}
	if !strings.Contains(result.Content, "stream-large") {
		t.Fatalf("expected streaming-first content, got %q", result.Content)
	}
}

func TestGenerateAICodePatchMode(t *testing.T) {
	testutil.SetupTestEnv(t)
	enableAI(t)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]interface{}{
						"content": `{"summary":"已生成补丁","content":"@@ -1,1+1,1 @@\n-print('bad')\n+print('good')\n"}`,
					},
				},
			},
		})
	}))
	defer upstream.Close()

	mustSetConfig(t, "ai_default_provider", "openai")
	mustSetConfig(t, "ai_openai_base_url", upstream.URL+"/v1")
	mustSetConfig(t, "ai_openai_api_key", "openai-secret")
	mustSetConfig(t, "ai_openai_model", "gpt-test")

	result, err := service.GenerateAICodeStream(context.Background(), service.AICodeRequest{
		Provider:       "openai",
		Mode:           "modify",
		ResponseMode:   "patch",
		Prompt:         "给我一个最小补丁",
		Language:       "python",
		TargetPath:     "demo.py",
		CurrentContent: "print('bad')\n",
	}, nil)
	if err != nil {
		t.Fatalf("generate ai patch: %v", err)
	}
	if result.ResponseMode != "patch" {
		t.Fatalf("expected response mode patch, got %q", result.ResponseMode)
	}
	if !result.CanApply {
		t.Fatalf("expected patch result can_apply true")
	}
	if !strings.Contains(result.Content, "--- a/demo.py") || !strings.Contains(result.Content, "+++ b/demo.py") {
		t.Fatalf("expected patch headers to be injected, got %q", result.Content)
	}
	if !strings.Contains(result.Content, "@@ -1,1 +1,1 @@") {
		t.Fatalf("expected malformed hunk header to be normalized, got %q", result.Content)
	}
	if result.PreviewContent != "print('good')\n" {
		t.Fatalf("expected preview content to contain applied patch result, got %q", result.PreviewContent)
	}
}

func TestGenerateAICodePatchModeRejectsInvalidPatch(t *testing.T) {
	testutil.SetupTestEnv(t)
	enableAI(t)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]interface{}{
						"content": `{"summary":"已尝试生成补丁","content":"把 ENV_NAME 改成 sfsyCK"}`,
					},
				},
			},
		})
	}))
	defer upstream.Close()

	mustSetConfig(t, "ai_default_provider", "openai")
	mustSetConfig(t, "ai_openai_base_url", upstream.URL+"/v1")
	mustSetConfig(t, "ai_openai_api_key", "openai-secret")
	mustSetConfig(t, "ai_openai_model", "gpt-test")

	_, err := service.GenerateAICodeStream(context.Background(), service.AICodeRequest{
		Provider:       "openai",
		Mode:           "modify",
		ResponseMode:   "patch",
		Prompt:         "给我一个最小补丁",
		Language:       "python",
		TargetPath:     "demo.py",
		CurrentContent: "print('bad')\n",
	}, nil)
	if err == nil {
		t.Fatal("expected invalid patch to be rejected")
	}
	if !strings.Contains(err.Error(), "不是合法的 unified diff") {
		t.Fatalf("expected unified diff validation error, got %v", err)
	}
}

func TestGenerateAICodeFallsBackToCodeFenceContentWhenModelDoesNotReturnJSON(t *testing.T) {
	testutil.SetupTestEnv(t)
	enableAI(t)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]interface{}{
						"content": "已按要求修复，代码如下：\n```python\nprint('fenced-fallback')\n```",
					},
				},
			},
		})
	}))
	defer upstream.Close()

	mustSetConfig(t, "ai_default_provider", "openai")
	mustSetConfig(t, "ai_openai_base_url", upstream.URL+"/v1")
	mustSetConfig(t, "ai_openai_api_key", "openai-secret")
	mustSetConfig(t, "ai_openai_model", "gpt-test")

	result, err := service.GenerateAICodeStream(context.Background(), service.AICodeRequest{
		Provider:       "openai",
		Mode:           "modify",
		ResponseMode:   "full",
		Prompt:         "给我修一下",
		Language:       "python",
		CurrentContent: "print('broken')\n",
	}, nil)
	if err != nil {
		t.Fatalf("expected fenced fallback to succeed, got %v", err)
	}
	if !strings.Contains(result.Content, "fenced-fallback") {
		t.Fatalf("expected fenced content fallback, got %q", result.Content)
	}
}

func TestGenerateAICodeFallsBackToLooseJSONFields(t *testing.T) {
	testutil.SetupTestEnv(t)
	enableAI(t)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]interface{}{
						"content": "{\"summary\":\"已修复\",\"content\":\"print('loose-json')\\n\"",
					},
				},
			},
		})
	}))
	defer upstream.Close()

	mustSetConfig(t, "ai_default_provider", "openai")
	mustSetConfig(t, "ai_openai_base_url", upstream.URL+"/v1")
	mustSetConfig(t, "ai_openai_api_key", "openai-secret")
	mustSetConfig(t, "ai_openai_model", "gpt-test")

	result, err := service.GenerateAICodeStream(context.Background(), service.AICodeRequest{
		Provider:       "openai",
		Mode:           "modify",
		ResponseMode:   "full",
		Prompt:         "给我修一下",
		Language:       "python",
		CurrentContent: "print('broken')\n",
	}, nil)
	if err != nil {
		t.Fatalf("expected loose-json fallback to succeed, got %v", err)
	}
	if result.Summary != "已修复" {
		t.Fatalf("expected loose summary fallback, got %q", result.Summary)
	}
	if !strings.Contains(result.Content, "loose-json") {
		t.Fatalf("expected loose-json content fallback, got %q", result.Content)
	}
}

func TestGenerateAICodeExplainModeFallsBackToPlainText(t *testing.T) {
	testutil.SetupTestEnv(t)
	enableAI(t)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]interface{}{
						"content": "问题出在环境变量名不一致，应该把 sfsyUrl 改成 sfsyCK，并保持其它逻辑不变。",
					},
				},
			},
		})
	}))
	defer upstream.Close()

	mustSetConfig(t, "ai_default_provider", "openai")
	mustSetConfig(t, "ai_openai_base_url", upstream.URL+"/v1")
	mustSetConfig(t, "ai_openai_api_key", "openai-secret")
	mustSetConfig(t, "ai_openai_model", "gpt-test")

	result, err := service.GenerateAICodeStream(context.Background(), service.AICodeRequest{
		Provider:       "openai",
		Mode:           "modify",
		ResponseMode:   "explain",
		Prompt:         "解释问题",
		Language:       "python",
		CurrentContent: "print('broken')\n",
	}, nil)
	if err != nil {
		t.Fatalf("expected explain fallback to succeed, got %v", err)
	}
	if !strings.Contains(result.Content, "sfsyCK") {
		t.Fatalf("expected explain text fallback, got %q", result.Content)
	}
}

func TestGenerateAICodeCarriesConversationHistory(t *testing.T) {
	testutil.SetupTestEnv(t)
	enableAI(t)

	var userPrompt string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}

		messages, ok := payload["messages"].([]interface{})
		if !ok || len(messages) < 2 {
			t.Fatalf("expected chat messages, got %#v", payload["messages"])
		}

		message, ok := messages[1].(map[string]interface{})
		if !ok {
			t.Fatalf("expected user message map, got %T", messages[1])
		}
		userPrompt, _ = message["content"].(string)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]interface{}{
						"content": `{"summary":"已基于历史继续优化","content":"print('draft v3')\n"}`,
					},
				},
			},
		})
	}))
	defer upstream.Close()

	mustSetConfig(t, "ai_default_provider", "openai")
	mustSetConfig(t, "ai_openai_base_url", upstream.URL+"/v1")
	mustSetConfig(t, "ai_openai_api_key", "openai-secret")
	mustSetConfig(t, "ai_openai_model", "gpt-history-test")

	_, err := service.GenerateAICodeStream(context.Background(), service.AICodeRequest{
		Provider:       "openai",
		Mode:           "modify",
		Prompt:         "继续把异常处理补齐",
		Language:       "python",
		CurrentContent: "print('draft v2')\n",
		ConversationHistory: []service.AICodeConversationTurn{
			{
				Mode:         "modify",
				ResponseMode: "full",
				Prompt:       "先把日志输出加上",
				Summary:      "已添加日志",
				Content:      "print('draft v1')\n",
			},
			{
				Mode:         "modify",
				ResponseMode: "full",
				Prompt:       "再把变量名改清楚",
				Summary:      "已重命名变量",
				Content:      "print('draft v2')\n",
			},
		},
	}, nil)
	if err != nil {
		t.Fatalf("generate ai code with conversation history: %v", err)
	}
	if !strings.Contains(userPrompt, "<conversation_history>") {
		t.Fatalf("expected conversation history block in user prompt, got %q", userPrompt)
	}
	if !strings.Contains(userPrompt, "先把日志输出加上") || !strings.Contains(userPrompt, "再把变量名改清楚") {
		t.Fatalf("expected previous prompts in conversation history, got %q", userPrompt)
	}
	if !strings.Contains(userPrompt, "AI 摘要: 已重命名变量") {
		t.Fatalf("expected previous summary in conversation history, got %q", userPrompt)
	}
}

func TestGenerateAICodeInjectsPanelRuntimeGuardrails(t *testing.T) {
	testutil.SetupTestEnv(t)
	enableAI(t)

	var systemPrompt string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}

		messages, ok := payload["messages"].([]interface{})
		if !ok || len(messages) < 2 {
			t.Fatalf("expected chat messages, got %#v", payload["messages"])
		}

		message, ok := messages[0].(map[string]interface{})
		if !ok {
			t.Fatalf("expected system message map, got %T", messages[0])
		}
		systemPrompt, _ = message["content"].(string)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]interface{}{
						"content": `{"summary":"已生成符合面板运行时的脚本","content":"print('ok')\n"}`,
					},
				},
			},
		})
	}))
	defer upstream.Close()

	mustSetConfig(t, "ai_default_provider", "openai")
	mustSetConfig(t, "ai_openai_base_url", upstream.URL+"/v1")
	mustSetConfig(t, "ai_openai_api_key", "openai-secret")
	mustSetConfig(t, "ai_openai_model", "gpt-guardrail-test")

	_, err := service.GenerateAICodeStream(context.Background(), service.AICodeRequest{
		Provider: "openai",
		Mode:     "generate",
		Prompt:   "生成一个会在面板里运行的通知脚本",
		Language: "python",
	}, nil)
	if err != nil {
		t.Fatalf("generate ai code with panel guardrails: %v", err)
	}
	for _, fragment := range []string{
		"非交互式托管运行时",
		"不要要求用户额外传递命令行参数",
		"notify.py 的 send(...)",
		"sendNotify.js 的 sendNotify(...)",
		"/api/v1/notifications/send",
		"/api/v1/open-api/token",
		"/api/v1/envs、/api/v1/tasks、/api/v1/scripts",
		"以下安全约束是固定规则",
		"不要生成用于绕过鉴权、伪造身份、窃取或回显 Authorization",
		"不要生成反弹 shell、远程命令执行后门",
	} {
		if !strings.Contains(systemPrompt, fragment) {
			t.Fatalf("expected system prompt to contain %q, got %q", fragment, systemPrompt)
		}
	}
}

func TestGenerateAICodeAppendsCustomPromptFromSystemConfig(t *testing.T) {
	testutil.SetupTestEnv(t)
	enableAI(t)

	var systemPrompt string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}

		messages, ok := payload["messages"].([]interface{})
		if !ok || len(messages) < 2 {
			t.Fatalf("expected chat messages, got %#v", payload["messages"])
		}

		message, ok := messages[0].(map[string]interface{})
		if !ok {
			t.Fatalf("expected system message map, got %T", messages[0])
		}
		systemPrompt, _ = message["content"].(string)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]interface{}{
						"content": `{"summary":"已生成脚本","content":"print('ok')\n"}`,
					},
				},
			},
		})
	}))
	defer upstream.Close()

	mustSetConfig(t, "ai_default_provider", "openai")
	mustSetConfig(t, "ai_openai_base_url", upstream.URL+"/v1")
	mustSetConfig(t, "ai_openai_api_key", "openai-secret")
	mustSetConfig(t, "ai_openai_model", "gpt-custom-prompt-test")
	mustSetConfig(t, "ai_code_custom_prompt", "默认使用中文日志，并把网络请求超时写清楚。")

	_, err := service.GenerateAICodeStream(context.Background(), service.AICodeRequest{
		Provider: "openai",
		Mode:     "generate",
		Prompt:   "生成一个定时脚本",
		Language: "python",
	}, nil)
	if err != nil {
		t.Fatalf("generate ai code with custom prompt: %v", err)
	}
	if !strings.Contains(systemPrompt, "系统设置附加要求") {
		t.Fatalf("expected custom prompt section, got %q", systemPrompt)
	}
	if !strings.Contains(systemPrompt, "默认使用中文日志，并把网络请求超时写清楚。") {
		t.Fatalf("expected custom prompt content, got %q", systemPrompt)
	}
}

func TestTestAICodeProviderConnectionUsesProtocolOverrides(t *testing.T) {
	testutil.SetupTestEnv(t)
	enableAI(t)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/relay/messages" {
			t.Fatalf("expected /relay/messages, got %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-secret" {
			t.Fatalf("expected override api key, got %q", got)
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

	result, err := service.TestAICodeProviderConnection(service.AICodeProviderTestRequest{
		Provider:     "anthropic",
		BaseURL:      upstream.URL + "/relay/messages",
		APIKey:       "test-secret",
		Model:        "claude-connect-test",
		APIFormat:    "anthropic",
		AuthStrategy: "bearer",
		IsFullURL:    boolPtr(true),
	})
	if err != nil {
		t.Fatalf("test ai provider connection: %v", err)
	}
	if result.Provider != "anthropic" {
		t.Fatalf("expected provider anthropic, got %q", result.Provider)
	}
	if result.Model != "claude-connect-test" {
		t.Fatalf("expected model claude-connect-test, got %q", result.Model)
	}
	if !strings.Contains(result.Message, "OK") {
		t.Fatalf("expected success message to contain OK, got %q", result.Message)
	}
}
