package service

import (
	"encoding/json"
	"testing"
	"time"

	"daidai-panel/testutil"
)

func TestBuildManagedRuntimeEnvMapIncludesPythonAutoInstallSettings(t *testing.T) {
	root := testutil.SetupTestEnv(t)

	envMap, err := BuildManagedRuntimeEnvMap(root, root, nil, time.Hour)
	if err != nil {
		t.Fatalf("build managed runtime env map: %v", err)
	}

	if got := envMap["DD_AUTO_INSTALL_DEPS"]; got != "1" {
		t.Fatalf("expected DD_AUTO_INSTALL_DEPS=1, got %q", got)
	}

	var aliases map[string]string
	if err := json.Unmarshal([]byte(envMap["DD_PY_AUTO_INSTALL_ALIASES"]), &aliases); err != nil {
		t.Fatalf("decode DD_PY_AUTO_INSTALL_ALIASES: %v", err)
	}
	if got := aliases["crypto"]; got != "pycryptodome" {
		t.Fatalf("expected crypto alias to be pycryptodome, got %q", got)
	}
}
