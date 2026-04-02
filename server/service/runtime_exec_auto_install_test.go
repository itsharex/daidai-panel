package service

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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

func TestBuildManagedPythonPathPrioritizesWorkDirAndScriptsDir(t *testing.T) {
	got := buildManagedPythonPath(
		filepath.Clean("/custom/pythonpath"),
		filepath.Clean("/work/scripts/subdir"),
		filepath.Clean("/work/scripts"),
		filepath.Clean("/deps/python/venv/lib/python3.11/site-packages"),
	)

	parts := strings.Split(got, string(os.PathListSeparator))
	want := []string{
		filepath.Clean("/work/scripts/subdir"),
		filepath.Clean("/work/scripts"),
		filepath.Clean("/custom/pythonpath"),
		filepath.Clean("/deps/python/venv/lib/python3.11/site-packages"),
	}

	if len(parts) != len(want) {
		t.Fatalf("unexpected python path parts: got=%v want=%v", parts, want)
	}
	for idx, expected := range want {
		if parts[idx] != expected {
			t.Fatalf("python path order mismatch at %d: got=%q want=%q (all=%v)", idx, parts[idx], expected, parts)
		}
	}
}
