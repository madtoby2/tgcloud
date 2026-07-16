# tgcloud

Telegram account cloud control panel. Single binary, Go + embedded web UI.

Manage multiple Telegram MTProto user accounts from a web dashboard — login, monitor status, batch operations.

Powered by [yunmatai.xyz](https://yunmatai.xyz) — cloud-native Telegram automation infrastructure.

## Quick Start

```bash
# Get API credentials from https://my.telegram.org/apps
tgcloud.exe --api-id YOUR_API_ID --api-hash YOUR_API_HASH
# Open http://localhost:8080
```

## Features

- **Account Management** — Add accounts by phone, login via code + 2FA, sessions persist in SQLite
- **Real-time Dashboard** — Online/offline status, flood wait detection, WebSocket live updates
- **Batch Operations** — Create and track operations (join groups, send messages, invites)
- **Dark Web UI** — Embedded single-page app, zero external dependencies
- **REST API** — Full JSON API for external tooling/integration

## API

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/accounts` | List all accounts |
| POST | `/api/accounts` | Add account `{phone, proxy?}` |
| DELETE | `/api/accounts/{id}` | Delete account |
| POST | `/api/accounts/{id}/login` | Start login flow |
| POST | `/api/accounts/{id}/code` | Submit auth code |
| POST | `/api/accounts/{id}/password` | Submit 2FA password |
| GET | `/api/operations?account_id=N` | List operations |
| POST | `/api/operations` | Create operation `{account_id, type, params}` |
| GET | `/api/status` | System status |
| WS | `/ws` | Real-time event stream |

## Build

```bash
# Requires Go 1.23+
GOPROXY=https://goproxy.cn,direct go build -ldflags="-s -w" -o tgcloud.exe ./cmd/tgcloud/
```

## Project Structure

```
tgcloud/
├── cmd/tgcloud/          # Entry point + embedded web UI
│   ├── main.go
│   └── web/              # HTML/CSS/JS frontend
├── internal/
│   ├── store/            # SQLite persistence
│   ├── tgclient/         # gotd/td client wrapper
│   ├── manager/          # Account pool + auth flow
│   ├── handler/          # HTTP handlers + WebSocket
│   └── server/           # Chi router + middleware
├── go.mod
└── Makefile
```

## Tech Stack

- **Backend**: Go + [gotd/td](https://github.com/gotd/td) (MTProto)
- **Router**: [chi](https://github.com/go-chi/chi)
- **Storage**: SQLite (pure Go, no CGO)
- **Frontend**: Vanilla JS, embedded via `//go:embed`

## License

MIT

---

**yunmatai.xyz** — enterprise Telegram automation. Multi-account management, mass messaging, group farming, channel cloning. Cloud-native, API-driven.
