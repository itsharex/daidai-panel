package service

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"daidai-panel/config"
	"daidai-panel/database"
	"daidai-panel/middleware"
	"daidai-panel/model"

	"gorm.io/gorm"
)

type LegacyBackupData struct {
	Version   string                `json:"version"`
	CreatedAt time.Time             `json:"created_at"`
	Tasks     []model.Task          `json:"tasks"`
	EnvVars   []model.EnvVar        `json:"env_vars"`
	Subs      []model.Subscription  `json:"subscriptions"`
	Channels  []model.NotifyChannel `json:"notify_channels"`
	SSHKeys   []model.SSHKey        `json:"ssh_keys"`
	Configs   []model.SystemConfig  `json:"system_configs"`
	Scripts   []ScriptFile          `json:"scripts,omitempty"`
	Deps      []model.Dependency    `json:"dependencies,omitempty"`
}

func createBackupArchive(options BackupCreateOptions) (string, error) {
	selection := options.Selection.NormalizeDefaults()
	if !selection.Any() {
		return "", fmt.Errorf("至少选择一个备份项")
	}

	manifest, err := buildBackupManifest(selection)
	if err != nil {
		return "", err
	}

	archiveData, err := buildBackupArchive(manifest)
	if err != nil {
		return "", err
	}

	backupDir := filepath.Join(config.C.Data.Dir, "backups")
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create backup dir: %w", err)
	}

	var (
		filename  string
		finalData []byte
	)

	if strings.TrimSpace(options.Password) != "" {
		finalData, err = encryptData(archiveData, options.Password)
		if err != nil {
			return "", fmt.Errorf("failed to encrypt backup: %w", err)
		}
		filename = fmt.Sprintf("backup_%s.enc", time.Now().Format("20060102_150405"))
	} else {
		finalData = archiveData
		filename = fmt.Sprintf("backup_%s.tgz", time.Now().Format("20060102_150405"))
	}

	filePath := filepath.Join(backupDir, filename)
	if err := os.WriteFile(filePath, finalData, 0o644); err != nil {
		return "", fmt.Errorf("failed to write backup: %w", err)
	}

	return filePath, nil
}

func buildBackupManifest(selection BackupSelection) (BackupManifest, error) {
	manifest := BackupManifest{
		Format:    "daidai-panel-backup",
		Version:   "0.4.0",
		Source:    "daidai-panel",
		CreatedAt: time.Now(),
		Selection: selection,
	}

	if selection.Configs {
		cfgBundle, err := snapshotConfigBundle()
		if err != nil {
			return BackupManifest{}, err
		}
		manifest.Data.Configs = cfgBundle
	}

	if selection.Tasks {
		if err := database.DB.Order("id ASC").Find(&manifest.Data.Tasks).Error; err != nil {
			return BackupManifest{}, fmt.Errorf("load tasks: %w", err)
		}
	}

	if selection.EnvVars {
		if err := database.DB.Order("position ASC, id ASC").Find(&manifest.Data.EnvVars).Error; err != nil {
			return BackupManifest{}, fmt.Errorf("load env vars: %w", err)
		}
	}

	if selection.Subscriptions {
		if err := database.DB.Order("id ASC").Find(&manifest.Data.Subscriptions).Error; err != nil {
			return BackupManifest{}, fmt.Errorf("load subscriptions: %w", err)
		}

		var sshKeys []model.SSHKey
		if err := database.DB.Order("id ASC").Find(&sshKeys).Error; err != nil {
			return BackupManifest{}, fmt.Errorf("load ssh keys: %w", err)
		}
		for _, key := range sshKeys {
			manifest.Data.SSHKeys = append(manifest.Data.SSHKeys, BackupSSHKey{
				ID:         key.ID,
				Name:       key.Name,
				PrivateKey: key.PrivateKey,
				CreatedAt:  key.CreatedAt,
				UpdatedAt:  key.UpdatedAt,
			})
		}
	}

	if selection.Dependencies {
		var deps []model.Dependency
		if err := database.DB.Order("id ASC").Find(&deps).Error; err != nil {
			return BackupManifest{}, fmt.Errorf("load dependencies: %w", err)
		}
		for _, dep := range deps {
			manifest.Data.Dependencies = append(manifest.Data.Dependencies, BackupDependency{
				Type: dep.Type,
				Name: dep.Name,
			})
		}
	}

	if selection.Logs {
		var taskLogs []model.TaskLog
		if err := database.DB.Preload("Task").Order("id ASC").Find(&taskLogs).Error; err != nil {
			return BackupManifest{}, fmt.Errorf("load task logs: %w", err)
		}
		for _, logItem := range taskLogs {
			taskName := ""
			if logItem.Task != nil {
				taskName = logItem.Task.Name
			}
			manifest.Data.TaskLogs = append(manifest.Data.TaskLogs, BackupTaskLog{
				TaskID:    logItem.TaskID,
				TaskName:  taskName,
				Content:   logItem.Content,
				Status:    logItem.Status,
				Duration:  logItem.Duration,
				LogPath:   logItem.LogPath,
				StartedAt: logItem.StartedAt,
				EndedAt:   logItem.EndedAt,
				CreatedAt: logItem.CreatedAt,
				UpdatedAt: logItem.UpdatedAt,
			})
		}
	}

	return manifest, nil
}

