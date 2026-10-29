package main

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"go.yaml.in/yaml/v3"
)

// config says which requests are intercepted and where they are sent.
type config struct {
	// Gateway is the base URL rewritten requests are sent to.
	Gateway string `yaml:"gateway"`
	// KeyHeader carries the client's key, the password it authenticated to
	// the proxy with, to the gateway.
	KeyHeader string `yaml:"keyHeader"`
	// HostHeader carries the host a rewritten request was sent to, for the
	// gateway to route on.
	HostHeader string  `yaml:"hostHeader"`
	Routes     []route `yaml:"routes"`

	gateway *url.URL
}

// route sends requests for some paths on one host to the gateway. CONNECTs
// to the host are intercepted; requests for its other paths go to the origin
// unchanged.
type route struct {
	Host string `yaml:"host"`
	// Paths matched exactly, mapped to the gateway path they are sent to.
	Paths map[string]string `yaml:"paths"`
}

func loadConfig(file string) (*config, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	return parseConfig(data)
}

func parseConfig(data []byte) (*config, error) {
	c := &config{
		KeyHeader:  "x-agentgateway-key",
		HostHeader: "x-agent-proxy-host",
	}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(c); err != nil {
		return nil, err
	}

	if c.Gateway == "" {
		return nil, errors.New("gateway is required")
	}
	gw, err := url.Parse(c.Gateway)
	if err != nil || (gw.Scheme != "http" && gw.Scheme != "https") || gw.Host == "" {
		return nil, fmt.Errorf("gateway %q is not an http(s) URL", c.Gateway)
	}
	c.gateway = gw
	if c.KeyHeader == "" || c.HostHeader == "" {
		return nil, errors.New("keyHeader and hostHeader may not be empty")
	}

	seen := map[string]bool{}
	for i := range c.Routes {
		r := &c.Routes[i]
		r.Host = strings.ToLower(r.Host)
		if r.Host == "" || strings.ContainsAny(r.Host, ":/") {
			return nil, fmt.Errorf("route %d: host %q must be a bare hostname", i, r.Host)
		}
		if seen[r.Host] {
			return nil, fmt.Errorf("route %d: host %s listed twice", i, r.Host)
		}
		seen[r.Host] = true
		if len(r.Paths) == 0 {
			return nil, fmt.Errorf("route %s: no paths", r.Host)
		}
		paths := make(map[string]string, len(r.Paths))
		for from, to := range r.Paths {
			if !strings.HasPrefix(from, "/") || !strings.HasPrefix(to, "/") {
				return nil, fmt.Errorf("route %s: paths must start with /: %s: %s",
					r.Host, from, to)
			}
			paths[strings.TrimSuffix(from, "/")] = to
		}
		r.Paths = paths
	}
	return c, nil
}

// intercepts reports whether CONNECTs to host are intercepted.
func (c *config) intercepts(host string) bool {
	for _, r := range c.Routes {
		if r.Host == strings.ToLower(host) {
			return true
		}
	}
	return false
}

// gatewayPath returns the gateway path for req, or "" when req goes to the
// origin.
func (c *config) gatewayPath(req *http.Request) string {
	for _, r := range c.Routes {
		if r.Host == strings.ToLower(req.URL.Hostname()) {
			return r.Paths[strings.TrimSuffix(req.URL.Path, "/")]
		}
	}
	return ""
}

func isWebsocket(req *http.Request) bool {
	return strings.EqualFold(req.Header.Get("Upgrade"), "websocket")
}

// rewrite points req at the gateway when a route matches it, and adds the
// client's key. It reports whether req was rewritten.
func (c *config) rewrite(req *http.Request, key string) bool {
	path := c.gatewayPath(req)
	if path == "" {
		return false
	}
	req.Header.Set(c.HostHeader, req.URL.Hostname())
	req.URL.Scheme = c.gateway.Scheme
	req.URL.Host = c.gateway.Host
	req.URL.Path = strings.TrimSuffix(c.gateway.Path, "/") + path
	req.URL.RawPath = ""
	req.Host = c.gateway.Host
	req.Header.Set(c.KeyHeader, key)
	return true
}
