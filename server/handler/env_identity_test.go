package handler_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"daidai-panel/database"
	"daidai-panel/model"
	"daidai-panel/testutil"
)

// TestCreateUpsertsOnNameAndRemarks exercises the plugin-friendly upsert:
// posting the same (name, remarks) must overwrite the existing value in
// place, not create a second row.
func TestCreateUpsertsOnNameAndRemarks(t *testing.T) {
	testutil.SetupTestEnv(t)

	engine := newProtectedRouter()
	user := testutil.MustCreateUser(t, "env-upsert-operator", "operator")
	token := testutil.MustCreateAccessToken(t, user.Username, user.Role)

	firstBody := `{"name":"elmck","value":"123","remarks":"到期时间2.13"}`
	firstRec := performJSONRequest(engine, http.MethodPost, "/api/v1/envs", firstBody,
		map[string]string{"Authorization": "Bearer " + token}, "")
	if firstRec.Code != http.StatusCreated {
		t.Fatalf("expected initial create to return 201, got %d body=%s", firstRec.Code, firstRec.Body.String())
	}

	firstPayload := decodeJSONMap(t, firstRec)
	firstData, _ := firstPayload["data"].(map[string]interface{})
	firstIDFloat, _ := firstData["id"].(float64)
	firstID := uint(firstIDFloat)
	if firstID == 0 {
		t.Fatalf("missing id on first create, body=%s", firstRec.Body.String())
	}

	// Second post — same (name, remarks), new value — should overwrite.
	secondBody := `{"name":"elmck","value":"456","remarks":"到期时间2.13"}`
	secondRec := performJSONRequest(engine, http.MethodPost, "/api/v1/envs", secondBody,
		map[string]string{"Authorization": "Bearer " + token}, "")
	if secondRec.Code != http.StatusOK {
		t.Fatalf("expected upsert to return 200, got %d body=%s", secondRec.Code, secondRec.Body.String())
	}

	secondPayload := decodeJSONMap(t, secondRec)
	secondData, _ := secondPayload["data"].(map[string]interface{})
	secondIDFloat, _ := secondData["id"].(float64)
	if uint(secondIDFloat) != firstID {
		t.Fatalf("expected same id after upsert, got first=%d second=%d", firstID, uint(secondIDFloat))
	}
	if got, _ := secondData["value"].(string); got != "456" {
		t.Fatalf("expected value overwritten to 456, got %q", got)
	}

	var rowCount int64
	database.DB.Model(&model.EnvVar{}).Where("name = ?", "elmck").Count(&rowCount)
	if rowCount != 1 {
		t.Fatalf("expected exactly one row after upsert, got %d", rowCount)
	}
}

// TestCreateInsertsWhenRemarksDiffer makes sure different remarks for the
// same name still create separate rows (multi-account scenario).
func TestCreateInsertsWhenRemarksDiffer(t *testing.T) {
	testutil.SetupTestEnv(t)

	engine := newProtectedRouter()
	user := testutil.MustCreateUser(t, "env-multi-operator", "operator")
	token := testutil.MustCreateAccessToken(t, user.Username, user.Role)

	bodies := []string{
		`{"name":"elmck","value":"123","remarks":"account-A"}`,
		`{"name":"elmck","value":"456","remarks":"account-B"}`,
	}
	for _, body := range bodies {
		rec := performJSONRequest(engine, http.MethodPost, "/api/v1/envs", body,
			map[string]string{"Authorization": "Bearer " + token}, "")
		if rec.Code != http.StatusCreated {
			t.Fatalf("expected create 201 for %s, got %d body=%s", body, rec.Code, rec.Body.String())
		}
	}

	var count int64
	database.DB.Model(&model.EnvVar{}).Where("name = ?", "elmck").Count(&count)
	if count != 2 {
		t.Fatalf("expected 2 rows for multi-account elmck, got %d", count)
	}
}