func snapshotConfigBundle() (BackupConfigBundle, error) {
	bundle := BackupConfigBundle{}

	if err := database.DB.Order("key ASC").Find(&bundle.SystemConfigs).Error; err != nil {
		return BackupConfigBundle{}, fmt.Errorf("load system configs: %w", err)
	}

	var apps []model.OpenApp
	if err := database.DB.Order("id ASC").Find(&apps).Error; err != nil {
		return BackupConfigBundle{}, fmt.Errorf("load open apps: %w", err)
	}
	for _, app := range apps {
		bundle.OpenApps = append(bundle.OpenApps, BackupOpenApp{
			ID:        app.ID,
			Name:      app.Name,
			AppKey:    app.AppKey,
			AppSecret: app.AppSecret,
			Scopes:    app.Scopes,
			Enabled:   app.Enabled,
			RateLimit: app.RateLimit,
			CallCount: app.CallCount,
			CreatedAt: app.CreatedAt,
			UpdatedAt: app.UpdatedAt,
		})
	}

	var channels []model.NotifyChannel
	if err := database.DB.Order("id ASC").Find(&channels).Error; err != nil {
		return BackupConfigBundle{}, fmt.Errorf("load notify channels: %w", err)
	}
	for _, channel := range channels {
		bundle.NotifyChannels = append(bundle.NotifyChannels, BackupNotifyChannel{
			ID:        channel.ID,
			Name:      channel.Name,
			Type:      channel.Type,
			Config:    channel.Config,
			Enabled:   channel.Enabled,
			CreatedAt: channel.CreatedAt,
			UpdatedAt: channel.UpdatedAt,
		})
	}

	var users []model.User
	if err := database.DB.Order("id ASC").Find(&users).Error; err != nil {
		return BackupConfigBundle{}, fmt.Errorf("load users: %w", err)
	}
	for _, user := range users {
		bundle.Users = append(bundle.Users, BackupUser{
			ID:           user.ID,
			Username:     user.Username,
			PasswordHash: user.Password,
			Role:         user.Role,
			Enabled:      user.Enabled,
			LastLoginAt:  user.LastLoginAt,
			CreatedAt:    user.CreatedAt,
			UpdatedAt:    user.UpdatedAt,
		})
	}

	if err := database.DB.Order("id ASC").Find(&bundle.IPWhitelists).Error; err != nil {
		return BackupConfigBundle{}, fmt.Errorf("load ip whitelists: %w", err)
	}

	var twoFactor []model.TwoFactorAuth
	if err := database.DB.Order("id ASC").Find(&twoFactor).Error; err != nil {
		return BackupConfigBundle{}, fmt.Errorf("load two factor auths: %w", err)
	}
	for _, item := range twoFactor {
		bundle.TwoFactorAuths = append(bundle.TwoFactorAuths, BackupTwoFactorAuth{
			UserID:    item.UserID,
			Secret:    item.Secret,
			Enabled:   item.Enabled,
			CreatedAt: item.CreatedAt,
			UpdatedAt: item.UpdatedAt,
		})
	}

	return bundle, nil
}

