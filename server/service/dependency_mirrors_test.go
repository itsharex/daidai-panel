package service

import "testing"

func TestEffectivePipMirrorFallsBackToDefaultAcceleratedMirror(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{name: "empty uses default", input: "", want: DefaultPipMirror},
		{name: "official uses default", input: "https://pypi.org/simple", want: DefaultPipMirror},
		{name: "custom mirror preserved", input: "https://mirrors.aliyun.com/pypi/simple", want: "https://mirrors.aliyun.com/pypi/simple"},
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
