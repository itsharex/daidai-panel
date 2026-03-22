package service

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"daidai-panel/config"
	"daidai-panel/testutil"
)

func TestParseCommandExecutionPlanSupportsTaskModesAndArgs(t *testing.T) {
	testutil.SetupTestEnv(t)

	spacedScript := filepath.Join(config.C.Data.ScriptsDir, "demo folder", "my script.py")
	if err := os.MkdirAll(filepath.Dir(spacedScript), 0755); err != nil {
		t.Fatalf("mkdir spaced script dir: %v", err)
	}
	if err := os.WriteFile(spacedScript, []byte("print('ok')\n"), 0644); err != nil {
		t.Fatalf("write spaced script: %v", err)
	}

	simpleScript := filepath.Join(config.C.Data.ScriptsDir, "simple.sh")
	if err := os.WriteFile(simpleScript, []byte("echo ok\n"), 0755); err != nil {
		t.Fatalf("write simple script: %v", err)
	}

	t.Run("parses now mode with timeout override and passthrough args", func(t *testing.T) {
		plan, err := ParseCommandExecutionPlan(`task -m 5m demo folder/my script.py now -- -u whyour -p password`, config.C.Data.ScriptsDir)
		if err != nil {
			t.Fatalf("parse task now plan: %v", err)
		}

		if plan.Interpreter != "python3" {
			t.Fatalf("expected python3 interpreter, got %q", plan.Interpreter)
		}
		expectedInfo, err := os.Stat(spacedScript)
		if err != nil {
			t.Fatalf("stat expected spaced script: %v", err)
		}
		actualInfo, err := os.Stat(plan.FullPath)
		if err != nil {
			t.Fatalf("stat actual spaced script: %v", err)
		}
		if !os.SameFile(expectedInfo, actualInfo) {
			t.Fatalf("expected plan path %q to reference %q", plan.FullPath, spacedScript)
		}
		if plan.TimeoutOverride == nil || *plan.TimeoutOverride != 300 {
			t.Fatalf("expected timeout override 300, got %#v", plan.TimeoutOverride)
		}
		if !plan.SkipRandomDelay {
			t.Fatal("expected now mode to skip random delay")
		}
		if plan.Mode != commandModeNow {
			t.Fatalf("expected now mode, got %q", plan.Mode)
		}
		if !reflect.DeepEqual(plan.ScriptArgs, []string{"-u", "whyour", "-p", "password"}) {
			t.Fatalf("unexpected script args: %#v", plan.ScriptArgs)
		}
	})

	t.Run("parses conc mode with env and account spec", func(t *testing.T) {
		plan, err := ParseCommandExecutionPlan(`task simple.sh conc JD_COOKIE 1-2`, config.C.Data.ScriptsDir)
		if err != nil {
			t.Fatalf("parse task conc plan: %v", err)
		}

		if plan.Mode != commandModeConc {
			t.Fatalf("expected conc mode, got %q", plan.Mode)
		}
		if !plan.SuppressLiveOutput {
			t.Fatal("expected conc mode to suppress live output")
		}
		if plan.EnvName != "JD_COOKIE" {
			t.Fatalf("expected env name JD_COOKIE, got %q", plan.EnvName)
		}
		if plan.AccountSpec != "1-2" {
			t.Fatalf("expected account spec 1-2, got %q", plan.AccountSpec)
		}
	})

	t.Run("parses designated env selection", func(t *testing.T) {
		plan, err := ParseCommandExecutionPlan(`task simple.sh desi JD_COOKIE 2`, config.C.Data.ScriptsDir)
		if err != nil {
			t.Fatalf("parse task desi plan: %v", err)
		}

		if plan.Mode != commandModeDesi {
			t.Fatalf("expected desi mode, got %q", plan.Mode)
		}
		if plan.EnvName != "JD_COOKIE" {
			t.Fatalf("expected env name JD_COOKIE, got %q", plan.EnvName)
		}
		if plan.AccountSpec != "2" {
			t.Fatalf("expected account spec 2, got %q", plan.AccountSpec)
		}
	})
}

func TestResolveTaskAccountSelections(t *testing.T) {
	envVars := map[string]string{
		"JD_COOKIE": "a&b&c",
	}

	selections, err := resolveTaskAccountSelections(envVars, "JD_COOKIE", "1-2 3")
	if err != nil {
		t.Fatalf("resolve task account selections: %v", err)
	}

	got := make([]string, 0, len(selections))
	for _, selection := range selections {
		got = append(got, selection.Value)
	}

	if !reflect.DeepEqual(got, []string{"a", "b", "c"}) {
		t.Fatalf("unexpected selected values: %#v", got)
	}
}

func TestApplyCommandEnvOverridesForDesi(t *testing.T) {
	plan := &CommandExecutionPlan{
		Mode:        commandModeDesi,
		EnvName:     "JD_COOKIE",
		AccountSpec: "2-3",
	}
	envVars := map[string]string{
		"JD_COOKIE": "a&b&c",
	}

	overridden, err := applyCommandEnvOverrides(plan, envVars)
	if err != nil {
		t.Fatalf("apply designated env overrides: %v", err)
	}
	if overridden["JD_COOKIE"] != "b&c" {
		t.Fatalf("expected designated env values b&c, got %q", overridden["JD_COOKIE"])
	}
	if overridden["envParam"] != "JD_COOKIE" {
		t.Fatalf("expected envParam JD_COOKIE, got %q", overridden["envParam"])
	}
	if overridden["numParam"] != "2 3" {
		t.Fatalf("expected numParam '2 3', got %q", overridden["numParam"])
	}
}