func buildBackupArchive(manifest BackupManifest) ([]byte, error) {
	var raw bytes.Buffer
	gzipWriter := gzip.NewWriter(&raw)
	tarWriter := tar.NewWriter(gzipWriter)

	manifestData, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal manifest: %w", err)
	}
	if err := writeTarBytes(tarWriter, "manifest.json", manifestData, 0o644, manifest.CreatedAt); err != nil {
		return nil, err
	}

	if manifest.Selection.Scripts {
		if err := addDirectoryToTar(tarWriter, config.C.Data.ScriptsDir, "files/scripts"); err != nil {
			return nil, err
		}
	}

	if manifest.Selection.Logs {
		if err := addDirectoryToTar(tarWriter, config.C.Data.LogDir, "files/logs"); err != nil {
			return nil, err
		}
		panelLogPath := filepath.Join(config.C.Data.Dir, "panel.log")
		if _, err := os.Stat(panelLogPath); err == nil {
			if err := addFileToTar(tarWriter, panelLogPath, "files/panel.log"); err != nil {
				return nil, err
			}
		}
	}

	if err := tarWriter.Close(); err != nil {
		return nil, fmt.Errorf("close tar writer: %w", err)
	}
	if err := gzipWriter.Close(); err != nil {
		return nil, fmt.Errorf("close gzip writer: %w", err)
	}

	return raw.Bytes(), nil
}

func writeTarBytes(tw *tar.Writer, name string, data []byte, mode int64, modTime time.Time) error {
	header := &tar.Header{
		Name:    filepath.ToSlash(name),
		Mode:    mode,
		Size:    int64(len(data)),
		ModTime: modTime,
	}
	if err := tw.WriteHeader(header); err != nil {
		return fmt.Errorf("write tar header %s: %w", name, err)
	}
	if _, err := tw.Write(data); err != nil {
		return fmt.Errorf("write tar body %s: %w", name, err)
	}
	return nil
}

func addFileToTar(tw *tar.Writer, sourcePath, archivePath string) error {
	info, err := os.Stat(sourcePath)
	if err != nil || info.IsDir() {
		return nil
	}

	file, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("open %s: %w", sourcePath, err)
	}
	defer file.Close()

	header, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return fmt.Errorf("build tar header %s: %w", sourcePath, err)
	}
	header.Name = filepath.ToSlash(archivePath)

	if err := tw.WriteHeader(header); err != nil {
		return fmt.Errorf("write tar header %s: %w", sourcePath, err)
	}
	if _, err := io.Copy(tw, file); err != nil {
		return fmt.Errorf("copy tar file %s: %w", sourcePath, err)
	}
	return nil
}

func addDirectoryToTar(tw *tar.Writer, sourceDir, archiveRoot string) error {
	if _, err := os.Stat(sourceDir); os.IsNotExist(err) {
		return nil
	}

	return filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}
		return addFileToTar(tw, path, filepath.Join(archiveRoot, relPath))
	})
}

func restoreBackupFile(filename, password string) error {
	backupDir := filepath.Join(config.C.Data.Dir, "backups")
	filePath := filepath.Join(backupDir, filepath.Base(filename))

	fileData, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read backup: %w", err)
	}

	rawData := fileData
	if strings.HasSuffix(strings.ToLower(filename), ".enc") {
		if strings.TrimSpace(password) == "" {
			return fmt.Errorf("加密备份需要密码")
		}
		rawData, err = decryptData(fileData, password)
		if err != nil {
			return fmt.Errorf("failed to decrypt backup: %w", err)
		}
	}

	if looksLikeGzip(rawData) {
		return restoreArchiveBytes(rawData)
	}
	if looksLikeJSON(rawData) {
		return restoreLegacyJSONBytes(rawData)
	}

	return fmt.Errorf("无法识别的备份格式")
}

func looksLikeGzip(data []byte) bool {
	return len(data) >= 2 && data[0] == 0x1f && data[1] == 0x8b
}

func looksLikeJSON(data []byte) bool {
	trimmed := strings.TrimSpace(string(data))
	return strings.HasPrefix(trimmed, "{")
}

func restoreArchiveBytes(data []byte) error {
	tempDir, err := os.MkdirTemp("", "daidai-restore-*")
	if err != nil {
		return fmt.Errorf("创建临时目录失败: %w", err)
	}
	defer os.RemoveAll(tempDir)

	if err := extractTarGzBytes(data, tempDir); err != nil {
		return err
	}

	manifestPath := filepath.Join(tempDir, "manifest.json")
	if _, err := os.Stat(manifestPath); err == nil {
		var manifest BackupManifest
		manifestData, err := os.ReadFile(manifestPath)
		if err != nil {
			return fmt.Errorf("读取备份清单失败: %w", err)
		}
		if err := json.Unmarshal(manifestData, &manifest); err != nil {
			return fmt.Errorf("解析备份清单失败: %w", err)
		}
		return restoreBackupManifest(manifest, tempDir)
	}

	manifest, err := buildQingLongManifest(tempDir)
	if err != nil {
		return err
	}
	return restoreBackupManifest(manifest, tempDir)
}

