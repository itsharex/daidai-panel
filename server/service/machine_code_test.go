package service

import (
	"regexp"
	"testing"

	"daidai-panel/database"
	"daidai-panel/model"
	"daidai-panel/testutil"
)

var machineCodeFormatRe = regexp.MustCompile(`^[0-9A-F]{4}(?:-[0-9A-F]{4}){7}$`)

func TestEnsureMachineCodeGeneratesAndPersists(t *testing.T) {
	testutil.SetupTestEnv(t)
	ResetMachineCodeCacheForTest()

	first := EnsureMachineCode()
	if !machineCodeFormatRe.MatchString(first) {
		t.Fatalf("expected machine code formatted XXXX-XXXX-...(8 groups), got %q", first)
	}

	second := EnsureMachineCode()
	if second != first {
		t.Fatalf("expected repeated calls to return the same code, got %q vs %q", first, second)
	}

	var row model.SystemConfig
	if err := database.DB.Where("`key` = ?", "machine_code").First(&row).Error; err != nil {
		t.Fatalf("query persisted machine_code row: %v", err)
	}
	if row.Value != first {
		t.Fatalf("expected persisted value %q, got %q", first, row.Value)
	}
}

func TestEnsureMachineCodeSurvivesCacheResetAndReturnsStoredValue(t *testing.T) {
	testutil.SetupTestEnv(t)
	ResetMachineCodeCacheForTest()

	original := EnsureMachineCode()

	// Simulate a process restart: drop in-memory cache, keep DB row.
	ResetMachineCodeCacheForTest()

	reloaded := EnsureMachineCode()
	if reloaded != original {
		t.Fatalf("expected stored code to survive cache reset, got %q originally, %q after reset", original, reloaded)
	}
}

func TestEnsureMachineCodeFillsBlankExistingRow(t *testing.T) {
	testutil.SetupTestEnv(t)
	ResetMachineCodeCacheForTest()

	blank := model.SystemConfig{Key: "machine_code", Value: "", Description: "blank"}
	if err := database.DB.Create(&blank).Error; err != nil {
		t.Fatalf("seed blank row: %v", err)
	}

	code := EnsureMachineCode()
	if !machineCodeFormatRe.MatchString(code) {
		t.Fatalf("expected generated code for blank row, got %q", code)
	}

	var row model.SystemConfig
	if err := database.DB.Where("`key` = ?", "machine_code").First(&row).Error; err != nil {
		t.Fatalf("query row: %v", err)
	}
	if row.Value != code {
		t.Fatalf("expected blank row filled with %q, got %q", code, row.Value)
	}
}

func TestFormatMachineCodeShape(t *testing.T) {
	raw := []byte{
		0x01, 0x23, 0x45, 0x67, 0x89, 0xAB, 0xCD, 0xEF,
		0x10, 0x32, 0x54, 0x76, 0x98, 0xBA, 0xDC, 0xFE,
	}
	got := formatMachineCode(raw)
	want := "0123-4567-89AB-CDEF-1032-5476-98BA-DCFE"
	if got != want {
		t.Fatalf("formatMachineCode: want %q, got %q", want, got)
	}
}
