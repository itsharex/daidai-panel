package service

import (
	"strings"
	"testing"

	"daidai-panel/database"
	"daidai-panel/model"
	"daidai-panel/testutil"
)

func TestReconcileDependenciesAfterRestartResumesRestoreJobs(t *testing.T) {
	testutil.SetupTestEnv(t)

	dep := &model.Dependency{
		Type:   model.DepTypeNodeJS,
		Name:   "left-pad",
		Status: model.DepStatusInstalling,
		Log:    "[恢复备份] 已提交依赖重装",
	}
	if err := database.DB.Create(dep).Error; err != nil {
		t.Fatalf("create dependency: %v", err)
	}

	originalInstalled := dependencyInstalledFunc
	originalReinstallBatch := dependencyReinstallBatchFunc
	t.Cleanup(func() {
		dependencyInstalledFunc = originalInstalled
		dependencyReinstallBatchFunc = originalReinstallBatch
	})

	dependencyInstalledFunc = func(depType, name string) bool {
		return false
	}

	var resumed []model.Dependency
	dependencyReinstallBatchFunc = func(deps []model.Dependency) {
		resumed = append(resumed, deps...)
	}

	ReconcileDependenciesAfterRestart()

	if len(resumed) != 1 {
		t.Fatalf("expected 1 dependency to resume, got %d", len(resumed))
	}
	if resumed[0].ID != dep.ID {
		t.Fatalf("expected resumed dependency id %d, got %d", dep.ID, resumed[0].ID)
	}

	var updated model.Dependency
	if err := database.DB.First(&updated, dep.ID).Error; err != nil {
		t.Fatalf("reload dependency: %v", err)
	}
	if updated.Status != model.DepStatusInstalling {
		t.Fatalf("expected dependency to stay installing, got %q", updated.Status)
	}
	if !strings.Contains(updated.Log, "已在重启后继续安装") {
		t.Fatalf("expected restart resume log, got %q", updated.Log)
	}
}

func TestRestoreBackupManifestPreservesCurrentPanelUsers(t *testing.T) {
	testutil.SetupTestEnv(t)

	currentUser := testutil.MustCreateUser(t, "current-admin", "admin")
	currentUser.Password = "current-password-hash"
	if err := database.DB.Model(currentUser).Update("password", currentUser.Password).Error; err != nil {
		t.Fatalf("update current user password: %v", err)
	}

	current2FA := &model.TwoFactorAuth{
		UserID:  currentUser.ID,
		Secret:  "current-2fa-secret",
		Enabled: true,
	}
	if err := database.DB.Create(current2FA).Error; err != nil {
		t.Fatalf("create current 2fa: %v", err)
	}

	if err := database.DB.Create(&model.OpenApp{
		Name:      "old-app",
		AppKey:    "old-key",
		AppSecret: "old-secret",
		Scopes:    "envs",
		Enabled:   true,
		RateLimit: 100,
	}).Error; err != nil {
		t.Fatalf("create old app: %v", err)
	}

	manifest := BackupManifest{
		Format:  "daidai-panel-backup",
		Version: "0.4.0",
		Source:  "daidai-panel",
		Selection: BackupSelection{
			Configs: true,
		},
		Data: BackupPayload{
			Configs: BackupConfigBundle{
				SystemConfigs: []model.SystemConfig{
					{Key: "panel_title", Value: "来自备份的标题"},
				},
				Users: []BackupUser{
					{ID: 99, Username: "backup-admin", PasswordHash: "backup-password-hash", Role: "admin", Enabled: true},
				},
				TwoFactorAuths: []BackupTwoFactorAuth{
					{UserID: 99, Secret: "backup-2fa-secret", Enabled: true},
				},
				OpenApps: []BackupOpenApp{
					{Name: "backup-app", AppKey: "backup-key", AppSecret: "backup-secret", Scopes: "envs", Enabled: true, RateLimit: 200},
				},
			},
		},
	}

	if err := restoreBackupManifest(manifest, t.TempDir()); err != nil {
		t.Fatalf("restore backup manifest: %v", err)
	}

	var users []model.User
	if err := database.DB.Order("id ASC").Find(&users).Error; err != nil {
		t.Fatalf("list users: %v", err)
	}
	if len(users) != 1 {
		t.Fatalf("expected current users to be preserved without importing backup users, got %d users", len(users))
	}
	if users[0].Username != "current-admin" {
		t.Fatalf("expected current user to remain, got %q", users[0].Username)
	}
	if users[0].Password != "current-password-hash" {
		t.Fatalf("expected current password hash to stay unchanged, got %q", users[0].Password)
	}

	var twoFactor []model.TwoFactorAuth
	if err := database.DB.Find(&twoFactor).Error; err != nil {
		t.Fatalf("list 2fa records: %v", err)
	}
	if len(twoFactor) != 1 {
		t.Fatalf("expected current 2fa to be preserved, got %d records", len(twoFactor))
	}
	if twoFactor[0].Secret != "current-2fa-secret" {
		t.Fatalf("expected current 2fa secret to stay unchanged, got %q", twoFactor[0].Secret)
	}

	if got := model.GetRegisteredConfig("panel_title"); got != "来自备份的标题" {
		t.Fatalf("expected panel_title to restore from backup, got %q", got)
	}

	var apps []model.OpenApp
	if err := database.DB.Order("id ASC").Find(&apps).Error; err != nil {
		t.Fatalf("list open apps: %v", err)
	}
	if len(apps) != 1 || apps[0].Name != "backup-app" {
		t.Fatalf("expected non-user config data to restore from backup, got %+v", apps)
	}
}
