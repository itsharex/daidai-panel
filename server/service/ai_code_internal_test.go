package service

import (
	"strings"
	"testing"
	"time"
)

func TestWithAdaptiveAICodeRuntimeConfigExtendsTimeoutForLongPrompt(t *testing.T) {
	cfg := aiProviderRuntimeConfig{Timeout: 30 * time.Second}

	adapted := withAdaptiveAICodeRuntimeConfig(
		cfg,
		"system",
		strings.Repeat("x", aiPromptCharsForLongTimeout+128),
	)

	if adapted.Timeout != 300*time.Second {
		t.Fatalf("expected long prompt timeout to extend to 300s, got %v", adapted.Timeout)
	}
}

func TestWithAdaptiveAICodeRuntimeConfigKeepsHigherConfiguredTimeout(t *testing.T) {
	cfg := aiProviderRuntimeConfig{Timeout: 420 * time.Second}

	adapted := withAdaptiveAICodeRuntimeConfig(
		cfg,
		"system",
		strings.Repeat("x", aiPromptCharsForLongTimeout+128),
	)

	if adapted.Timeout != 420*time.Second {
		t.Fatalf("expected higher configured timeout to be preserved, got %v", adapted.Timeout)
	}
}

func TestNormalizeUnifiedDiffPatchCompletesMissingHeader(t *testing.T) {
	patch, err := normalizeUnifiedDiffPatch("+++ b/demo.py\n@@ -1,1 +1,1 @@\n-print('bad')\n+print('good')", "demo.py")
	if err != nil {
		t.Fatalf("normalize patch: %v", err)
	}
	if !strings.Contains(patch, "--- a/demo.py") || !strings.Contains(patch, "+++ b/demo.py") {
		t.Fatalf("expected normalizeUnifiedDiffPatch to complete missing header, got %q", patch)
	}
}

func TestNormalizeUnifiedDiffPatchNormalizesTightHeaders(t *testing.T) {
	patch, err := normalizeUnifiedDiffPatch("---a/demo.py\n+++b/demo.py\n@@-1,1+1,1@@\n-print('bad')\n+print('good')", "demo.py")
	if err != nil {
		t.Fatalf("normalize patch with tight headers: %v", err)
	}
	if !strings.Contains(patch, "--- a/demo.py") || !strings.Contains(patch, "+++ b/demo.py") {
		t.Fatalf("expected file headers to be normalized, got %q", patch)
	}
	if !strings.Contains(patch, "@@ -1,1 +1,1 @@") {
		t.Fatalf("expected hunk header to be normalized, got %q", patch)
	}
}

func TestApplyUnifiedDiffPatch(t *testing.T) {
	result, err := applyUnifiedDiffPatch(
		"APP_NAME = '顺丰'\nENV_NAME = 'sfsyUrl'\nPROXY = ''\n",
		"--- a/demo.py\n+++ b/demo.py\n@@ -1,3 +1,3 @@\n APP_NAME = '顺丰'\n-ENV_NAME = 'sfsyUrl'\n+ENV_NAME = 'sfsyCK'\n PROXY = ''\n",
	)
	if err != nil {
		t.Fatalf("apply patch: %v", err)
	}
	expected := "APP_NAME = '顺丰'\nENV_NAME = 'sfsyCK'\nPROXY = ''\n"
	if result != expected {
		t.Fatalf("expected applied patch result %q, got %q", expected, result)
	}
}

func TestApplyUnifiedDiffPatchToleratesMissingContextPrefixes(t *testing.T) {
	result, err := applyUnifiedDiffPatch(
		"class Config:\n\"\"\"全局配置\"\"\"\nAPP_NAME:str=\"顺丰速运\"\nVERSION:str=\"1.2.0\"\nENV_NAME:str=\"sfsyUrl\"\nPROXY_API_URL:str=os.getenv('SF_PROXY_API_URL','')\n\n# 代理相关配置常量\n",
		"--- a/顺丰.py\n+++ b/顺丰.py\n@@ -1,7 +1,7 @@\nclass Config:\n\"\"\"全局配置\"\"\"\nAPP_NAME:str=\"顺丰速运\"\nVERSION:str=\"1.2.0\"\n-ENV_NAME:str=\"sfsyUrl\"\n+ENV_NAME:str=\"SFSYCK\"\nPROXY_API_URL:str=os.getenv('SF_PROXY_API_URL','')\n\n# 代理相关配置常量\n",
	)
	if err != nil {
		t.Fatalf("apply patch with missing context prefixes: %v", err)
	}
	if !strings.Contains(result, "ENV_NAME:str=\"SFSYCK\"") {
		t.Fatalf("expected patched env name, got %q", result)
	}
}

func TestNormalizeEscapedScriptLikeContentDecodesVisibleEscapes(t *testing.T) {
	raw := `"""\\n顺丰速运日常积分任务\\n"""\\nimport os\\nENV_NAME = "sfsyCK"\\n`

	normalized := normalizeEscapedScriptLikeContent(raw)

	if strings.Contains(normalized, `\n`) {
		t.Fatalf("expected visible escapes to be decoded, got %q", normalized)
	}
	if !strings.Contains(normalized, "ENV_NAME = \"sfsyCK\"") {
		t.Fatalf("expected decoded script content, got %q", normalized)
	}
}
