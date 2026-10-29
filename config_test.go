package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestRewrite(t *testing.T) {
	data, err := os.ReadFile("config.example.yaml")
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := parseConfig(data)
	if err != nil {
		t.Fatal(err)
	}

	const gw = "https://agentgateway.example.com"
	tests := []struct {
		in   string
		want string // empty: not rewritten
	}{
		{"https://api.anthropic.com/v1/messages?beta=true", gw + "/v1/messages?beta=true"},
		{"https://chatgpt.com/backend-api/codex/responses", gw + "/v1/responses"},
		{"https://api.openai.com/v1/responses", gw + "/v1/responses"},
		{"https://api.anthropic.com/v1/messages/count_tokens", ""},
		{"https://api.anthropic.com/api/oauth/profile", ""},
		{"https://chatgpt.com/backend-api/wham/usage", ""},
		{"https://api.openai.com/v1/models", ""},
		{"https://example.com/v1/messages", ""},
	}
	for _, tt := range tests {
		req := httptest.NewRequest(http.MethodPost, tt.in, nil)
		host := req.URL.Hostname()
		req.Header.Set("Authorization", "Bearer agent-token")
		got := cfg.rewrite(req, "agw_key")

		if tt.want == "" {
			if got || req.URL.String() != tt.in || req.Header.Get(cfg.KeyHeader) != "" {
				t.Errorf("%s: rewritten to %s", tt.in, req.URL)
			}
			continue
		}
		if !got || req.URL.String() != tt.want {
			t.Errorf("%s: got %s (rewritten %v), want %s", tt.in, req.URL, got, tt.want)
		}
		if req.Host != "agentgateway.example.com" {
			t.Errorf("%s: Host %q", tt.in, req.Host)
		}
		if k := req.Header.Get("x-agentgateway-key"); k != "agw_key" {
			t.Errorf("%s: key %q", tt.in, k)
		}
		if h := req.Header.Get("x-agent-proxy-host"); h != host {
			t.Errorf("%s: host header %q, want %q", tt.in, h, host)
		}
		if a := req.Header.Get("Authorization"); a != "Bearer agent-token" {
			t.Errorf("%s: Authorization %q", tt.in, a)
		}
	}
}

func TestRewriteGatewayPathAndHeaders(t *testing.T) {
	cfg, err := parseConfig([]byte(`
gateway: http://127.0.0.1:4000/llm/
keyHeader: x-key
hostHeader: x-host
routes:
  - host: API.Example.com
    paths:
      /v1/chat/: /chat
`))
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.intercepts("api.example.com") || cfg.intercepts("example.com") {
		t.Error("intercepts")
	}
	req := httptest.NewRequest(http.MethodPost, "https://api.example.com/v1/chat", nil)
	if !cfg.rewrite(req, "k") {
		t.Fatal("not rewritten")
	}
	if got := req.URL.String(); got != "http://127.0.0.1:4000/llm/chat" {
		t.Errorf("URL %s", got)
	}
	if req.Header.Get("x-key") != "k" || req.Header.Get("x-host") != "api.example.com" {
		t.Errorf("headers %v", req.Header)
	}
}

func TestParseConfigErrors(t *testing.T) {
	tests := map[string]string{
		"no gateway":    "routes: []",
		"bad gateway":   "gateway: agentgateway.example.com",
		"unknown field": "gateway: https://gw\ngatway: x",
		"empty header":  "gateway: https://gw\nkeyHeader: ''",
		"host with port": `gateway: https://gw
routes: [{host: "a.com:443", paths: {/a: /a}}]`,
		"duplicate host": `gateway: https://gw
routes: [{host: a.com, paths: {/a: /a}}, {host: A.com, paths: {/b: /b}}]`,
		"no paths": `gateway: https://gw
routes: [{host: a.com}]`,
		"relative path": `gateway: https://gw
routes: [{host: a.com, paths: {a: /a}}]`,
	}
	for name, data := range tests {
		_, err := parseConfig([]byte(data))
		t.Logf("%s: %v", name, err)
		if err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}
