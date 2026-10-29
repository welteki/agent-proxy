package main

import (
	"bytes"
	"io"
	"net/http"
	"strings"
)

// identity is a client of the proxy. Key is its gateway key, sent as
// the password of the proxy's Basic auth; User only labels it in the log.
type identity struct {
	User string
	Key  string
}

// proxyAuth reads the client's identity from Proxy-Authorization and removes
// the header so it is never forwarded.
func proxyAuth(req *http.Request) (identity, bool) {
	r := &http.Request{Header: http.Header{
		"Authorization": req.Header.Values("Proxy-Authorization"),
	}}
	req.Header.Del("Proxy-Authorization")
	user, key, ok := r.BasicAuth()
	if !ok || strings.TrimSpace(key) == "" {
		return identity{}, false
	}
	return identity{User: user, Key: key}, true
}

var unauthorizedMsg = []byte("407 Proxy Authentication Required\n")

func proxyAuthRequired(req *http.Request) *http.Response {
	return &http.Response{
		StatusCode: http.StatusProxyAuthRequired,
		ProtoMajor: 1,
		ProtoMinor: 1,
		Request:    req,
		Header: http.Header{
			"Proxy-Authenticate": {`Basic realm="agent-proxy"`},
			"Proxy-Connection":   {"close"},
		},
		Body:          io.NopCloser(bytes.NewReader(unauthorizedMsg)),
		ContentLength: int64(len(unauthorizedMsg)),
	}
}
