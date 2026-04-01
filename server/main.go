package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"daidai-panel/config"
	"daidai-panel/database"
	"daidai-panel/handler"
	"daidai-panel/middleware"
	"daidai-panel/model"
	"daidai-panel/router"
	"daidai-panel/service"

	"github.com/gin-gonic/gin"
)

type startupLogFilter struct {
	dst io.Writer
}

func (w *startupLogFilter) Write(p []byte) (int, error) {
	text := string(p)
	if shouldSuppressStartupLog(text) {
		return len(p), nil
	}
	return w.dst.Write(p)
}

func shouldSuppressStartupLog(text string) bool {
	markers := []string{
		"database connected:",
		"added missing column:",
		"column check completed",
		"scheduler v2 started:",
		"scheduler v2 initialized with",
		"scheduler v2 enqueued",
		"subscription scheduler initialized with",
		"resource watcher started",
		"server starting on",
	}

	for _, marker := range markers {
		if strings.Contains(text, marker) {
			return true
		}
	}

	return false
}

func buildAccessURLs(port int) []string {
	if port <= 0 {
		return nil
	}

	seen := map[string]struct{}{}
	var urls []string

	addURL := func(host string) {
		host = strings.TrimSpace(host)
		if host == "" {
			return
		}
		url := fmt.Sprintf("http://%s:%d", host, port)
		if _, exists := seen[url]; exists {
			return
		}
		seen[url] = struct{}{}
		urls = append(urls, url)
	}

	addURL("127.0.0.1")
	addURL("localhost")

	ifaces, err := net.Interfaces()
	if err != nil {
		return urls
	}

	var localIPs []string
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			ip = ip.To4()
			if ip == nil || ip.IsLoopback() {
				continue
			}

			localIPs = append(localIPs, ip.String())
		}
	}

	sort.Strings(localIPs)
	for _, ip := range localIPs {
		addURL(ip)
	}

	return urls
}

func printStartupSummary(port int) {
	urls := buildAccessURLs(port)
	fmt.Println("呆呆面板已经启动")
	if len(urls) == 0 {
		fmt.Printf("访问地址：http://127.0.0.1:%d\n", port)
		return
	}

	fmt.Println("访问地址：")
	for _, url := range urls {
		fmt.Println(url)
	}
	fmt.Printf("请使用上面显示的宿主机访问地址，不要直接使用容器内端口 %d/%d。\n", 5700, 5701)
}

func setupPanelLog(dataDir string) io.Writer {
	logFilePath := filepath.Join(dataDir, "panel.log")
	if err := os.MkdirAll(filepath.Dir(logFilePath), 0o755); err != nil {
		return os.Stdout
	}

	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return os.Stdout
	}

	return io.MultiWriter(os.Stdout, logFile)
}

func main() {
	cfg, err := config.Load("config.yaml")
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	panelWriter := setupPanelLog(cfg.Data.Dir)
	log.SetOutput(&startupLogFilter{dst: panelWriter})
	gin.DefaultWriter = panelWriter
	gin.DefaultErrorWriter = panelWriter

	database.Init(&cfg.Database)

	database.AutoMigrate(
		&model.User{},
		&model.TokenBlocklist{},
		&model.Task{},
		&model.TaskLog{},
		&model.SystemConfig{},
		&model.EnvVar{},
		&model.ScriptVersion{},
		&model.Subscription{},
		&model.SubLog{},
		&model.NotifyChannel{},
		&model.SSHKey{},
		&model.LoginLog{},
		&model.LoginAttempt{},
		&model.UserSession{},
		&model.IPWhitelist{},
		&model.SecurityAudit{},
		&model.TwoFactorAuth{},
		&model.OpenApp{},
		&model.ApiCallLog{},
		&model.Platform{},
		&model.PlatformToken{},
		&model.PlatformTokenLog{},
		&model.Dependency{},
		&model.TaskView{},
	)

	database.EnsureColumns()

	model.InitDefaultConfigs()
	if err := middleware.ConfigureTrustedProxyCIDRs(model.GetRegisteredConfig("trusted_proxy_cidrs")); err != nil {
		log.Fatalf("failed to configure trusted proxies: %v", err)
	}

	verifyInstalledDeps()
	handler.FinalizePendingAutoUpdateOnStartup()
	if err := service.EnsureBuiltinNotifyHelpers(cfg.Data.ScriptsDir); err != nil {
		log.Printf("prepare builtin notify helpers failed: %v", err)
	}
	if err := service.CleanupManagedHelperCopiesUnderRoot(cfg.Data.ScriptsDir); err != nil {
		log.Printf("cleanup duplicated notify helpers failed: %v", err)
	}

	service.InitSchedulerV2()
	defer service.ShutdownSchedulerV2()

	service.InitSubscriptionScheduler()
	defer service.ShutdownSubscriptionScheduler()

	service.StartResourceWatcher()
	defer service.StopResourceWatcher()

	handler.StartPanelAutoUpdateWatcher()
	defer handler.StopPanelAutoUpdateWatcher()

	if cfg.Server.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	engine := gin.New()
	if err := engine.SetTrustedProxies(middleware.CurrentTrustedProxyCIDRs()); err != nil {
		log.Fatalf("failed to apply trusted proxies to gin engine: %v", err)
	}
	engine.RemoteIPHeaders = []string{"X-Real-IP", "X-Forwarded-For"}
	engine.Use(gin.Logger())
	engine.Use(gin.Recovery())

	router.Setup(engine)

	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("server failed: %v", err)
	}

	log.SetOutput(panelWriter)
	printStartupSummary(cfg.Server.Port)

	if err := engine.RunListener(listener); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}

func verifyInstalledDeps() {
	service.ReconcileDependenciesAfterRestart()
}
