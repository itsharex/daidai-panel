package handler_test

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
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