// TestCreateUpsertsIgnoresEmptyRemarksQuirk removes the old "only upsert when
// remarks non-empty" behavior — empty remarks should still dedupe.
func TestCreateUpsertsWithEmptyRemarks(t *testing.T) {
	testutil.SetupTestEnv(t)

	engine := newProtectedRouter()
	user := testutil.MustCreateUser(t, "env-empty-operator", "operator")
	token := testutil.MustCreateAccessToken(t, user.Username, user.Role)

	firstBody := `{"name":"FOO","value":"alpha","remarks":""}`
	firstRec := performJSONRequest(engine, http.MethodPost, "/api/v1/envs", firstBody,
		map[string]string{"Authorization": "Bearer " + token}, "")
	if firstRec.Code != http.StatusCreated {
		t.Fatalf("expected initial 201, got %d body=%s", firstRec.Code, firstRec.Body.String())
	}

	secondBody := `{"name":"FOO","value":"beta","remarks":""}`
	secondRec := performJSONRequest(engine, http.MethodPost, "/api/v1/envs", secondBody,
		map[string]string{"Authorization": "Bearer " + token}, "")
	if secondRec.Code != http.StatusOK {
		t.Fatalf("expected upsert 200 for empty-remarks match, got %d body=%s", secondRec.Code, secondRec.Body.String())
	}

	var count int64
	database.DB.Model(&model.EnvVar{}).Where("name = ?", "FOO").Count(&count)
	if count != 1 {
		t.Fatalf("expected single FOO row, got %d", count)
	}
}

// TestUpdateAcceptsEnabledAndValueInSingleRequest verifies the enabled flag
// now flows through the generic PUT endpoint instead of needing a separate
// /enable or /disable call.
func TestUpdateAcceptsEnabledAndValueInSingleRequest(t *testing.T) {
	testutil.SetupTestEnv(t)

	engine := newProtectedRouter()
	user := testutil.MustCreateUser(t, "env-update-operator", "operator")
	token := testutil.MustCreateAccessToken(t, user.Username, user.Role)

	env := &model.EnvVar{Name: "TOGGLED", Value: "on", Enabled: true, Position: 1000}
	if err := database.DB.Create(env).Error; err != nil {
		t.Fatalf("seed env: %v", err)
	}

	body := `{"value":"off","enabled":false,"remarks":"disabled-by-admin"}`
	rec := performJSONRequest(engine, http.MethodPut, fmt.Sprintf("/api/v1/envs/%d", env.ID), body,
		map[string]string{"Authorization": "Bearer " + token}, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var reloaded model.EnvVar
	if err := database.DB.First(&reloaded, env.ID).Error; err != nil {
		t.Fatalf("reload env: %v", err)
	}
	if reloaded.Value != "off" {
		t.Fatalf("expected value off, got %q", reloaded.Value)
	}
	if reloaded.Enabled {
		t.Fatalf("expected enabled=false, got true")
	}
	if reloaded.Remarks != "disabled-by-admin" {
		t.Fatalf("expected remarks updated, got %q", reloaded.Remarks)
	}
}

// TestUpdateRejectsIdentityCollision keeps two rows distinct — renaming or
// rewriting remarks cannot stomp an existing (name, remarks) pair owned by
// another row.
func TestUpdateRejectsIdentityCollision(t *testing.T) {
	testutil.SetupTestEnv(t)

	engine := newProtectedRouter()
	user := testutil.MustCreateUser(t, "env-collision-operator", "operator")
	token := testutil.MustCreateAccessToken(t, user.Username, user.Role)

	keeper := &model.EnvVar{Name: "KEEPER", Value: "k", Remarks: "stable", Enabled: true, Position: 1000}
	victim := &model.EnvVar{Name: "VICTIM", Value: "v", Remarks: "movable", Enabled: true, Position: 2000}
	if err := database.DB.Create(keeper).Error; err != nil {
		t.Fatalf("seed keeper: %v", err)
	}
	if err := database.DB.Create(victim).Error; err != nil {
		t.Fatalf("seed victim: %v", err)
	}

	body := `{"name":"KEEPER","remarks":"stable"}`
	rec := performJSONRequest(engine, http.MethodPut, fmt.Sprintf("/api/v1/envs/%d", victim.ID), body,
		map[string]string{"Authorization": "Bearer " + token}, "")
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409 on identity collision, got %d body=%s", rec.Code, rec.Body.String())
	}

	var reloaded model.EnvVar
	if err := database.DB.First(&reloaded, victim.ID).Error; err != nil {
		t.Fatalf("reload victim: %v", err)
	}
	if reloaded.Name != "VICTIM" || reloaded.Remarks != "movable" {
		t.Fatalf("victim should be unchanged after reject, got name=%q remarks=%q", reloaded.Name, reloaded.Remarks)
	}
}

