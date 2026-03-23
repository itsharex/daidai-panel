package service

import "testing"

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