func restoreLegacyJSONBytes(data []byte) error {
	var legacy LegacyBackupData
	if err := json.Unmarshal(data, &legacy); err != nil {
		return fmt.Errorf("failed to parse backup: %w", err)
	}

	tempDir, err := os.MkdirTemp("", "daidai-legacy-restore-*")
	if err != nil {
		return fmt.Errorf("创建旧备份临时目录失败: %w", err)
	}
	defer os.RemoveAll(tempDir)

	scriptsDir := filepath.Join(tempDir, "files", "scripts")
	for _, scriptFile := range legacy.Scripts {
		if strings.Contains(scriptFile.Path, "..") {
			continue
		}
		content, err := base64.StdEncoding.DecodeString(scriptFile.Content)
		if err != nil {
			continue
		}
		targetPath := filepath.Join(scriptsDir, filepath.FromSlash(scriptFile.Path))
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return fmt.Errorf("创建旧脚本目录失败: %w", err)
		}
		if err := os.WriteFile(targetPath, content, 0o755); err != nil {
			return fmt.Errorf("写入旧脚本失败: %w", err)
		}
	}

	manifest := BackupManifest{
		Format:    "daidai-panel-backup",
		Version:   legacy.Version,
		Source:    "daidai-panel",
		CreatedAt: legacy.CreatedAt,
		Selection: BackupSelection{
			Configs:       true,
			Tasks:         true,
			Subscriptions: true,
			EnvVars:       true,
			Logs:          false,
			Scripts:       len(legacy.Scripts) > 0,
			Dependencies:  len(legacy.Deps) > 0,
		},
		Data: BackupPayload{
			Configs: BackupConfigBundle{
				SystemConfigs: legacy.Configs,
			},
			Tasks:         legacy.Tasks,
			EnvVars:       legacy.EnvVars,
			Subscriptions: legacy.Subs,
		},
	}

	for _, channel := range legacy.Channels {
		manifest.Data.Configs.NotifyChannels = append(manifest.Data.Configs.NotifyChannels, BackupNotifyChannel{
			ID:        channel.ID,
			Name:      channel.Name,
			Type:      channel.Type,
			Config:    channel.Config,
			Enabled:   channel.Enabled,
			CreatedAt: channel.CreatedAt,
			UpdatedAt: channel.UpdatedAt,
		})
	}

	for _, key := range legacy.SSHKeys {
		manifest.Data.SSHKeys = append(manifest.Data.SSHKeys, BackupSSHKey{
			ID:         key.ID,
			Name:       key.Name,
			PrivateKey: key.PrivateKey,
			CreatedAt:  key.CreatedAt,
			UpdatedAt:  key.UpdatedAt,
		})
	}

	for _, dep := range legacy.Deps {
		manifest.Data.Dependencies = append(manifest.Data.Dependencies, BackupDependency{
			Type: dep.Type,
			Name: dep.Name,
		})
	}

	return restoreBackupManifest(manifest, tempDir)
}

func extractTarGzBytes(data []byte, targetDir string) error {
	gzipReader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("打开备份压缩包失败: %w", err)
	}
	defer gzipReader.Close()

	tarReader := tar.NewReader(gzipReader)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("读取备份压缩内容失败: %w", err)
		}

		targetPath := filepath.Join(targetDir, filepath.FromSlash(header.Name))
		cleanTarget := filepath.Clean(targetPath)
		if !strings.HasPrefix(cleanTarget, filepath.Clean(targetDir)+string(os.PathSeparator)) && cleanTarget != filepath.Clean(targetDir) {
			return fmt.Errorf("备份包含非法路径: %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(cleanTarget, 0o755); err != nil {
				return fmt.Errorf("创建目录失败: %w", err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(cleanTarget), 0o755); err != nil {
				return fmt.Errorf("创建文件目录失败: %w", err)
			}
			file, err := os.OpenFile(cleanTarget, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(header.Mode))
			if err != nil {
				return fmt.Errorf("创建文件失败: %w", err)
			}
			if _, err := io.Copy(file, tarReader); err != nil {
				file.Close()
				return fmt.Errorf("写入文件失败: %w", err)
			}
			file.Close()
		}
	}

	return nil
}

