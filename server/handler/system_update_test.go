package handler

import (
	"errors"
	"slices"
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

func TestCollectVolumeMappingsPreservesNamedVolumeAlongsideBind(t *testing.T) {
	info := &dockerInspectInfo{
		HostConfig: dockerInspectHostConfig{
			Binds: []string{
				"/var/run/docker.sock:/var/run/docker.sock",
			},
		},
		Mounts: []dockerInspectMount{
			{Type: "volume", Name: "daidai_panel_data", Destination: "/app/Dumb-Panel", RW: true},
			{Type: "bind", Source: "/var/run/docker.sock", Destination: "/var/run/docker.sock", RW: true},
		},
	}

	got := collectVolumeMappings(info)
	if len(got) != 2 {
		t.Fatalf("expected both named volume and bind mount to be preserved, got %v", got)
	}

	gotSet := make(map[string]struct{}, len(got))
	for _, mapping := range got {
		gotSet[mapping] = struct{}{}
	}

	if _, exists := gotSet["daidai_panel_data:/app/Dumb-Panel"]; !exists {
		t.Fatalf("expected named data volume to be preserved, got %v", got)
	}
	if _, exists := gotSet["/var/run/docker.sock:/var/run/docker.sock"]; !exists {
		t.Fatalf("expected docker socket bind to be preserved, got %v", got)
	}
}

func TestCollectVolumeMappingsDeduplicatesEquivalentRWBindings(t *testing.T) {
	info := &dockerInspectInfo{
		HostConfig: dockerInspectHostConfig{
			Binds: []string{
				"/srv/panel-data:/app/Dumb-Panel:rw",
				"/var/run/docker.sock:/var/run/docker.sock:rw",
			},
		},
		Mounts: []dockerInspectMount{
			{Type: "bind", Source: "/srv/panel-data", Destination: "/app/Dumb-Panel", RW: true},
			{Type: "bind", Source: "/var/run/docker.sock", Destination: "/var/run/docker.sock", RW: true},
		},
	}

	got := collectVolumeMappings(info)
	if len(got) != 2 {
		t.Fatalf("expected equivalent rw bindings to be deduplicated, got %v", got)
	}

	gotSet := make(map[string]struct{}, len(got))
	for _, mapping := range got {
		gotSet[mapping] = struct{}{}
	}

	if _, exists := gotSet["/srv/panel-data:/app/Dumb-Panel:rw"]; !exists {
		t.Fatalf("expected original data bind to be preserved, got %v", got)
	}
	if _, exists := gotSet["/var/run/docker.sock:/var/run/docker.sock:rw"]; !exists {
		t.Fatalf("expected original docker socket bind to be preserved, got %v", got)
	}
}

func TestBuildContainerRunArgsPreservesCustomDataDirEnvAndMount(t *testing.T) {
	info := &dockerInspectInfo{
		HostConfig: dockerInspectHostConfig{
			Binds: []string{
				"/opt/daidai-data:/srv/custom-data",
				"/var/run/docker.sock:/var/run/docker.sock",
			},
		},
		Config: dockerInspectConfig{
			Env: []string{
				"TZ=Asia/Shanghai",
				"DATA_DIR=/srv/custom-data",
				"CONTAINER_NAME=daidai-panel",
				"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
			},
		},
		Mounts: []dockerInspectMount{
			{Type: "bind", Source: "/opt/daidai-data", Destination: "/srv/custom-data", RW: true},
			{Type: "bind", Source: "/var/run/docker.sock", Destination: "/var/run/docker.sock", RW: true},
		},
	}

	got := buildContainerRunArgs("daidai-panel", "linzixuanzz/daidai-panel:latest", info)

	if !slices.Contains(got, "/opt/daidai-data:/srv/custom-data") {
		t.Fatalf("expected custom data mount to be preserved, got %v", got)
	}
	if !slices.Contains(got, "DATA_DIR=/srv/custom-data") {
		t.Fatalf("expected custom DATA_DIR env to be preserved, got %v", got)
	}
	if slices.Contains(got, "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin") {
		t.Fatalf("expected runtime PATH env to be filtered out, got %v", got)
	}
	if got[len(got)-1] != "linzixuanzz/daidai-panel:latest" {
		t.Fatalf("expected image name to remain the final run arg, got %v", got)
	}
}

var errContextDeadlineExceeded = errors.New("context deadline exceeded")
