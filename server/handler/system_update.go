package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"time"

	"daidai-panel/model"
	"daidai-panel/pkg/response"

	"github.com/gin-gonic/gin"
)

const (
	dockerSocketPath             = "/var/run/docker.sock"
	defaultDockerHubRegistryHost = "registry-1.docker.io"
)

type panelUpdateStatusSnapshot struct {
	Status        string    `json:"status"`
	Phase         string    `json:"phase"`
	Message       string    `json:"message"`
	Error         string    `json:"error,omitempty"`
	StartedAt     time.Time `json:"started_at,omitempty"`
	UpdatedAt     time.Time `json:"updated_at"`
	ContainerName string    `json:"container_name,omitempty"`
	ImageName     string    `json:"image_name,omitempty"`
	PullImageName string    `json:"pull_image_name,omitempty"`
	MirrorHost    string    `json:"mirror_host,omitempty"`
	RegistryURL   string    `json:"registry_url,omitempty"`
}

type panelUpdateManager struct {
	mu       sync.RWMutex
	snapshot panelUpdateStatusSnapshot
}

type panelUpdatePlan struct {
	ContainerName string
	ImageName     string
	PullImageName string
	Channel       string
	MirrorHost    string
	RegistryURL   string
	RunArgs       []string
}

type dockerInspectInfo struct {
	Name       string `json:"Name"`
	Mounts     []dockerInspectMount
	Config     dockerInspectConfig     `json:"Config"`
	HostConfig dockerInspectHostConfig `json:"HostConfig"`
}

type dockerInspectConfig struct {
	Image string   `json:"Image"`
	Env   []string `json:"Env"`
}

type dockerInspectHostConfig struct {
	Binds         []string `json:"Binds"`
	ExtraHosts    []string `json:"ExtraHosts"`
	NetworkMode   string   `json:"NetworkMode"`
	RestartPolicy struct {
		Name string `json:"Name"`
	} `json:"RestartPolicy"`
	PortBindings map[string][]struct {
		HostIP   string `json:"HostIp"`
		HostPort string `json:"HostPort"`
	} `json:"PortBindings"`
}

type dockerInspectMount struct {
	Type        string `json:"Type"`
	Name        string `json:"Name"`
	Source      string `json:"Source"`
	Destination string `json:"Destination"`
	RW          bool   `json:"RW"`
}

var panelUpdater = newPanelUpdateManager()

func newPanelUpdateManager() *panelUpdateManager {
	return &panelUpdateManager{
		snapshot: panelUpdateStatusSnapshot{
			Status:    "idle",
			Phase:     "idle",
			Message:   "当前没有进行中的更新任务",
			UpdatedAt: time.Now(),
		},
	}
}

func (m *panelUpdateManager) begin(plan *panelUpdatePlan) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.snapshot.Status == "running" || m.snapshot.Status == "restarting" {
		return fmt.Errorf("已有更新任务正在进行中，请稍后查看状态")
	}

	now := time.Now()
	m.snapshot = panelUpdateStatusSnapshot{
		Status:        "running",
		Phase:         "preparing",
		Message:       "更新环境校验通过，准备检查镜像仓库并拉取最新镜像",
		StartedAt:     now,
		UpdatedAt:     now,
		ContainerName: plan.ContainerName,
		ImageName:     plan.ImageName,
		PullImageName: plan.PullImageName,
		MirrorHost:    plan.MirrorHost,
		RegistryURL:   plan.RegistryURL,
	}
	return nil
}

func (m *panelUpdateManager) setRunning(phase, message string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.snapshot.Status = "running"
	m.snapshot.Phase = phase
	m.snapshot.Message = message
	m.snapshot.Error = ""
	m.snapshot.UpdatedAt = time.Now()
}

func (m *panelUpdateManager) setRestarting(message string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.snapshot.Status = "restarting"
	m.snapshot.Phase = "restarting"
	m.snapshot.Message = message
	m.snapshot.Error = ""
	m.snapshot.UpdatedAt = time.Now()
}

func (m *panelUpdateManager) fail(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	msg := "更新失败"
	if err != nil {
		msg = err.Error()
	}

	m.snapshot.Status = "failed"
	m.snapshot.Phase = "failed"
	m.snapshot.Message = msg
	m.snapshot.Error = msg
	m.snapshot.UpdatedAt = time.Now()
}