func restoreBackupManifest(manifest BackupManifest, extractedDir string) error {
	selection := manifest.Selection
	if !selection.Any() {
		return fmt.Errorf("备份内容为空")
	}

	tx := database.DB.Begin()
	if tx.Error != nil {
		return tx.Error
	}

	var createdDependencies []model.Dependency
	taskIDMap := map[uint]uint{}
	sshKeyIDMap := map[uint]uint{}

	rollback := func(err error) error {
		tx.Rollback()
		return err
	}

	if selection.Logs || selection.Tasks {
		if err := deleteAll(tx, "task_logs"); err != nil {
			return rollback(err)
		}
	}
	if selection.Tasks {
		if err := deleteAll(tx, "tasks"); err != nil {
			return rollback(err)
		}
	}
	if selection.Subscriptions {
		for _, table := range []string{"sub_logs", "subscriptions", "ssh_keys"} {
			if err := deleteAll(tx, table); err != nil {
				return rollback(err)
			}
		}
	}
	if selection.EnvVars {
		if err := deleteAll(tx, "env_vars"); err != nil {
			return rollback(err)
		}
	}
	if selection.Configs {
		for _, table := range []string{
			"api_call_logs",
			"open_apps",
			"notify_channels",
			"ip_whitelists",
			"system_configs",
		} {
			if err := deleteAll(tx, table); err != nil {
				return rollback(err)
			}
		}
	}
	if selection.Dependencies {
		if err := deleteAll(tx, "dependencies"); err != nil {
			return rollback(err)
		}
	}

	notifyChannelIDMap := map[uint]uint{}

	if selection.Configs {
		if err := restoreSystemConfigs(tx, manifest.Data.Configs.SystemConfigs); err != nil {
			return rollback(err)
		}
		var err error
		notifyChannelIDMap, err = restoreNotifyChannels(tx, manifest.Data.Configs.NotifyChannels)
		if err != nil {
			return rollback(err)
		}
		if err := restoreOpenApps(tx, manifest.Data.Configs.OpenApps); err != nil {
			return rollback(err)
		}
		if err := restoreIPWhitelists(tx, manifest.Data.Configs.IPWhitelists); err != nil {
			return rollback(err)
		}
	}

	if selection.EnvVars {
		if err := restoreEnvVars(tx, manifest.Data.EnvVars); err != nil {
			return rollback(err)
		}
	}

	if selection.Tasks {
		var err error
		taskIDMap, err = restoreTasks(tx, manifest.Data.Tasks, notifyChannelIDMap)
		if err != nil {
			return rollback(err)
		}
	}

	if selection.Subscriptions {
		var err error
		sshKeyIDMap, err = restoreSSHKeys(tx, manifest.Data.SSHKeys)
		if err != nil {
			return rollback(err)
		}
		if err := restoreSubscriptions(tx, manifest.Data.Subscriptions, sshKeyIDMap); err != nil {
			return rollback(err)
		}
	}

	if selection.Logs {
		if err := restoreTaskLogs(tx, manifest.Data.TaskLogs, taskIDMap); err != nil {
			return rollback(err)
		}
	}

	if selection.Dependencies {
		var err error
		createdDependencies, err = restoreDependencies(tx, manifest.Data.Dependencies)
		if err != nil {
			return rollback(err)
		}
	}

	if err := tx.Commit().Error; err != nil {
		return err
	}

	if selection.Scripts {
		if err := restoreScriptFiles(extractedDir, manifest.Source); err != nil {
			return err
		}
	}
	if selection.Logs {
		if err := restoreLogFiles(extractedDir, manifest.Source); err != nil {
			return err
		}
	}

	model.InitDefaultConfigs()
	_ = middleware.ConfigureTrustedProxyCIDRs(model.GetRegisteredConfig("trusted_proxy_cidrs"))

	if scheduler := GetSchedulerV2(); scheduler != nil {
		scheduler.ReloadAllJobs()
	}
	if subScheduler := GetSubscriptionScheduler(); subScheduler != nil {
		subScheduler.ReloadAllJobs()
	}
	if selection.Dependencies {
		dependencyReinstallBatchFunc(createdDependencies)
	}

	return nil
}

func deleteAll(tx *gorm.DB, table string) error {
	return tx.Exec("DELETE FROM " + table).Error
}

func restoreSystemConfigs(tx *gorm.DB, configs []model.SystemConfig) error {
	for _, item := range configs {
		item.ID = 0
		if err := tx.Create(&item).Error; err != nil {
			return err
		}
	}
	return nil
}

