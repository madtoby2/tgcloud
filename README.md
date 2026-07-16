# tgcloud

Telegram account cloud control panel. Single binary, Go + embedded web UI.

Manage multiple Telegram MTProto user accounts from a web dashboard — login, monitor status, batch operations, farming, scraping, cloning.

Powered by [yunmatai.xyz](https://yunmatai.xyz) — cloud-native Telegram automation infrastructure.

## Quick Start

```bash
# Get API credentials from https://my.telegram.org/apps
tgcloud.exe --api-id YOUR_API_ID --api-hash YOUR_API_HASH
# Open http://localhost:8080
```

## Features

### Account Management
- Add accounts by phone, login via code + 2FA
- Session persistence in SQLite (survives restarts)
- Online/offline/flood_wait status, real-time WebSocket updates

### Cloud Control Operations (8 tools)
| Tool | Description |
|------|-------------|
| **Send Message** | 群发消息 — batch send to multiple targets |
| **Join Groups** | 批量加群 — join via invite links or usernames |
| **Invite Users** | 批量拉人 — invite users to your channel/group |
| **Farming** | 养号炒群 — scripted message rotation with random intervals, anti-spam noise |
| **Scrape Members** | 爬成员 — extract group member lists |
| **Phone Filter** | 手机号过滤 — check if phone numbers are registered on TG |
| **Search Groups** | 全局搜索 — find groups/users by keyword |
| **Clone Channel** | 频道克隆 — copy messages from source to target channel |

### Anti-Spam Protections
- FloodWait auto-detection and backoff
- Random intervals between actions
- Message noise injection (random emoji/whitespace) in farming mode
- Per-account rate limiting

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
| POST | `/api/operations/{id}/cancel` | Cancel running operation |
| GET | `/api/status` | System status (online/total accounts) |
| WS | `/ws` | Real-time event stream |

### Operation Types
`join_group` | `send_message` | `invite_users` | `farming` | `scrape_members` | `phone_filter` | `search_groups` | `clone_channel`

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
│   └── web/              # HTML/CSS/JS frontend (tabs, tools, dark theme)
├── internal/
│   ├── store/            # SQLite persistence (accounts, ops, sessions)
│   ├── tgclient/         # gotd/td client wrapper + MTProto helpers
│   ├── manager/          # Account pool, auth flow, op orchestration
│   ├── operator/         # Operation execution engine (all 8 op types)
│   ├── handler/          # HTTP handlers + WebSocket hub
│   └── server/           # Chi router + CORS middleware
├── go.mod
└── Makefile
```

## Tech Stack

- **Backend**: Go + [gotd/td](https://github.com/gotd/td) (MTProto)
- **Router**: [chi](https://github.com/go-chi/chi)
- **Storage**: SQLite (pure Go, no CGO)
- **Frontend**: Vanilla JS, embedded via `//go:embed`
- **Real-time**: WebSocket for live account status + operation updates

## License

MIT

---

**yunmatai.xyz** — enterprise Telegram automation. Multi-account management, mass messaging, group farming, channel cloning. Cloud-native, API-driven.
