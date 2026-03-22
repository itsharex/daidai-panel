package handler_test

import (
	"fmt"
	"net/http"
	"testing"

	"daidai-panel/database"
	"daidai-panel/model"
	"daidai-panel/testutil"
)

func TestEnvBatchSetGroupUpdatesSelectedRows(t *testing.T) {
	testutil.SetupTestEnv(t)

	engine := newProtectedRouter()
	user := testutil.MustCreateUser(t, "env-operator", "operator")
	token := testutil.MustCreateAccessToken(t, user.Username, user.Role)

	envs := []*model.EnvVar{
		{Name: "FOO", Value: "1", Enabled: true, Position: 1000},
		{Name: "BAR", Value: "2", Enabled: true, Position: 2000},
	}
	for _, env := range envs {
		if err := database.DB.Create(env).Error; err != nil {
			t.Fatalf("create env %q: %v", env.Name, err)
		}
	}

	rec := performJSONRequest(
		engine,
		http.MethodPut,
		"/api/v1/envs/batch/group",
		fmt.Sprintf(`{"ids":[%d,%d],"group":"release"}`, envs[0].ID, envs[1].ID),
		map[string]string{"Authorization": "Bearer " + token},
		"",
	)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", rec.Code, rec.Body.String())
	}

	for _, env := range envs {
		var current model.EnvVar
		if err := database.DB.First(&current, env.ID).Error; err != nil {
			t.Fatalf("reload env %d: %v", env.ID, err)
		}
		if current.Group != "release" {
			t.Fatalf("expected env %d group release, got %q", env.ID, current.Group)
		}
	}
}

func TestEnvSortToFirstKeepsItemUnpinned(t *testing.T) {
	testutil.SetupTestEnv(t)

	engine := newProtectedRouter()
	user := testutil.MustCreateUser(t, "env-operator", "operator")
	token := testutil.MustCreateAccessToken(t, user.Username, user.Role)

	envs := []*model.EnvVar{
		{Name: "ALPHA", Value: "1", Enabled: true, SortOrder: 0, Position: 1000},
		{Name: "BETA", Value: "2", Enabled: true, SortOrder: 0, Position: 2000},
		{Name: "GAMMA", Value: "3", Enabled: true, SortOrder: 0, Position: 3000},
	}
	for _, env := range envs {
		if err := database.DB.Create(env).Error; err != nil {
			t.Fatalf("create env %q: %v", env.Name, err)
		}
	}

	rec := performJSONRequest(
		engine,
		http.MethodPut,
		"/api/v1/envs/sort",
		fmt.Sprintf(`{"source_id":%d,"target_id":%d}`, envs[2].ID, envs[0].ID),
		map[string]string{"Authorization": "Bearer " + token},
		"",
	)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", rec.Code, rec.Body.String())
	}

	var moved model.EnvVar
	if err := database.DB.First(&moved, envs[2].ID).Error; err != nil {
		t.Fatalf("reload moved env: %v", err)
	}
	if moved.SortOrder != 0 {
		t.Fatalf("expected moved env to remain unpinned, got sort_order=%d", moved.SortOrder)
	}

	listRec := performRequest(engine, http.MethodGet, "/api/v1/envs", map[string]string{
		"Authorization": "Bearer " + token,
	})
	if listRec.Code != http.StatusOK {
		t.Fatalf("expected list 200, got %d, body=%s", listRec.Code, listRec.Body.String())
	}

	payload := decodeJSONMap(t, listRec)
	items, ok := payload["data"].([]interface{})
	if !ok || len(items) < 3 {
		t.Fatalf("expected env list with at least 3 items, got %#v", payload["data"])
	}

	first, ok := items[0].(map[string]interface{})
	if !ok {
		t.Fatalf("expected first env object, got %#v", items[0])
	}
	if got, _ := first["name"].(string); got != "GAMMA" {
		t.Fatalf("expected GAMMA to be first after sort, got %q", got)
	}
	if sortOrder, _ := first["sort_order"].(float64); sortOrder != 0 {
		t.Fatalf("expected first env to be unpinned after drag, got sort_order=%v", sortOrder)
	}
}

