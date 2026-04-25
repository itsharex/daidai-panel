package handler_test

import (
	"net/http"
	"testing"

	"daidai-panel/database"
	"daidai-panel/handler"
	"daidai-panel/model"
	"daidai-panel/testutil"

	"github.com/gin-gonic/gin"
)

func TestUserListIncludesTwoFactorEnabledState(t *testing.T) {
	testutil.SetupTestEnv(t)

	engine := gin.New()
	api := engine.Group("/api/v1")
	handler.NewUserHandler().RegisterRoutes(api)
	admin := testutil.MustCreateUser(t, "user-admin", "admin")
	adminToken := testutil.MustCreateAccessToken(t, admin.Username, admin.Role)

	securedUser := testutil.MustCreateUser(t, "user-with-2fa", "operator")
	plainUser := testutil.MustCreateUser(t, "user-without-2fa", "viewer")

	if err := database.DB.Create(&model.TwoFactorAuth{
		UserID:  securedUser.ID,
		Secret:  "SECRET",
		Enabled: true,
	}).Error; err != nil {
		t.Fatalf("create 2fa record: %v", err)
	}

	if err := database.DB.Create(&model.TwoFactorAuth{
		UserID:  plainUser.ID,
		Secret:  "DISABLED",
		Enabled: false,
	}).Error; err != nil {
		t.Fatalf("create disabled 2fa record: %v", err)
	}

	rec := performRequest(engine, http.MethodGet, "/api/v1/users", map[string]string{
		"Authorization": "Bearer " + adminToken,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	payload := decodeJSONMap(t, rec)
	items, ok := payload["data"].([]interface{})
	if !ok {
		t.Fatalf("expected user list array, got %#v", payload["data"])
	}

	stateByUsername := make(map[string]bool, len(items))
	for _, raw := range items {
		item := raw.(map[string]interface{})
		username, _ := item["username"].(string)
		enabled, _ := item["two_factor_enabled"].(bool)
		stateByUsername[username] = enabled
	}

	if !stateByUsername[securedUser.Username] {
		t.Fatalf("expected %s to expose two_factor_enabled=true, got %#v", securedUser.Username, stateByUsername)
	}
	if stateByUsername[plainUser.Username] {
		t.Fatalf("expected %s to expose two_factor_enabled=false, got %#v", plainUser.Username, stateByUsername)
	}
}
