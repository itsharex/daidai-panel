package handler_test

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"daidai-panel/config"
	"daidai-panel/testutil"
)

func TestScriptUploadSupportsMultipleFiles(t *testing.T) {
	testutil.SetupTestEnv(t)

	engine := newProtectedRouter()
	user := testutil.MustCreateUser(t, "script-operator", "operator")
	token := testutil.MustCreateAccessToken(t, user.Username, user.Role)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	if err := writer.WriteField("dir", "batch"); err != nil {
		t.Fatalf("write dir field: %v", err)
	}

	fileCases := []struct {
		name    string
		content string
	}{
		{name: "one.py", content: "print('one')\n"},
		{name: "two.sh", content: "echo two\n"},
	}

	for _, fileCase := range fileCases {
		part, err := writer.CreateFormFile("file", fileCase.name)
		if err != nil {
			t.Fatalf("create form file %s: %v", fileCase.name, err)
		}
		if _, err := part.Write([]byte(fileCase.content)); err != nil {
			t.Fatalf("write form file %s: %v", fileCase.name, err)
		}
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/scripts/upload", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)

	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d, body=%s", rec.Code, rec.Body.String())
	}

	payload := decodeJSONMap(t, rec)
	if got, _ := payload["uploaded_count"].(float64); got != 2 {
		t.Fatalf("expected uploaded_count=2, got %v", payload["uploaded_count"])
	}

	paths, ok := payload["paths"].([]interface{})
	if !ok || len(paths) != 2 {
		t.Fatalf("expected 2 uploaded paths, got %#v", payload["paths"])
	}

	for _, fileCase := range fileCases {
		uploadedPath := filepath.Join(config.C.Data.ScriptsDir, "batch", fileCase.name)
		content, err := os.ReadFile(uploadedPath)
		if err != nil {
			t.Fatalf("read uploaded file %s: %v", uploadedPath, err)
		}
		if string(content) != fileCase.content {
			t.Fatalf("unexpected content for %s: %q", uploadedPath, string(content))
		}
	}
}

func TestScriptUploadAllowsCommonMobileFilenamesAndSupportsReadDelete(t *testing.T) {
	testutil.SetupTestEnv(t)

	engine := newProtectedRouter()
	user := testutil.MustCreateUser(t, "script-mobile", "operator")
	token := testutil.MustCreateAccessToken(t, user.Username, user.Role)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	fileName := "手机 导入 (1).sh"
	part, err := writer.CreateFormFile("file", fileName)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write([]byte("echo hi\r\necho again\r\n")); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/scripts/upload", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)

	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d, body=%s", rec.Code, rec.Body.String())
	}

	contentRec := performRequest(
		engine,
		http.MethodGet,
		"/api/v1/scripts/content?path="+url.QueryEscape(fileName),
		map[string]string{"Authorization": "Bearer " + token},
	)
	if contentRec.Code != http.StatusOK {
		t.Fatalf("expected content request to succeed, got %d, body=%s", contentRec.Code, contentRec.Body.String())
	}

	contentPayload := decodeJSONMap(t, contentRec)
	data, ok := contentPayload["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("unexpected content payload: %#v", contentPayload)
	}
	if got, _ := data["content"].(string); got != "echo hi\necho again\n" {
		t.Fatalf("expected normalized shell content, got %q", got)
	}

	deleteRec := performRequest(
		engine,
		http.MethodDelete,
		"/api/v1/scripts?path="+url.QueryEscape(fileName)+"&type=file",
		map[string]string{"Authorization": "Bearer " + token},
	)
	if deleteRec.Code != http.StatusOK {
		t.Fatalf("expected delete request to succeed, got %d, body=%s", deleteRec.Code, deleteRec.Body.String())
	}

	if _, err := os.Stat(filepath.Join(config.C.Data.ScriptsDir, fileName)); !os.IsNotExist(err) {
		t.Fatalf("expected uploaded file to be deleted, stat err=%v", err)
	}
}