func (m *panelUpdateManager) snapshotCopy() panelUpdateStatusSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.snapshot
}

func (h *SystemHandler) UpdateStatus(c *gin.Context) {
	response.Success(c, gin.H{"data": panelUpdater.snapshotCopy()})
}

func buildPanelUpdatePlan() (*panelUpdatePlan, error) {
	if _, err := exec.LookPath("docker"); err != nil {
		return nil, fmt.Errorf("当前运行环境未提供 Docker CLI，无法使用面板内一键更新")
	}

	if _, err := os.Stat(dockerSocketPath); err != nil {
		return nil, fmt.Errorf("未检测到 %s，请在部署时挂载 Docker Socket 后再使用一键更新", dockerSocketPath)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if output, err := dockerCommandOutput(ctx, "info"); err != nil {
		return nil, formatDockerCommandError("无法连接 Docker 守护进程，请确认 docker.sock 可访问", err, output)
	}

	info, err := inspectCurrentPanelContainer()
	if err != nil {
		return nil, err
	}

	containerName := strings.TrimPrefix(strings.TrimSpace(info.Name), "/")
	if envName := strings.TrimSpace(os.Getenv("CONTAINER_NAME")); envName != "" {
		containerName = envName
	}
	if containerName == "" {
		return nil, fmt.Errorf("无法识别当前面板容器名称，请设置环境变量 CONTAINER_NAME")
	}

	imageName := strings.TrimSpace(os.Getenv("IMAGE_NAME"))
	if imageName == "" {
		imageName = strings.TrimSpace(info.Config.Image)
	}
	if imageName == "" {
		return nil, fmt.Errorf("无法识别当前容器镜像，请设置环境变量 IMAGE_NAME")
	}
	imageName = normalizePanelUpdateImageName(imageName)

	pullImageName, mirrorHost, registryURL := resolveUpdateImageTarget(
		imageName,
		model.GetRegisteredConfig("update_image_mirror"),
	)

	return &panelUpdatePlan{
		ContainerName: containerName,
		ImageName:     imageName,
		PullImageName: pullImageName,
		Channel:       resolvePanelUpdateChannel(imageName),
		MirrorHost:    mirrorHost,
		RegistryURL:   registryURL,
		RunArgs:       buildContainerRunArgs(containerName, imageName, info),
	}, nil
}

func inspectCurrentPanelContainer() (*dockerInspectInfo, error) {
	candidates := uniqueNonEmptyStrings(
		os.Getenv("CONTAINER_NAME"),
		os.Getenv("HOSTNAME"),
		mustHostname(),
		"daidai-panel",
	)

	for _, candidate := range candidates {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		output, err := dockerCommandOutput(ctx, "inspect", "--format", "{{json .}}", candidate)
		cancel()
		if err != nil {
			continue
		}

		var info dockerInspectInfo
		if err := json.Unmarshal(output, &info); err != nil {
			continue
		}
		if strings.TrimSpace(info.Name) == "" {
			continue
		}
		return &info, nil
	}

	return nil, fmt.Errorf("无法识别当前面板容器，请设置环境变量 CONTAINER_NAME 后重试")
}

func buildContainerRunArgs(containerName, imageName string, info *dockerInspectInfo) []string {
	runArgs := []string{"run", "-d", "--name", containerName}

	restartPolicy := strings.TrimSpace(info.HostConfig.RestartPolicy.Name)
	if restartPolicy != "" && restartPolicy != "no" {
		runArgs = append(runArgs, "--restart", restartPolicy)
	}

	networkMode := strings.TrimSpace(info.HostConfig.NetworkMode)
	if networkMode != "" && networkMode != "default" {
		runArgs = append(runArgs, "--network", networkMode)
	}

	extraHosts := make([]string, 0, len(info.HostConfig.ExtraHosts))
	for _, item := range info.HostConfig.ExtraHosts {
		item = strings.TrimSpace(item)
		if item != "" {
			extraHosts = append(extraHosts, item)
		}
	}
	sort.Strings(extraHosts)
	for _, item := range extraHosts {
		runArgs = append(runArgs, "--add-host", item)
	}

	for _, mapping := range collectPortMappings(info.HostConfig.PortBindings) {
		runArgs = append(runArgs, "-p", mapping)
	}

	for _, volume := range collectVolumeMappings(info) {
		runArgs = append(runArgs, "-v", volume)
	}

	for _, env := range filterContainerEnv(info.Config.Env) {
		runArgs = append(runArgs, "-e", env)
	}

	runArgs = append(runArgs, imageName)
	return runArgs
}

func collectPortMappings(portBindings map[string][]struct {
	HostIP   string `json:"HostIp"`
	HostPort string `json:"HostPort"`
}) []string {
	keys := make([]string, 0, len(portBindings))
	for port := range portBindings {
		keys = append(keys, port)
	}
	sort.Strings(keys)

	var result []string
	for _, port := range keys {
		bindings := portBindings[port]
		for _, binding := range bindings {
			if strings.TrimSpace(binding.HostPort) == "" {
				continue
			}

			containerPort := strings.Split(port, "/")[0]
			mapping := binding.HostPort + ":" + containerPort
			hostIP := strings.TrimSpace(binding.HostIP)
			if hostIP != "" && hostIP != "0.0.0.0" && hostIP != "::" {
				mapping = hostIP + ":" + mapping
			}
			result = append(result, mapping)
		}
	}
	return result
}

func collectVolumeMappings(info *dockerInspectInfo) []string {
	if len(info.HostConfig.Binds) > 0 {
		result := make([]string, 0, len(info.HostConfig.Binds))
		for _, bind := range info.HostConfig.Binds {
			bind = strings.TrimSpace(bind)
			if bind != "" {
				result = append(result, bind)
			}
		}
		return result
	}

	var result []string
	for _, mount := range info.Mounts {
		destination := strings.TrimSpace(mount.Destination)
		if destination == "" {
			continue
		}

		var source string
		switch mount.Type {
		case "bind":
			source = strings.TrimSpace(mount.Source)
		case "volume":
			source = strings.TrimSpace(mount.Name)
			if source == "" {
				source = strings.TrimSpace(mount.Source)
			}
		default:
			continue
		}

		if source == "" {
			continue
		}

		mapping := source + ":" + destination
		if !mount.RW {
			mapping += ":ro"
		}
		result = append(result, mapping)
	}

	sort.Strings(result)
	return result
}

func filterContainerEnv(envList []string) []string {
	skipPrefixes := []string{
		"PATH=",
		"HOME=",
		"HOSTNAME=",
		"LANG=",
		"LC_",
		"TERM=",
		"PWD=",
		"SHLVL=",
		"_=",
	}

	result := make([]string, 0, len(envList))
	for _, env := range envList {
		env = strings.TrimSpace(env)
		if env == "" {
			continue
		}

		skip := false
		for _, prefix := range skipPrefixes {
			if strings.HasPrefix(env, prefix) {
				skip = true
				break
			}
		}
		if !skip {
			result = append(result, env)
		}
	}

	return result
}

func executePanelUpdate(plan *panelUpdatePlan) {
	executePanelUpdateWithOptions(plan, panelUpdateExecutionOptions{})
}

func executePanelUpdateWithOptions(plan *panelUpdatePlan, options panelUpdateExecutionOptions) {
	panelUpdater.setRunning("preparing", fmt.Sprintf("正在检查镜像仓库连通性 %s", plan.RegistryURL))
	if err := preflightUpdateRegistry(plan); err != nil {
		panelUpdater.fail(err)
		if options.AutoUpdate {
			notifyAutoUpdateFailure(options.TargetVersion, err)
		}
		return
	}

	panelUpdater.setRunning("pulling", fmt.Sprintf("正在拉取最新镜像 %s", plan.PullImageName))

	pullCtx, pullCancel := context.WithTimeout(context.Background(), 20*time.Minute)
	pullOutput, err := dockerCommandOutput(pullCtx, "pull", plan.PullImageName)
	pullCancel()
	if err != nil {
		formatted := formatPanelUpdatePullError(plan, err, pullOutput)
		panelUpdater.fail(formatted)
		if options.AutoUpdate {
			notifyAutoUpdateFailure(options.TargetVersion, formatted)
		}
		return
	}

	if plan.PullImageName != "" && plan.PullImageName != plan.ImageName {
		panelUpdater.setRunning("pulling", fmt.Sprintf("镜像已拉取完成，正在同步更新标签 %s", plan.ImageName))

		tagCtx, tagCancel := context.WithTimeout(context.Background(), time.Minute)
		tagOutput, tagErr := dockerCommandOutput(tagCtx, "tag", plan.PullImageName, plan.ImageName)
		tagCancel()
		if tagErr != nil {
			formatted := formatDockerCommandError("同步更新镜像标签失败", tagErr, tagOutput)
			panelUpdater.fail(formatted)
			if options.AutoUpdate {
				notifyAutoUpdateFailure(options.TargetVersion, formatted)
			}
			return
		}
	}

	panelUpdater.setRunning("scheduling", "镜像已拉取完成，正在启动更新辅助容器")

	helperScript := buildPanelUpdateHelperScript(plan)
	helperArgs := []string{
		"run", "-d", "--rm",
		"-v", dockerSocketPath + ":" + dockerSocketPath,
		"--entrypoint", "sh",
		plan.ImageName,
		"-c", helperScript,
	}

	helperCtx, helperCancel := context.WithTimeout(context.Background(), time.Minute)
	helperOutput, err := dockerCommandOutput(helperCtx, helperArgs...)
	helperCancel()
	if err != nil {
		formatted := formatDockerCommandError("启动更新辅助容器失败", err, helperOutput)
		panelUpdater.fail(formatted)
		if options.AutoUpdate {
			notifyAutoUpdateFailure(options.TargetVersion, formatted)
		}
		return
	}

	panelUpdater.setRestarting("更新任务已启动，正在重建面板容器并切换到新版本")
}

func buildPanelUpdateHelperScript(plan *panelUpdatePlan) string {
	quotedArgs := make([]string, 0, len(plan.RunArgs))
	for _, arg := range plan.RunArgs {
		quotedArgs = append(quotedArgs, shellQuote(arg))
	}

	return fmt.Sprintf(
		"sleep 2 && docker rm -f %s >/dev/null 2>&1 || true && docker %s",
		shellQuote(plan.ContainerName),
		strings.Join(quotedArgs, " "),
	)
}

func dockerCommandOutput(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "docker", args...)
	return cmd.CombinedOutput()
}