// TestUpdateSkipsDatabaseWriteWhenNoChange asserts the change-detection short
// circuit: unchanged payloads must not bump updated_at.
func TestUpdateSkipsDatabaseWriteWhenNoChange(t *testing.T) {
	testutil.SetupTestEnv(t)

	engine := newProtectedRouter()
	user := testutil.MustCreateUser(t, "env-noop-operator", "operator")
	token := testutil.MustCreateAccessToken(t, user.Username, user.Role)

	env := &model.EnvVar{Name: "UNCHANGED", Value: "same", Remarks: "same", Enabled: true, Position: 1000}
	if err := database.DB.Create(env).Error; err != nil {
		t.Fatalf("seed env: %v", err)
	}

	var before model.EnvVar
	if err := database.DB.First(&before, env.ID).Error; err != nil {
		t.Fatalf("load before: %v", err)
	}
	beforeUpdated := before.UpdatedAt

	body := `{"name":"UNCHANGED","value":"same","remarks":"same","enabled":true}`
	rec := performJSONRequest(engine, http.MethodPut, fmt.Sprintf("/api/v1/envs/%d", env.ID), body,
		map[string]string{"Authorization": "Bearer " + token}, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var after model.EnvVar
	if err := database.DB.First(&after, env.ID).Error; err != nil {
		t.Fatalf("load after: %v", err)
	}
	if !after.UpdatedAt.Equal(beforeUpdated) {
		t.Fatalf("expected updated_at unchanged on no-op update, before=%v after=%v", beforeUpdated, after.UpdatedAt)
	}
}

// TestImportMergeMatchesOnNameAndRemarks switches the import semantics from
// matching on (name, value) — where a refreshed value forked a duplicate — to
// matching on (name, remarks), so imports cleanly overwrite the value.
func TestImportMergeMatchesOnNameAndRemarks(t *testing.T) {
	testutil.SetupTestEnv(t)

	engine := newProtectedRouter()
	user := testutil.MustCreateUser(t, "env-import-operator", "operator")
	token := testutil.MustCreateAccessToken(t, user.Username, user.Role)

	existing := &model.EnvVar{Name: "PAIR", Value: "old", Remarks: "id-1", Enabled: false, Position: 1000}
	if err := database.DB.Create(existing).Error; err != nil {
		t.Fatalf("seed existing: %v", err)
	}

	payload := map[string]interface{}{
		"mode": "merge",
		"envs": []map[string]interface{}{
			{"name": "PAIR", "value": "new", "remarks": "id-1", "enabled": true},
		},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/envs/import", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", rec.Code, rec.Body.String())
	}

	var count int64
	database.DB.Model(&model.EnvVar{}).Where("name = ? AND remarks = ?", "PAIR", "id-1").Count(&count)
	if count != 1 {
		t.Fatalf("expected single row after merge import, got %d", count)
	}

	var reloaded model.EnvVar
	if err := database.DB.First(&reloaded, existing.ID).Error; err != nil {
		t.Fatalf("reload existing: %v", err)
	}
	if reloaded.Value != "new" {
		t.Fatalf("expected merged value new, got %q", reloaded.Value)
	}
	if !reloaded.Enabled {
		t.Fatalf("expected enabled flipped to true on merge, got false")
	}
}
