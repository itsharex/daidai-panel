package service

import (
	"path/filepath"
	"testing"
)

func TestEffectivePipMirrorFallsBackToDefaultAcceleratedMirror(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{name: "empty uses default", input: "", want: DefaultPipMirror},
		{name: "official uses default", input: "https://pypi.org/simple", want: DefaultPipMirror},
		{name: "custom mirror preserved", input: "https://pypi.tuna.tsinghua.edu.cn/simple", want: "https://pypi.tuna.tsinghua.edu.cn/simple"},
	}

	for _, tc := range cases {
		if got := EffectivePipMirror(tc.input); got != tc.want {
			t.Fatalf("%s: EffectivePipMirror(%q) = %q, want %q", tc.name, tc.input, got, tc.want)
		}
	}
}

func TestEffectiveNpmMirrorFallsBackToDefaultAcceleratedMirror(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{name: "empty uses default", input: "", want: "https://registry.npmmirror.com/"},
		{name: "official uses default", input: "https://registry.npmjs.org/", want: "https://registry.npmmirror.com/"},
		{name: "custom mirror preserved", input: "https://mirrors.cloud.tencent.com/npm/", want: "https://mirrors.cloud.tencent.com/npm/"},
	}

	for _, tc := range cases {
		if got := EffectiveNpmMirror(tc.input); got != tc.want {
			t.Fatalf("%s: EffectiveNpmMirror(%q) = %q, want %q", tc.name, tc.input, got, tc.want)
		}
	}
}

func TestSetPipMirrorWritesAndClearsConfig(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))

	if err := SetPipMirror("https://mirrors.aliyun.com/pypi/simple"); err != nil {
		t.Fatalf("set pip mirror: %v", err)
	}
	if got := CurrentPipMirror(); got != "https://mirrors.aliyun.com/pypi/simple" {
		t.Fatalf("expected saved pip mirror, got %q", got)
	}

	if err := SetPipMirror(""); err != nil {
		t.Fatalf("clear pip mirror: %v", err)
	}
	if got := CurrentPipMirror(); got != "" {
		t.Fatalf("expected cleared pip mirror, got %q", got)
	}
}

func TestSetNpmMirrorWritesAndClearsConfig(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	t.Setenv("HOME", home)

	if err := SetNpmMirror("https://mirrors.cloud.tencent.com/npm/"); err != nil {
		t.Fatalf("set npm mirror: %v", err)
	}
	if got := CurrentNpmMirror(); got != "https://mirrors.cloud.tencent.com/npm/" {
		t.Fatalf("expected saved npm mirror, got %q", got)
	}

	if err := SetNpmMirror(""); err != nil {
		t.Fatalf("clear npm mirror: %v", err)
	}
	if got := CurrentNpmMirror(); got != "" {
		t.Fatalf("expected cleared npm mirror, got %q", got)
	}
}
