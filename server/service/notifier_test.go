package service

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestSplitNotificationTargets(t *testing.T) {
	got := splitNotificationTargets("uid-a; uid-b,\nuid-c\tuid-d")
	want := []string{"uid-a", "uid-b", "uid-c", "uid-d"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected targets: got %v want %v", got, want)
	}
}

func TestSplitNotificationIntTargets(t *testing.T) {
	got, err := splitNotificationIntTargets("101; 102,103")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []int{101, 102, 103}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected int targets: got %v want %v", got, want)
	}
}

func TestSplitNotificationIntTargetsRejectsInvalidValue(t *testing.T) {
	if _, err := splitNotificationIntTargets("101;abc"); err == nil {
		t.Fatal("expected invalid topic id to return an error")
	}
}

func TestSendWecomTextWithMentions(t *testing.T) {
	var body map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	err := sendWecom(map[string]string{
		"webhook":               server.URL,
		"msg_type":              "text",
		"content_template":      "{{title}}\n{{content}}",
		"mentioned_list":        "wangqing,@all",
		"mentioned_mobile_list": "13800001111",
	}, "告警标题", "告警内容")
	if err != nil {
		t.Fatalf("send wecom text: %v", err)
	}

	if got := body["msgtype"]; got != "text" {
		t.Fatalf("unexpected msgtype: %#v", got)
	}

	textBody, ok := body["text"].(map[string]interface{})
	if !ok {
		t.Fatalf("unexpected text payload: %#v", body["text"])
	}
	if got := textBody["content"]; got != "告警标题\n告警内容" {
		t.Fatalf("unexpected text content: %#v", got)
	}

	mentionedList, ok := textBody["mentioned_list"].([]interface{})
	if !ok || len(mentionedList) != 2 {
		t.Fatalf("unexpected mentioned_list: %#v", textBody["mentioned_list"])
	}
}

func TestSendWecomTemplateCard(t *testing.T) {
	var body map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	err := sendWecom(map[string]string{
		"webhook":  server.URL,
		"msg_type": "template_card",
		"template_card_payload": `{
			"card_type":"text_notice",
			"main_title":{"title":"{{title}}","desc":"{{content}}"}
		}`,
	}, "系统通知", "任务执行完成")
	if err != nil {
		t.Fatalf("send wecom template card: %v", err)
	}

	if got := body["msgtype"]; got != "template_card" {
		t.Fatalf("unexpected msgtype: %#v", got)
	}
	cardBody, ok := body["template_card"].(map[string]interface{})
	if !ok {
		t.Fatalf("unexpected template_card payload: %#v", body["template_card"])
	}
	mainTitle, ok := cardBody["main_title"].(map[string]interface{})
	if !ok {
		t.Fatalf("unexpected main_title: %#v", cardBody["main_title"])
	}
	if got := mainTitle["title"]; got != "系统通知" {
		t.Fatalf("unexpected template title: %#v", got)
	}
	if got := mainTitle["desc"]; got != "任务执行完成" {
		t.Fatalf("unexpected template desc: %#v", got)
	}
}

func TestSendWecomAppMarkdown(t *testing.T) {
	var (
		tokenRequested bool
		messageBody    map[string]interface{}
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/cgi-bin/gettoken":
			tokenRequested = true
			if got := r.URL.Query().Get("corpid"); got != "ww-demo" {
				t.Fatalf("unexpected corp id: %s", got)
			}
			if got := r.URL.Query().Get("corpsecret"); got != "secret-demo" {
				t.Fatalf("unexpected corp secret: %s", got)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"errcode":0,"errmsg":"ok","access_token":"token-demo"}`))
		case "/cgi-bin/message/send":
			if got := r.URL.Query().Get("access_token"); got != "token-demo" {
				t.Fatalf("unexpected access_token: %s", got)
			}
			if err := json.NewDecoder(r.Body).Decode(&messageBody); err != nil {
				t.Fatalf("decode message body: %v", err)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"errcode":0,"errmsg":"ok"}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	oldTokenURL := wecomAppTokenURL
	oldSendURL := wecomAppSendURL
	wecomAppTokenURL = server.URL + "/cgi-bin/gettoken"
	wecomAppSendURL = server.URL + "/cgi-bin/message/send"
	defer func() {
		wecomAppTokenURL = oldTokenURL
		wecomAppSendURL = oldSendURL
	}()

	err := sendWecomApp(map[string]string{
		"corp_id":  "ww-demo",
		"secret":   "secret-demo",
		"agent_id": "1000001",
		"to_user":  "@all",
		"msg_type": "markdown",
	}, "标题", "正文")
	if err != nil {
		t.Fatalf("send wecom app: %v", err)
	}
	if !tokenRequested {
		t.Fatal("expected token endpoint to be requested")
	}
	if got := messageBody["msgtype"]; got != "markdown" {
		t.Fatalf("unexpected msgtype: %#v", got)
	}
	if got := messageBody["touser"]; got != "@all" {
		t.Fatalf("unexpected touser: %#v", got)
	}

	markdown, ok := messageBody["markdown"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected markdown payload, got %#v", messageBody["markdown"])
	}
	if got := markdown["content"]; got != "**标题**\n正文" {
		t.Fatalf("unexpected markdown content: %#v", got)
	}
}

func TestSendWecomAppTextWithAdvancedOptions(t *testing.T) {
	var (
		tokenRequested bool
		messageBody    map[string]interface{}
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/cgi-bin/gettoken":
			tokenRequested = true
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"errcode":0,"errmsg":"ok","access_token":"token-demo"}`))
		case "/cgi-bin/message/send":
			if err := json.NewDecoder(r.Body).Decode(&messageBody); err != nil {
				t.Fatalf("decode message body: %v", err)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"errcode":0,"errmsg":"ok"}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	oldTokenURL := wecomAppTokenURL
	oldSendURL := wecomAppSendURL
	wecomAppTokenURL = server.URL + "/cgi-bin/gettoken"
	wecomAppSendURL = server.URL + "/cgi-bin/message/send"
	defer func() {
		wecomAppTokenURL = oldTokenURL
		wecomAppSendURL = oldSendURL
	}()

	err := sendWecomApp(map[string]string{
		"corp_id":                  "ww-demo",
		"secret":                   "secret-demo",
		"agent_id":                 "1000001",
		"to_user":                  "zhangsan|lisi",
		"msg_type":                 "text",
		"content_template":         "{{title}}\n{{content}}",
		"safe":                     "1",
		"enable_id_trans":          "1",
		"enable_duplicate_check":   "1",
		"duplicate_check_interval": "7200",
	}, "标题", "正文")
	if err != nil {
		t.Fatalf("send wecom app text: %v", err)
	}

	if !tokenRequested {
		t.Fatal("expected token endpoint to be requested")
	}
	if got := messageBody["msgtype"]; got != "text" {
		t.Fatalf("unexpected msgtype: %#v", got)
	}
	if got := messageBody["touser"]; got != "zhangsan|lisi" {
		t.Fatalf("unexpected touser: %#v", got)
	}
	if got := messageBody["safe"]; got != float64(1) {
		t.Fatalf("unexpected safe: %#v", got)
	}
	if got := messageBody["enable_id_trans"]; got != float64(1) {
		t.Fatalf("unexpected enable_id_trans: %#v", got)
	}
	if got := messageBody["enable_duplicate_check"]; got != float64(1) {
		t.Fatalf("unexpected enable_duplicate_check: %#v", got)
	}
	if got := messageBody["duplicate_check_interval"]; got != float64(7200) {
		t.Fatalf("unexpected duplicate_check_interval: %#v", got)
	}

	textBody, ok := messageBody["text"].(map[string]interface{})
	if !ok {
		t.Fatalf("unexpected text payload: %#v", messageBody["text"])
	}
	if got := textBody["content"]; got != "标题\n正文" {
		t.Fatalf("unexpected text content: %#v", got)
	}
}