func restoreUsers(tx *gorm.DB, users []BackupUser) (map[uint]uint, error) {
	idMap := make(map[uint]uint, len(users))
	for _, item := range users {
		user := model.User{
			Username:    item.Username,
			Password:    item.PasswordHash,
			Role:        item.Role,
			Enabled:     item.Enabled,
			LastLoginAt: item.LastLoginAt,
			CreatedAt:   item.CreatedAt,
			UpdatedAt:   item.UpdatedAt,
		}
		if err := tx.Create(&user).Error; err != nil {
			return nil, err
		}
		idMap[item.ID] = user.ID
	}
	return idMap, nil
}

func restoreNotifyChannels(tx *gorm.DB, channels []BackupNotifyChannel) (map[uint]uint, error) {
	idMap := make(map[uint]uint, len(channels))
	for _, item := range channels {
		channel := model.NotifyChannel{
			Name:      item.Name,
			Type:      item.Type,
			Config:    item.Config,
			Enabled:   item.Enabled,
			CreatedAt: item.CreatedAt,
			UpdatedAt: item.UpdatedAt,
		}
		if err := tx.Create(&channel).Error; err != nil {
			return nil, err
		}
		idMap[item.ID] = channel.ID
	}
	return idMap, nil
}

func restoreOpenApps(tx *gorm.DB, apps []BackupOpenApp) error {
	for _, item := range apps {
		app := model.OpenApp{
			Name:      item.Name,
			AppKey:    item.AppKey,
			AppSecret: item.AppSecret,
			Scopes:    item.Scopes,
			Enabled:   item.Enabled,
			RateLimit: item.RateLimit,
			CallCount: item.CallCount,
			CreatedAt: item.CreatedAt,
			UpdatedAt: item.UpdatedAt,
		}
		if err := tx.Create(&app).Error; err != nil {
			return err
		}
	}
	return nil
}

func restoreIPWhitelists(tx *gorm.DB, items []model.IPWhitelist) error {
	for _, item := range items {
		item.ID = 0
		if err := tx.Create(&item).Error; err != nil {
			return err
		}
	}
	return nil
}

func restoreTwoFactorAuths(tx *gorm.DB, items []BackupTwoFactorAuth, userIDMap map[uint]uint) error {
	for _, item := range items {
		userID := userIDMap[item.UserID]
		if userID == 0 {
			continue
		}
		record := model.TwoFactorAuth{
			UserID:    userID,
			Secret:    item.Secret,
			Enabled:   item.Enabled,
			CreatedAt: item.CreatedAt,
			UpdatedAt: item.UpdatedAt,
		}
		if err := tx.Create(&record).Error; err != nil {
			return err
		}
	}
	return nil
}

func restoreEnvVars(tx *gorm.DB, envVars []model.EnvVar) error {
	for _, item := range envVars {
		item.ID = 0
		if err := tx.Create(&item).Error; err != nil {
			return err
		}
	}
	return nil
}

func restoreTasks(tx *gorm.DB, tasks []model.Task, notifyChannelIDMap map[uint]uint) (map[uint]uint, error) {
	idMap := make(map[uint]uint, len(tasks))
	pendingDepends := make(map[uint]uint)

	for _, item := range tasks {
		oldID := item.ID
		oldDepends := item.DependsOn
		oldNotificationChannelID := item.NotificationChannelID
		item.ID = 0
		item.PID = nil
		item.TaskType = item.GetTaskType()
		item.Status = normalizeRestoredTaskStatus(item.Status)
		item.DependsOn = nil
		item.NotificationChannelID = nil
		if oldNotificationChannelID != nil {
			if mapped := notifyChannelIDMap[*oldNotificationChannelID]; mapped != 0 {
				item.NotificationChannelID = &mapped
			}
		}

		if err := tx.Select("*").Create(&item).Error; err != nil {
			return nil, err
		}
		idMap[oldID] = item.ID
		if oldDepends != nil {
			pendingDepends[item.ID] = *oldDepends
		}
	}

	for newID, oldDepends := range pendingDepends {
		mapped := idMap[oldDepends]
		if mapped == 0 {
			continue
		}
		if err := tx.Model(&model.Task{}).Where("id = ?", newID).Update("depends_on", mapped).Error; err != nil {
			return nil, err
		}
	}

	return idMap, nil
}

func normalizeRestoredTaskStatus(status float64) float64 {
	if status == model.TaskStatusDisabled {
		return model.TaskStatusDisabled
	}
	return model.TaskStatusEnabled
}

