package handler

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"daidai-panel/service"
)

const latestPanelReleaseAPI = "https://api.github.com/repos/linzixuanzz/daidai-panel/releases/latest"

type panelReleaseInfo struct {
	TagName     string `json:"tag_name"`
	Name        string `json:"name"`
	Body        string `json:"body"`
	HTMLURL     string `json:"html_url"`
	PublishedAt string `json:"published_at"`
}

func (r panelReleaseInfo) version() string {
	return strings.TrimPrefix(strings.TrimSpace(r.TagName), "v")
}

func fetchLatestPanelRelease() (*panelReleaseInfo, error) {
	client := service.NewHTTPClient(15 * time.Second)
	resp, err := client.Get(latestPanelReleaseAPI)
	if err != nil {
		return nil, fmt.Errorf("检查更新失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("GitHub API 返回异常状态")
	}

	var release panelReleaseInfo
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("解析 GitHub 响应失败")
	}

	return &release, nil
}