func TestEnvMoveTopAndCancelTopUseExplicitPinnedState(t *testing.T) {
	testutil.SetupTestEnv(t)

	engine := newProtectedRouter()
	user := testutil.MustCreateUser(t, "env-operator", "operator")
	token := testutil.MustCreateAccessToken(t, user.Username, user.Role)

	envs := []*model.EnvVar{
		{Name: "LEFT", Value: "1", Enabled: true, SortOrder: 0, Position: 1000},
		{Name: "RIGHT", Value: "2", Enabled: true, SortOrder: 0, Position: 2000},
	}
	for _, env := range envs {
		if err := database.DB.Create(env).Error; err != nil {
			t.Fatalf("create env %q: %v", env.Name, err)
		}
	}

	moveTopRec := performRequest(engine, http.MethodPut, fmt.Sprintf("/api/v1/envs/%d/move-top", envs[1].ID), map[string]string{
		"Authorization": "Bearer " + token,
	})
	if moveTopRec.Code != http.StatusOK {
		t.Fatalf("expected move-top 200, got %d, body=%s", moveTopRec.Code, moveTopRec.Body.String())
	}

	var pinned model.EnvVar
	if err := database.DB.First(&pinned, envs[1].ID).Error; err != nil {
		t.Fatalf("reload pinned env: %v", err)
	}
	if pinned.SortOrder != 1 {
		t.Fatalf("expected sort_order=1 after move-top, got %d", pinned.SortOrder)
	}

	cancelTopRec := performRequest(engine, http.MethodPut, fmt.Sprintf("/api/v1/envs/%d/cancel-top", envs[1].ID), map[string]string{
		"Authorization": "Bearer " + token,
	})
	if cancelTopRec.Code != http.StatusOK {
		t.Fatalf("expected cancel-top 200, got %d, body=%s", cancelTopRec.Code, cancelTopRec.Body.String())
	}

	if err := database.DB.First(&pinned, envs[1].ID).Error; err != nil {
		t.Fatalf("reload unpinned env: %v", err)
	}
	if pinned.SortOrder != 0 {
		t.Fatalf("expected sort_order=0 after cancel-top, got %d", pinned.SortOrder)
	}
}

func TestEnvExportAllHonorsSelectedIDs(t *testing.T) {
	testutil.SetupTestEnv(t)

	engine := newProtectedRouter()
	user := testutil.MustCreateUser(t, "env-operator", "operator")
	token := testutil.MustCreateAccessToken(t, user.Username, user.Role)

	envs := []*model.EnvVar{
		{Name: "ALPHA", Value: "1", Enabled: true, Position: 1000},
		{Name: "BETA", Value: "2", Enabled: true, Position: 2000},
		{Name: "GAMMA", Value: "3", Enabled: false, Position: 3000},
	}
	for _, env := range envs {
		if err := database.DB.Create(env).Error; err != nil {
			t.Fatalf("create env %q: %v", env.Name, err)
		}
	}

	rec := performRequest(
		engine,
		http.MethodGet,
		fmt.Sprintf("/api/v1/envs/export-all?ids=%d,%d", envs[0].ID, envs[2].ID),
		map[string]string{"Authorization": "Bearer " + token},
	)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", rec.Code, rec.Body.String())
	}

	payload := decodeJSONMap(t, rec)
	items, ok := payload["data"].([]interface{})
	if !ok || len(items) != 2 {
		t.Fatalf("expected 2 exported envs, got %#v", payload["data"])
	}

	gotNames := make(map[string]struct{}, len(items))
	for _, item := range items {
		env, ok := item.(map[string]interface{})
		if !ok {
			t.Fatalf("expected env object, got %#v", item)
		}
		gotNames[env["name"].(string)] = struct{}{}
	}

	if _, exists := gotNames["ALPHA"]; !exists {
		t.Fatalf("expected ALPHA in export, got %v", gotNames)
	}
	if _, exists := gotNames["GAMMA"]; !exists {
		t.Fatalf("expected GAMMA in export, got %v", gotNames)
	}
	if _, exists := gotNames["BETA"]; exists {
		t.Fatalf("did not expect BETA in selected export, got %v", gotNames)
	}
}
