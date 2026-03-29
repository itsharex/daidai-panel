package handler

import (
	"errors"
	"strings"
	"testing"
)

func TestResolveUpdateImageTargetUsesMirrorForDockerHubImage(t *testing.T) {
	pullImage, mirrorHost, registryURL := resolveUpdateImageTarget("linzixuanzz/daidai-panel:latest", "docker.1ms.run")

	if pullImage != "docker.1ms.run/linzixuanzz/daidai-panel:latest" {
		t.Fatalf("expected mirrored pull image, got %q", pullImage)
	}
	if mirrorHost != "docker.1ms.run" {
		t.Fatalf("expected mirror host docker.1ms.run, got %q", mirrorHost)
	}
	if registryURL != "https://docker.1ms.run/v2/" {
		t.Fatalf("expected mirror registry url, got %q", registryURL)
	}
}

func TestResolveUpdateImageTargetStripsExplicitDockerHubHost(t *testing.T) {
	pullImage, mirrorHost, registryURL := resolveUpdateImageTarget("docker.io/linzixuanzz/daidai-panel:latest", "docker.1ms.run")

	if pullImage != "docker.1ms.run/linzixuanzz/daidai-panel:latest" {
		t.Fatalf("expected mirrored pull image without explicit docker.io prefix, got %q", pullImage)
	}
	if mirrorHost != "docker.1ms.run" {
		t.Fatalf("expected mirror host docker.1ms.run, got %q", mirrorHost)
	}
	if registryURL != "https://docker.1ms.run/v2/" {
		t.Fatalf("expected mirror registry url, got %q", registryURL)
	}
}

func TestResolveUpdateImageTargetKeepsCustomRegistryDirect(t *testing.T) {
	pullImage, mirrorHost, registryURL := resolveUpdateImageTarget("ghcr.io/acme/panel:latest", "docker.1ms.run")

	if pullImage != "ghcr.io/acme/panel:latest" {
		t.Fatalf("expected custom registry image to remain unchanged, got %q", pullImage)
	}
	if mirrorHost != "" {
		t.Fatalf("expected mirror host to be ignored for custom registry, got %q", mirrorHost)
	}
	if registryURL != "https://ghcr.io/v2/" {
		t.Fatalf("expected ghcr registry url, got %q", registryURL)
	}
}

func TestNormalizePanelUpdateImageNameUsesRollingDebianTag(t *testing.T) {
	got := normalizePanelUpdateImageName("linzixuanzz/daidai-panel:1.9.8-debian")
	if got != "linzixuanzz/daidai-panel:debian" {
		t.Fatalf("expected debian rolling tag, got %q", got)
	}
}

func TestNormalizePanelUpdateImageNameUsesRollingLatestTag(t *testing.T) {
	got := normalizePanelUpdateImageName("docker.io/linzixuanzz/daidai-panel:1.9.8")
	if got != "docker.io/linzixuanzz/daidai-panel:latest" {
		t.Fatalf("expected latest rolling tag, got %q", got)
	}
}

func TestNormalizePanelUpdateImageNameKeepsCustomRepo(t *testing.T) {
	got := normalizePanelUpdateImageName("ghcr.io/acme/panel:1.0.0")
	if got != "ghcr.io/acme/panel:1.0.0" {
		t.Fatalf("expected custom repo to stay unchanged, got %q", got)
	}
}

func TestFormatPanelUpdatePullErrorAddsNetworkHint(t *testing.T) {
	plan := &panelUpdatePlan{
		ImageName:     "linzixuanzz/daidai-panel:latest",
		PullImageName: "docker.1ms.run/linzixuanzz/daidai-panel:latest",
		MirrorHost:    "docker.1ms.run",
		RegistryURL:   "https://docker.1ms.run/v2/",
	}

	err := formatPanelUpdatePullError(
		plan,
		errContextDeadlineExceeded,
		[]byte(`Get "https://docker.1ms.run/v2/": context deadline exceeded (Client.Timeout exceeded while awaiting headers)`),
	)

	msg := err.Error()
	if !strings.Contains(msg, "宿主机到镜像仓库的网络或 DNS 异常") {
		t.Fatalf("expected network hint in error message, got %q", msg)
	}
	if !strings.Contains(msg, "docker.1ms.run") {
		t.Fatalf("expected mirror host in error message, got %q", msg)
	}
}

func TestCollectVolumeMappingsKeepsCustomBindPath(t *testing.T) {
	info := &dockerInspectInfo{
		HostConfig: dockerInspectHostConfig{
			Binds: []string{
				"/srv/panel-data:/app/Dumb-Panel",
			},
		},
		Mounts: []dockerInspectMount{
			{Type: "bind", Source: "/srv/panel-data", Destination: "/app/Dumb-Panel", RW: true},
			{Type: "bind", Source: "/var/run/docker.sock", Destination: "/var/run/docker.sock", RW: true},
		},
	}

	got := collectVolumeMappings(info)
	if len(got) != 2 {
		t.Fatalf("expected two distinct volume mappings, got %v", got)
	}
	if got[0] != "/srv/panel-data:/app/Dumb-Panel" {
		t.Fatalf("expected custom data bind to be preserved, got %v", got)
	}
	if got[1] != "/var/run/docker.sock:/var/run/docker.sock" {
		t.Fatalf("expected docker socket bind to be preserved, got %v", got)
	}
}

var errContextDeadlineExceeded = errors.New("context deadline exceeded")
