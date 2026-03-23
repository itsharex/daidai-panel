package handler

import "testing"

func TestScriptCommandParts(t *testing.T) {
	parts, err := scriptCommandParts(".py", "demo.py")
	if err != nil {
		t.Fatalf("expected python command, got error: %v", err)
	}
	if len(parts) != 3 || parts[0] != "python" || parts[1] != "-u" || parts[2] != "demo.py" {
		t.Fatalf("unexpected command parts: %#v", parts)
	}
}

func TestScriptCommandPartsSupportsGo(t *testing.T) {
	parts, err := scriptCommandParts(".go", "demo.go")
	if err != nil {
		t.Fatalf("expected go command, got error: %v", err)
	}
	if len(parts) != 3 || parts[0] != "go" || parts[1] != "run" || parts[2] != "demo.go" {
		t.Fatalf("unexpected go command parts: %#v", parts)
	}
}

func TestScriptCommandPartsRejectsUnsupportedExtension(t *testing.T) {
	if _, err := scriptCommandParts(".rb", "demo.rb"); err == nil {
		t.Fatal("expected unsupported extension error")
	}
}

func TestScriptLanguageExtMapSupportsGo(t *testing.T) {
	if got := scriptLanguageExtMap["go"]; got != ".go" {
		t.Fatalf("expected go language map to .go, got %q", got)
	}
}

func TestDetectMissingDep(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   string
	}{
		{
			name:   "node_module",
			output: "Error: Cannot find module 'axios'",
			want:   "axios",
		},
		{
			name:   "node_relative_module",
			output: "Error: Cannot find module './local-helper'",
			want:   "",
		},
		{
			name:   "python_module",
			output: "ModuleNotFoundError: No module named 'requests.sessions'",
			want:   "requests",
		},
		{
			name:   "no_match",
			output: "plain output",
			want:   "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := detectMissingDep(tc.output); got != tc.want {
				t.Fatalf("detectMissingDep(%q) = %q, want %q", tc.output, got, tc.want)
			}
		})
	}
}
