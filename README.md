# agent-proxy

A minimal HTTP(S) proxy that sends coding agents' model requests to an LLM
gateway such as [agentgateway](https://agentgateway.dev).

## Why

Many coding agents, especially when signed in with a subscription, can't be
pointed at an LLM gateway, so their model traffic bypasses it unless each agent
gets its own workaround.

Most agents do support `HTTPS_PROXY`. agent-proxy uses that to route all of
them to the gateway the same way, passing their own logins through.

## How it works

- Hosts listed in the config are intercepted with a local CA; other hosts are
  tunnelled.
- Configured paths on those hosts go to the gateway; all other requests go to
  the original server.
- The agent's credentials are passed through; the client's gateway key is added.

## Config

The gateway and routes are read from a YAML file, `~/.config/agent-proxy/config.yaml`
by default. [`config.example.yaml`](config.example.yaml) routes the model
requests of Claude Code, Codex, opencode and pi to agentgateway:

```yaml
gateway: https://agentgateway.example.com   # required
# keyHeader: x-agentgateway-key             # default
# hostHeader: x-agent-proxy-host            # default
routes:
  - host: chatgpt.com
    paths:
      # Matched exactly; the value is the path at the gateway.
      /backend-api/codex/responses: /v1/responses
```

Rewritten requests carry the host they were sent to in `hostHeader`, so the
gateway can route on it where the request alone is ambiguous: with agentgateway,
`gpt-*` from `api.openai.com` (pi) can go to the OpenAI API and other `gpt-*`
(Codex) to ChatGPT. The gateway should remove the header before forwarding.

Websocket upgrades on configured paths are refused with `426` rather than
passed to the origin. Codex then falls back to HTTPS immediately.

## Identity

The proxy holds no key. Clients authenticate to it with Basic proxy auth: the
password is the client's gateway key, sent to the gateway in `keyHeader`; the
username only labels requests in the proxy's log. Requests without credentials
get `407`. The key is checked by the gateway, not the proxy (with agentgateway,
a wrong key gets `401`), and is never sent to an origin.

## Usage

```sh
go build -o agent-proxy .    # or: nix build (result/bin/agent-proxy), nix run . -- -v
cp config.example.yaml ~/.config/agent-proxy/config.yaml   # then set gateway
./agent-proxy   # flags and defaults: -listen 127.0.0.1:8899
                #   -config ~/.config/agent-proxy/config.yaml -ca-dir ~/.config/agent-proxy -v
```

On first start a CA is generated in `~/.config/agent-proxy` (`ca.crt`,
`ca.key` with mode 0600). Clients must trust `ca.crt`:

```sh
export HTTPS_PROXY="http://alice:$(cat path/to/gateway.key)@127.0.0.1:8899"
export NODE_EXTRA_CA_CERTS=~/.config/agent-proxy/ca.crt    # Claude Code, opencode, pi
export CODEX_CA_CERTIFICATE=~/.config/agent-proxy/ca.crt   # Codex
```

Run the agents without other gateway settings: an agent that already points at
the gateway (e.g. `ANTHROPIC_BASE_URL`) is only tunnelled.

## Tested

Claude Code, Codex, opencode and pi, with the routes in `config.example.yaml`
and agentgateway 1.6.0.

## License

MIT, see [LICENSE](LICENSE).