func restoreSSHKeys(tx *gorm.DB, keys []BackupSSHKey) (map[uint]uint, error) {
	idMap := make(map[uint]uint, len(keys))
	for _, item := range keys {
		key := model.SSHKey{
			Name:       item.Name,
			PrivateKey: item.PrivateKey,
			CreatedAt:  item.CreatedAt,
			UpdatedAt:  item.UpdatedAt,
		}
		if err := tx.Create(&key).Error; err != nil {
			return nil, err
		}
		idMap[item.ID] = key.ID
	}
	return idMap, nil
}

func restoreSubscriptions(tx *gorm.DB, subscriptions []model.Subscription, sshKeyIDMap map[uint]uint) error {
	for _, item := range subscriptions {
		item.ID = 0
		item.Status = 0
		if item.SSHKeyID != nil {
			mapped := sshKeyIDMap[*item.SSHKeyID]
			if mapped == 0 {
				item.SSHKeyID = nil
			} else {
				item.SSHKeyID = &mapped
			}
		}
		if err := tx.Create(&item).Error; err != nil {
			return err
		}
	}
	return nil
}

func restoreTaskLogs(tx *gorm.DB, logs []BackupTaskLog, taskIDMap map[uint]uint) error {
	for _, item := range logs {
		taskID := taskIDMap[item.TaskID]
		if taskID == 0 && item.TaskName != "" {
			var task model.Task
			if err := tx.Where("name = ?", item.TaskName).First(&task).Error; err == nil {
				taskID = task.ID
			}
		}
		if taskID == 0 {
			continue
		}

		logRecord := model.TaskLog{
			TaskID:    taskID,
			Content:   item.Content,
			Status:    item.Status,
			Duration:  item.Duration,
			LogPath:   item.LogPath,
			StartedAt: item.StartedAt,
			EndedAt:   item.EndedAt,
			CreatedAt: item.CreatedAt,
			UpdatedAt: item.UpdatedAt,
		}
		if err := tx.Create(&logRecord).Error; err != nil {
			return err
		}
	}
	return nil
}

func restoreDependencies(tx *gorm.DB, deps []BackupDependency) ([]model.Dependency, error) {
	pending := make([]model.Dependency, 0, len(deps))
	seen := map[string]struct{}{}
	for _, item := range deps {
		depType := strings.TrimSpace(item.Type)
		name := strings.TrimSpace(item.Name)
		if depType == "" || name == "" {
			continue
		}

		key := depType + "::" + strings.ToLower(name)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}

		dep := model.Dependency{
			Type: depType,
			Name: name,
		}
		if DependencyInstalled(depType, name) {
			dep.Status = model.DepStatusInstalled
			dep.Log = "[恢复备份] 已检测到依赖已存在，无需重装"
		} else {
			dep.Status = model.DepStatusInstalling
			dep.Log = "[恢复备份] 已提交依赖重装"
		}
		if err := tx.Create(&dep).Error; err != nil {
			return nil, err
		}
		if dep.Status == model.DepStatusInstalling {
			pending = append(pending, dep)
		}
	}
	return pending, nil
}

func restoreScriptFiles(extractedDir, source string) error {
	switch source {
	case "qinglong":
		return restoreQingLongScripts(extractedDir)
	default:
		sourceDir := filepath.Join(extractedDir, "files", "scripts")
		if _, err := os.Stat(sourceDir); os.IsNotExist(err) {
			return nil
		}
		if err := clearDirectoryContents(config.C.Data.ScriptsDir); err != nil {
			return err
		}
		return copyDirectoryContents(sourceDir, config.C.Data.ScriptsDir)
	}
}

func restoreLogFiles(extractedDir, source string) error {
	if err := clearDirectoryContents(config.C.Data.LogDir); err != nil {
		return err
	}
	panelLogPath := filepath.Join(config.C.Data.Dir, "panel.log")
	_ = os.Remove(panelLogPath)

	switch source {
	case "qinglong":
		return restoreQingLongLogs(extractedDir)
	default:
		logDir := filepath.Join(extractedDir, "files", "logs")
		if err := copyDirectoryContents(logDir, config.C.Data.LogDir); err != nil {
			return err
		}
		panelLogSource := filepath.Join(extractedDir, "files", "panel.log")
		if _, err := os.Stat(panelLogSource); err == nil {
			if err := copyFile(panelLogSource, panelLogPath); err != nil {
				return err
			}
		}
		return nil
	}
}

func clearDirectoryContents(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if err := os.RemoveAll(filepath.Join(dir, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}

func copyDirectoryContents(sourceDir, targetDir string) error {
	if _, err := os.Stat(sourceDir); os.IsNotExist(err) {
		return nil
	}
	return filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relPath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}
		targetPath := filepath.Join(targetDir, relPath)
		if info.IsDir() {
			return os.MkdirAll(targetPath, 0o755)
		}
		return copyFile(path, targetPath)
	})
}

