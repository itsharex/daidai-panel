package service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"daidai-panel/config"
	"daidai-panel/middleware"
	"daidai-panel/testutil"
)

func TestBuildNotifyHelperEnvCreatesManagedHelpers(t *testing.T) {
	root := testutil.SetupTestEnv(t)

	scriptsDir := config.C.Data.ScriptsDir
	workDir := filepath.Join(scriptsDir, "nested")

	env, err := BuildNotifyHelperEnv(scriptsDir, workDir, config.C.Server.Port, nil, time.Hour)
	if err != nil {
		t.Fatalf("build notify helper env: %v", err)
	}

	if env["DAIDAI_NOTIFY_URL"] == "" || env["DAIDAI_NOTIFY_TOKEN"] == "" {
		t.Fatalf("expected notify url/token in env, got %#v", env)
	}
	if _, err := middleware.ParseToken(env["DAIDAI_NOTIFY_TOKEN"]); err != nil {
		t.Fatalf("parse helper token: %v", err)
	}

	paths := []string{
		filepath.Join(scriptsDir, notifyPyFilename),
		filepath.Join(scriptsDir, sendNotifyJSFilename),
		filepath.Join(workDir, notifyPyFilename),
		filepath.Join(workDir, sendNotifyJSFilename),
	}
	for _, path := range paths {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read helper %s: %v", path, err)
		}
		if !strings.Contains(string(content), "DAIDAI_PANEL_MANAGED_NOTIFY_HELPER") {
			t.Fatalf("expected helper marker in %s", path)
		}
	}

	if _, err := os.Stat(filepath.Join(root, "data")); err != nil {
		t.Fatalf("expected test data dir to exist: %v", err)
	}
}

func TestEnsureManagedHelperFileRewritesManagedJSFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), sendNotifyJSFilename)
	if err := os.WriteFile(path, []byte("// "+managedNotifyHelperToken+"\nmodule.exports = {}\n"), 0o644); err != nil {
		t.Fatalf("seed helper file: %v", err)
	}

	if err := ensureManagedHelperFile(path, managedSendNotifyJSContent+"\n"); err != nil {
		t.Fatalf("rewrite managed helper file: %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read rewritten helper file: %v", err)
	}
	if string(content) != managedSendNotifyJSContent+"\n" {
		t.Fatalf("expected managed JS helper to be refreshed")
	}
}

func TestManagedHelperContentIncludesUsageDocs(t *testing.T) {
	if !strings.Contains(managedNotifyPyContent, "Usage:") {
		t.Fatalf("expected python helper usage docs")
	}
	if !strings.Contains(managedNotifyPyContent, "def send(title, content, ignore_default_config=False, **kwargs):") {
		t.Fatalf("expected python helper send signature docs")
	}
	if !strings.Contains(managedSendNotifyJSContent, "QingLong-style notify entry point.") {
		t.Fatalf("expected js helper entry point docs")
	}
	if !strings.Contains(managedSendNotifyJSContent, "@param {object} params") {
		t.Fatalf("expected js helper JSDoc params")
	}
}

func TestAppendScriptHelperPathsKeepsExistingEntries(t *testing.T) {
	env := map[string]string{
		"NODE_PATH":  "/tmp/node_modules",
		"PYTHONPATH": "/tmp/site-packages",
	}

	AppendScriptHelperPaths(env, "/tmp/scripts")
	AppendScriptHelperPaths(env, "/tmp/scripts")

	if got := env["NODE_PATH"]; !strings.Contains(got, "/tmp/node_modules") || !strings.Contains(got, "/tmp/scripts") {
		t.Fatalf("unexpected NODE_PATH: %q", got)
	}
	if strings.Count(env["NODE_PATH"], "/tmp/scripts") != 1 {
		t.Fatalf("expected deduplicated NODE_PATH, got %q", env["NODE_PATH"])
	}
	if got := env["PYTHONPATH"]; !strings.Contains(got, "/tmp/site-packages") || !strings.Contains(got, "/tmp/scripts") {
		t.Fatalf("unexpected PYTHONPATH: %q", got)
	}
}
