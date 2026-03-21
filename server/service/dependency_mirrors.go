package service

import (
	"os"
	"strings"
)

const (
	DefaultPipMirror = "https://pypi.tuna.tsinghua.edu.cn/simple"
	DefaultNpmMirror = "https://registry.npmmirror.com"
)

func EffectivePipMirror(configured string) string {
	mirror := strings.TrimSpace(configured)
	if mirror == "" || isOfficialPipMirror(mirror) {
		return DefaultPipMirror
	}
	return mirror
}

func EffectiveNpmMirror(configured string) string {
	mirror := normalizeNpmMirror(configured)
	if mirror == "" || isOfficialNpmMirror(mirror) {
		return normalizeNpmMirror(DefaultNpmMirror)
	}
	return mirror
}

func PipInstallEnv(base []string, configured string) []string {
	env := append([]string{}, base...)
	mirror := EffectivePipMirror(configured)
	if mirror == "" {
		return env
	}

	env = append(env, "PIP_INDEX_URL="+mirror)
	if host := extractMirrorHost(mirror); host != "" {
		env = append(env, "PIP_TRUSTED_HOST="+host)
	}
	return env
}

func NpmInstallEnv(base []string, configured string) []string {
	env := append([]string{}, base...)
	mirror := EffectiveNpmMirror(configured)
	if mirror == "" {
		return env
	}

	env = append(env, "npm_config_registry="+mirror)
	env = append(env, "NPM_CONFIG_REGISTRY="+mirror)
	return env
}

func CurrentPipMirror() string {
	if out, err := os.ReadFile(pipMirrorConfigPath()); err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(strings.ToLower(line), "index-url") {
				parts := strings.SplitN(line, "=", 2)
				if len(parts) == 2 {
					return strings.TrimSpace(parts[1])
				}
			}
		}
	}
	return ""
}

func CurrentNpmMirror() string {
	if out, err := os.ReadFile(npmConfigPath()); err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(strings.ToLower(line), "registry=") {
				parts := strings.SplitN(line, "=", 2)
				if len(parts) == 2 {
					return strings.TrimSpace(parts[1])
				}
			}
		}
	}
	return ""
}

func CurrentEffectivePipMirror() string {
	return EffectivePipMirror(CurrentPipMirror())
}

func CurrentEffectiveNpmMirror() string {
	return EffectiveNpmMirror(CurrentNpmMirror())
}

func isOfficialPipMirror(mirror string) bool {
	mirror = strings.TrimRight(strings.ToLower(strings.TrimSpace(mirror)), "/")
	switch mirror {
	case "https://pypi.org/simple", "http://pypi.org/simple",
		"https://pypi.python.org/simple", "http://pypi.python.org/simple":
		return true
	default:
		return false
	}
}

func isOfficialNpmMirror(mirror string) bool {
	mirror = normalizeNpmMirror(mirror)
	return mirror == "https://registry.npmjs.org/"
}

func normalizeNpmMirror(mirror string) string {
	mirror = strings.TrimSpace(mirror)
	if mirror == "" {
		return ""
	}
	if !strings.HasSuffix(mirror, "/") {
		mirror += "/"
	}
	return mirror
}

func extractMirrorHost(url string) string {
	url = strings.TrimPrefix(url, "https://")
	url = strings.TrimPrefix(url, "http://")
	parts := strings.SplitN(url, "/", 2)
	if len(parts) == 0 {
		return ""
	}
	hostPort := strings.SplitN(parts[0], ":", 2)
	return strings.TrimSpace(hostPort[0])
}

func pipMirrorConfigPath() string {
	if xdg := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")); xdg != "" {
		return xdg + "/pip/pip.conf"
	}
	if home := strings.TrimSpace(os.Getenv("HOME")); home != "" {
		return home + "/.config/pip/pip.conf"
	}
	return ""
}

func npmConfigPath() string {
	if home := strings.TrimSpace(os.Getenv("HOME")); home != "" {
		return home + "/.npmrc"
	}
	return ""
}
