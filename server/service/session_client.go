package service

import "strings"

const (
	SessionClientWeb = "web"
	SessionClientApp = "app"
)

func DetectSessionClientType(headerClientType, headerClientApp, userAgent string) string {
	clientType := strings.ToLower(strings.TrimSpace(headerClientType))
	switch clientType {
	case SessionClientWeb, SessionClientApp:
		return clientType
	}

	clientApp := strings.ToLower(strings.TrimSpace(headerClientApp))
	switch clientApp {
	case "daidai-panel-app":
		return SessionClientApp
	case "daidai-panel-web":
		return SessionClientWeb
	}

	ua := strings.ToLower(strings.TrimSpace(userAgent))
	switch {
	case strings.Contains(ua, "daidaipanelapp/"):
		return SessionClientApp
	case strings.Contains(ua, "dart/"), strings.Contains(ua, "dart:io"):
		return SessionClientApp
	case strings.Contains(ua, " cfnetwork/"), strings.Contains(ua, " okhttp/"):
		return SessionClientApp
	default:
		return SessionClientWeb
	}
}

func NormalizeSessionClientType(clientType string) string {
	if strings.EqualFold(strings.TrimSpace(clientType), SessionClientApp) {
		return SessionClientApp
	}
	return SessionClientWeb
}

func SessionClientLabel(clientType string) string {
	if NormalizeSessionClientType(clientType) == SessionClientApp {
		return "App端"
	}
	return "网页端"
}
