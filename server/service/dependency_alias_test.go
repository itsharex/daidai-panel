package service

import (
	"encoding/json"
	"testing"
)

func TestResolvePythonAutoInstallPackage(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{name: "crypto alias", input: "Crypto", expect: "pycryptodome"},
		{name: "execjs alias", input: "execjs", expect: "pyexecjs"},
		{name: "case insensitive", input: "crypto", expect: "pycryptodome"},
		{name: "passthrough", input: "requests", expect: "requests"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := ResolvePythonAutoInstallPackage(tc.input); got != tc.expect {
				t.Fatalf("expected %q, got %q", tc.expect, got)
			}
		})
	}
}

func TestEncodePythonAutoInstallAliases(t *testing.T) {
	var decoded map[string]string
	if err := json.Unmarshal([]byte(EncodePythonAutoInstallAliases()), &decoded); err != nil {
		t.Fatalf("decode aliases json: %v", err)
	}
	if got := decoded["crypto"]; got != "pycryptodome" {
		t.Fatalf("expected crypto alias to be pycryptodome, got %q", got)
	}
	if got := decoded["execjs"]; got != "pyexecjs" {
		t.Fatalf("expected execjs alias to be pyexecjs, got %q", got)
	}
}