func preflightUpdateRegistry(plan *panelUpdatePlan) error {
	registryURL := strings.TrimSpace(plan.RegistryURL)
	if registryURL == "" {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	client := &http.Client{
		Timeout:   12 * time.Second,
		Transport: transport,
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, registryURL, nil)
	if err != nil {
		return fmt.Errorf("更新前镜像仓库连通性检查失败：%w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("更新前镜像仓库连通性检查失败：%w。%s", err, buildPanelUpdateNetworkHint(plan))
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusInternalServerError {
		return fmt.Errorf("更新前镜像仓库连通性检查失败：镜像仓库返回状态 %d。%s", resp.StatusCode, buildPanelUpdateNetworkHint(plan))
	}

	return nil
}

func formatDockerCommandError(prefix string, err error, output []byte) error {
	detail := trimCommandOutput(output)
	switch {
	case detail != "":
		return fmt.Errorf("%s: %s", prefix, detail)
	case err != nil:
		return fmt.Errorf("%s: %v", prefix, err)
	default:
		return fmt.Errorf("%s", prefix)
	}
}

func formatPanelUpdatePullError(plan *panelUpdatePlan, err error, output []byte) error {
	base := formatDockerCommandError("拉取最新镜像失败", err, output)
	hint := buildPanelUpdatePullHint(plan, err, output)
	if hint == "" {
		return base
	}
	return fmt.Errorf("%s。%s", strings.TrimSpace(base.Error()), hint)
}

func trimCommandOutput(output []byte) string {
	text := strings.TrimSpace(string(output))
	if text == "" {
		return ""
	}

	lines := strings.Split(text, "\n")
	if len(lines) > 6 {
		lines = lines[len(lines)-6:]
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func buildPanelUpdatePullHint(plan *panelUpdatePlan, err error, output []byte) string {
	lower := strings.ToLower(strings.TrimSpace(string(output)))
	if err != nil {
		lower += "\n" + strings.ToLower(err.Error())
	}

	if strings.Contains(lower, "context deadline exceeded") ||
		strings.Contains(lower, "client.timeout exceeded while awaiting headers") ||
		strings.Contains(lower, "i/o timeout") ||
		strings.Contains(lower, "tls handshake timeout") ||
		strings.Contains(lower, "no such host") ||
		strings.Contains(lower, "temporary failure in name resolution") {
		return fmt.Sprintf("这通常是宿主机到镜像仓库的网络或 DNS 异常。目标仓库：%s。%s", plan.RegistryURL, buildPanelUpdateNetworkHint(plan))
	}

	return ""
}

func buildPanelUpdateNetworkHint(plan *panelUpdatePlan) string {
	if strings.TrimSpace(plan.MirrorHost) != "" {
		return fmt.Sprintf("当前系统更新镜像源为 %s，请检查该镜像源是否可访问；如需恢复直连，可在“系统设置 / 网络代理”中清空系统更新镜像源。", plan.MirrorHost)
	}
	return "当前将直连 Docker Hub；如宿主机访问 Docker Hub 较慢，可在“系统设置 / 网络代理”中填写系统更新镜像源，例如 docker.1ms.run。"
}

func uniqueNonEmptyStrings(values ...string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func resolveUpdateImageTarget(imageName, mirrorHost string) (pullImageName, resolvedMirrorHost, registryURL string) {
	imageName = strings.TrimSpace(imageName)
	mirrorHost = strings.TrimSpace(mirrorHost)
	registryHost, repoRef := splitImageRegistry(imageName)

	if mirrorHost != "" && (registryHost == defaultDockerHubRegistryHost || registryHost == mirrorHost) {
		if registryHost == mirrorHost {
			return imageName, mirrorHost, buildRegistryEndpoint(mirrorHost)
		}
		return mirrorHost + "/" + repoRef, mirrorHost, buildRegistryEndpoint(mirrorHost)
	}

	return imageName, "", buildRegistryEndpoint(registryHost)
}

func splitImageRegistry(imageName string) (registryHost, repoRef string) {
	imageName = strings.TrimSpace(imageName)
	if imageName == "" {
		return defaultDockerHubRegistryHost, ""
	}

	parts := strings.Split(imageName, "/")
	if len(parts) <= 1 || !isExplicitRegistryHost(parts[0]) {
		return defaultDockerHubRegistryHost, imageName
	}

	registryHost = strings.ToLower(strings.TrimSpace(parts[0]))
	switch registryHost {
	case "docker.io", "index.docker.io":
		registryHost = defaultDockerHubRegistryHost
	}
	return registryHost, strings.Join(parts[1:], "/")
}

func isExplicitRegistryHost(segment string) bool {
	segment = strings.ToLower(strings.TrimSpace(segment))
	return strings.Contains(segment, ".") || strings.Contains(segment, ":") || segment == "localhost"
}

func buildRegistryEndpoint(host string) string {
	host = strings.TrimSpace(host)
	if host == "" {
		host = defaultDockerHubRegistryHost
	}
	return "https://" + host + "/v2/"
}

func mustHostname() string {
	hostname, err := os.Hostname()
	if err != nil {
		return ""
	}
	return hostname
}

func respondUpdateConflict(c *gin.Context, message string) {
	response.Error(c, http.StatusConflict, message)
}

func normalizePanelUpdateImageName(imageName string) string {
	baseImage, tag, _ := splitImageTag(strings.TrimSpace(imageName))
	_, repoRef := splitImageRegistry(baseImage)
	if repoRef != "linzixuanzz/daidai-panel" {
		return strings.TrimSpace(imageName)
	}

	channel := resolvePanelUpdateChannelFromTag(tag)
	return baseImage + ":" + channel
}

func resolvePanelUpdateChannel(imageName string) string {
	_, tag, _ := splitImageTag(strings.TrimSpace(imageName))
	return resolvePanelUpdateChannelFromTag(tag)
}

func resolvePanelUpdateChannelFromTag(tag string) string {
	tag = strings.ToLower(strings.TrimSpace(tag))
	if tag == "debian" || strings.HasSuffix(tag, "-debian") {
		return "debian"
	}
	return "latest"
}

func splitImageTag(imageName string) (base string, tag string, hasTag bool) {
	imageName = strings.TrimSpace(imageName)
	if imageName == "" {
		return "", "", false
	}

	if digestIdx := strings.Index(imageName, "@"); digestIdx >= 0 {
		imageName = imageName[:digestIdx]
	}

	lastSlash := strings.LastIndex(imageName, "/")
	lastColon := strings.LastIndex(imageName, ":")
	if lastColon > lastSlash {
		return imageName[:lastColon], imageName[lastColon+1:], true
	}

	return imageName, "", false
}
