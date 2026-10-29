// agent-proxy is an HTTP(S) proxy for coding agents. It intercepts requests
// to configured hosts and paths, such as their model requests to providers,
// and sends them to a gateway with the key the client authenticated to the
// proxy with. All other traffic passes through unchanged.
package main

import (
	"flag"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"

	"github.com/elazarl/goproxy"
)

func main() {
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".config", "agent-proxy")
	listen := flag.String("listen", "127.0.0.1:8899", "address to listen on")
	configFile := flag.String("config", filepath.Join(dir, "config.yaml"),
		"YAML file with the gateway and routes")
	caDir := flag.String("ca-dir", dir,
		"directory holding ca.crt and ca.key, generated if missing")
	verbose := flag.Bool("v", false, "verbose proxy logging")
	flag.Parse()

	cfg, err := loadConfig(*configFile)
	if err != nil {
		log.Fatalf("load config %s: %v", *configFile, err)
	}

	ca, err := loadCA(*caDir)
	if err != nil {
		log.Fatalf("load CA from %s: %v", *caDir, err)
	}

	proxy := goproxy.NewProxyHttpServer()
	proxy.Verbose = *verbose
	// Never send our own traffic through HTTPS_PROXY, which may point here.
	proxy.Tr = &http.Transport{
		Proxy:             nil,
		DialContext:       (&net.Dialer{}).DialContext,
		ForceAttemptHTTP2: true,
	}

	mitm := &goproxy.ConnectAction{
		Action:    goproxy.ConnectMitm,
		TLSConfig: goproxy.TLSConfigFromCA(&ca),
	}
	tunnel := &goproxy.ConnectAction{Action: goproxy.ConnectAccept}

	proxy.OnRequest().HandleConnectFunc(func(
		host string, ctx *goproxy.ProxyCtx,
	) (*goproxy.ConnectAction, string) {
		id, ok := proxyAuth(ctx.Req)
		if !ok {
			ctx.Resp = proxyAuthRequired(ctx.Req)
			return goproxy.RejectConnect, host
		}
		// Copied to the context of every request in an intercepted tunnel.
		ctx.UserData = id
		h, _, err := net.SplitHostPort(host)
		if err != nil {
			h = host
		}
		if cfg.intercepts(h) {
			return mitm, host
		}
		return tunnel, host
	})

	proxy.OnRequest().DoFunc(func(
		req *http.Request, ctx *goproxy.ProxyCtx,
	) (*http.Request, *http.Response) {
		id, ok := ctx.UserData.(identity)
		if !ok {
			// A plain HTTP proxy request, not inside a CONNECT tunnel.
			if id, ok = proxyAuth(req); !ok {
				return req, proxyAuthRequired(req)
			}
		}
		from := req.URL.Host + req.URL.Path
		// Websockets aren't sent to the gateway. Refuse the upgrade rather
		// than let it reach the origin past the gateway; Codex then falls
		// back to HTTPS.
		if isWebsocket(req) && cfg.gatewayPath(req) != "" {
			log.Printf("%s %s %s: websocket refused", id.User, req.Method, from)
			return req, goproxy.NewResponse(req, goproxy.ContentTypeText,
				http.StatusUpgradeRequired, "websocket not supported, use HTTPS\n")
		}
		if cfg.rewrite(req, id.Key) {
			log.Printf("%s %s %s -> %s", id.User, req.Method, from, req.URL.Path)
		}
		return req, nil
	})

	log.Printf("listening on %s, gateway %s, %d routes, CA %s",
		*listen, cfg.gateway, len(cfg.Routes), filepath.Join(*caDir, "ca.crt"))
	log.Fatal(http.ListenAndServe(*listen, proxy))
}