func copyFile(sourcePath, targetPath string) error {
	info, err := os.Stat(sourcePath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return err
	}

	source, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer source.Close()

	target, err := os.OpenFile(targetPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode())
	if err != nil {
		return err
	}
	defer target.Close()

	_, err = io.Copy(target, source)
	return err
}

func reinstallDependenciesAsync(deps []model.Dependency) {
	go func() {
		for _, dep := range deps {
			reinstallDependency(dep)
		}
	}()
}

func reinstallDependency(dep model.Dependency) {
	depsDir := filepath.Join(config.C.Data.Dir, "deps")
	database.DB.Model(&model.Dependency{}).Where("id = ?", dep.ID).Update("log", fmt.Sprintf("[恢复备份] 正在安装 %s 依赖 %s", dep.Type, dep.Name))

	var cmd *exec.Cmd
	switch dep.Type {
	case model.DepTypeNodeJS:
		cmd = exec.Command("npm", "install", "--prefix", filepath.Join(depsDir, "nodejs"), dep.Name)
		cmd.Env = NpmInstallEnv(AppendProxyEnv(os.Environ()), CurrentNpmMirror())
	case model.DepTypePython:
		pipBin := filepath.Join(depsDir, "python", "venv", "bin", "pip")
		if _, err := os.Stat(pipBin); err != nil {
			pipBin = "pip3"
		}
		cmd = exec.Command(pipBin, "install", dep.Name)
		cmd.Env = append(PipInstallEnv(AppendProxyEnv(os.Environ()), CurrentPipMirror()), "TMPDIR=/tmp")
	case model.DepTypeLinux:
		var err error
		cmd, err = buildLinuxDependencyInstallCommand(dep.Name)
		if err != nil {
			database.DB.Model(&model.Dependency{}).Where("id = ?", dep.ID).Updates(map[string]interface{}{
				"status": model.DepStatusFailed,
				"log":    "[恢复备份] " + err.Error(),
			})
			return
		}
	default:
		database.DB.Model(&model.Dependency{}).Where("id = ?", dep.ID).Updates(map[string]interface{}{
			"status": model.DepStatusFailed,
			"log":    "[恢复备份] 不支持的依赖类型",
		})
		return
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		database.DB.Model(&model.Dependency{}).Where("id = ?", dep.ID).Updates(map[string]interface{}{
			"status": model.DepStatusFailed,
			"log":    fmt.Sprintf("[恢复备份] 安装失败\n%s", strings.TrimSpace(string(output))),
		})
		return
	}

	database.DB.Model(&model.Dependency{}).Where("id = ?", dep.ID).Updates(map[string]interface{}{
		"status": model.DepStatusInstalled,
		"log":    fmt.Sprintf("[恢复备份] 安装成功\n%s", strings.TrimSpace(string(output))),
	})
}

func buildLinuxDependencyInstallCommand(packageName string) (*exec.Cmd, error) {
	for _, binary := range []string{"apk", "apt-get", "dnf", "yum", "microdnf", "zypper"} {
		if _, err := exec.LookPath(binary); err != nil {
			continue
		}

		switch binary {
		case "apk":
			cmd := exec.Command(binary, "add", "--no-cache", packageName)
			cmd.Env = AppendProxyEnv(append(os.Environ(), "TMPDIR=/tmp"))
			return cmd, nil
		case "apt-get":
			script := "export DEBIAN_FRONTEND=noninteractive; apt-get update; apt-get install -y --no-install-recommends " + shellQuoteForBackup(packageName)
			cmd := exec.Command("sh", "-lc", script)
			cmd.Env = AppendProxyEnv(append(os.Environ(), "TMPDIR=/tmp"))
			return cmd, nil
		case "dnf", "yum", "microdnf":
			cmd := exec.Command(binary, "install", "-y", packageName)
			cmd.Env = AppendProxyEnv(append(os.Environ(), "TMPDIR=/tmp"))
			return cmd, nil
		case "zypper":
			cmd := exec.Command(binary, "--non-interactive", "install", packageName)
			cmd.Env = AppendProxyEnv(append(os.Environ(), "TMPDIR=/tmp"))
			return cmd, nil
		}
	}

	return nil, fmt.Errorf("未检测到可用的 Linux 包管理器")
}

func shellQuoteForBackup(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}
