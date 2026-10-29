package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProxyAuth(t *testing.T) {
	ok := httptest.NewRequest(http.MethodConnect, "https://api.anthropic.com:443", nil)
	ok.Header.Set("Proxy-Authorization", "Basic YWxpY2U6YWd3X2tleQ==") // alice:agw_key
	id, valid := proxyAuth(ok)
	if !valid || id != (identity{User: "alice", Key: "agw_key"}) {
		t.Errorf("got %+v, %v", id, valid)
	}
	if ok.Header.Get("Proxy-Authorization") != "" {
		t.Error("Proxy-Authorization not removed")
	}

	invalid := []string{
		"",
		"Bearer agw_key",
		"Basic !!!",
		"Basic YWxpY2U=", // alice, no key
		"Basic YWxpY2U6", // empty key
	}
	for _, h := range invalid {
		req := httptest.NewRequest(http.MethodConnect, "https://api.anthropic.com:443", nil)
		if h != "" {
			req.Header.Set("Proxy-Authorization", h)
		}
		if _, valid := proxyAuth(req); valid {
			t.Errorf("%q accepted", h)
		}
	}
}
