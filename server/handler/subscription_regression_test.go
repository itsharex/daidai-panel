package handler_test

import (
	"net/http"
	"strconv"
	"testing"

	"daidai-panel/database"
	"daidai-panel/model"
	"daidai-panel/testutil"
)

func TestSubscriptionCreatePersistsForceOverwriteFalse(t *testing.T) {
	testutil.SetupTestEnv(t)

	operator := testutil.MustCreateUser(t, "subscription-operator", "operator")
	token := testutil.MustCreateAccessToken(t, operator.Username, operator.Role)
	engine := newProtectedRouter()

	body := `{"name":"demo-sub","type":"git-repo","url":"https://github.com/example/demo.git","force_overwrite":false}`
	rec := performJSONRequest(engine, http.MethodPost, "/api/v1/subscriptions", body, map[string]string{
		"Authorization": "Bearer " + token,
	}, "")

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d, body=%s", rec.Code, rec.Body.String())
	}

	payload := decodeJSONMap(t, rec)
	data, ok := payload["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected data map, got %T", payload["data"])
	}
	if got, _ := data["force_overwrite"].(bool); got {
		t.Fatalf("expected response force_overwrite false, got %v", data["force_overwrite"])
	}

	var sub model.Subscription
	if err := database.DB.Where("name = ?", "demo-sub").First(&sub).Error; err != nil {
		t.Fatalf("query subscription: %v", err)
	}
	if sub.ForceOverwrite == nil || *sub.ForceOverwrite {
		t.Fatalf("expected force_overwrite persisted false, got %#v", sub.ForceOverwrite)
	}
}

func TestSubscriptionUpdateKeepsForceOverwriteFalseAfterReload(t *testing.T) {
	testutil.SetupTestEnv(t)

	operator := testutil.MustCreateUser(t, "subscription-editor", "operator")
	token := testutil.MustCreateAccessToken(t, operator.Username, operator.Role)
	engine := newProtectedRouter()

	forceOverwrite := true
	sub := model.Subscription{
		Name:           "editable-sub",
		Type:           model.SubTypeGitRepo,
		URL:            "https://github.com/example/editable.git",
		Enabled:        true,
		ForceOverwrite: &forceOverwrite,
	}
	if err := database.DB.Create(&sub).Error; err != nil {
		t.Fatalf("create subscription: %v", err)
	}

	updateBody := `{"force_overwrite":false,"alias":"edited-sub"}`
	updateRec := performJSONRequest(engine, http.MethodPut, "/api/v1/subscriptions/"+strconv.FormatUint(uint64(sub.ID), 10), updateBody, map[string]string{
		"Authorization": "Bearer " + token,
	}, "")
	if updateRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", updateRec.Code, updateRec.Body.String())
	}

	var updated model.Subscription
	if err := database.DB.First(&updated, sub.ID).Error; err != nil {
		t.Fatalf("reload subscription: %v", err)
	}
	if updated.ForceOverwrite == nil || *updated.ForceOverwrite {
		t.Fatalf("expected force_overwrite updated false, got %#v", updated.ForceOverwrite)
	}

	listRec := performRequest(engine, http.MethodGet, "/api/v1/subscriptions?keyword=editable-sub", map[string]string{
		"Authorization": "Bearer " + token,
	})
	if listRec.Code != http.StatusOK {
		t.Fatalf("expected list 200, got %d, body=%s", listRec.Code, listRec.Body.String())
	}

	listPayload := decodeJSONMap(t, listRec)
	items, ok := listPayload["data"].([]interface{})
	if !ok || len(items) == 0 {
		t.Fatalf("expected subscription list, got %T", listPayload["data"])
	}
	item, ok := items[0].(map[string]interface{})
	if !ok {
		t.Fatalf("expected list item map, got %T", items[0])
	}
	if got, _ := item["force_overwrite"].(bool); got {
		t.Fatalf("expected list force_overwrite false after reload, got %v", item["force_overwrite"])
	}
}