func TestSendWecomAppTemplateCard(t *testing.T) {
	var messageBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/cgi-bin/gettoken":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"errcode":0,"errmsg":"ok","access_token":"token-demo"}`))
		case "/cgi-bin/message/send":
			if err := json.NewDecoder(r.Body).Decode(&messageBody); err != nil {
				t.Fatalf("decode message body: %v", err)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"errcode":0,"errmsg":"ok"}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	oldTokenURL := wecomAppTokenURL
	oldSendURL := wecomAppSendURL
	wecomAppTokenURL = server.URL + "/cgi-bin/gettoken"
	wecomAppSendURL = server.URL + "/cgi-bin/message/send"
	defer func() {
		wecomAppTokenURL = oldTokenURL
		wecomAppSendURL = oldSendURL
	}()

	err := sendWecomApp(map[string]string{
		"corp_id":  "ww-demo",
		"secret":   "secret-demo",
		"agent_id": "1000001",
		"to_user":  "@all",
		"msg_type": "template_card",
		"template_card_payload": `{
			"card_type":"text_notice",
			"main_title":{"title":"{{title}}","desc":"{{content}}"}
		}`,
	}, "系统通知", "任务执行完成")
	if err != nil {
		t.Fatalf("send wecom app template card: %v", err)
	}

	if got := messageBody["msgtype"]; got != "template_card" {
		t.Fatalf("unexpected msgtype: %#v", got)
	}
	cardBody, ok := messageBody["template_card"].(map[string]interface{})
	if !ok {
		t.Fatalf("unexpected template_card payload: %#v", messageBody["template_card"])
	}
	mainTitle, ok := cardBody["main_title"].(map[string]interface{})
	if !ok {
		t.Fatalf("unexpected main_title: %#v", cardBody["main_title"])
	}
	if got := mainTitle["title"]; got != "系统通知" {
		t.Fatalf("unexpected template title: %#v", got)
	}
	if got := mainTitle["desc"]; got != "任务执行完成" {
		t.Fatalf("unexpected template desc: %#v", got)
	}
}

func TestSendWecomAppReturnsEnterpriseError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/cgi-bin/gettoken":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"errcode":0,"errmsg":"ok","access_token":"token-demo"}`))
		case "/cgi-bin/message/send":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"errcode":40003,"errmsg":"invalid user","invaliduser":"zhangsan|lisi"}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	oldTokenURL := wecomAppTokenURL
	oldSendURL := wecomAppSendURL
	wecomAppTokenURL = server.URL + "/cgi-bin/gettoken"
	wecomAppSendURL = server.URL + "/cgi-bin/message/send"
	defer func() {
		wecomAppTokenURL = oldTokenURL
		wecomAppSendURL = oldSendURL
	}()

	err := sendWecomApp(map[string]string{
		"corp_id":  "ww-demo",
		"secret":   "secret-demo",
		"agent_id": "1000001",
		"to_user":  "@all",
		"msg_type": "text",
	}, "标题", "正文")
	if err == nil {
		t.Fatal("expected enterprise error")
	}
	if got := err.Error(); got != "发送企业微信应用消息失败: invalid user (invaliduser=zhangsan|lisi)" {
		t.Fatalf("unexpected error: %s", got)
	}
}
