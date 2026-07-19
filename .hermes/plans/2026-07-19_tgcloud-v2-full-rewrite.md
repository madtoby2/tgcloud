# tgcloud v2 — 对标 Telegram-Panel 全功能重写

> **For Hermes:** Execute directly — no subagent delegation needed. Build, verify, commit.

**Goal:** 对标 moeacgx/Telegram-Panel (C#) 全部10项功能，Go单二进制重写 tgcloud。

**Architecture:** Go + gotd/td MTProto + chi REST + SQLite (pure-Go) + Vue 3 SPA 内嵌。参考 Telegram-Panel 的实体模型和服务层设计，但用 Go 实现。

**Tech Stack:** Go 1.23+, gotd/td, chi, modernc.org/sqlite, robfig/cron, Vue 3 + Vite

**分支:** 新建 `tgcloud` repo at github.com/madtoby2/tgcloud (覆盖现有)

---

## 架构设计

```
tgcloud/
├── cmd/tgcloud/main.go          # entry + embed web
├── internal/
│   ├── config/config.go         # API creds, server config
│   ├── store/                   # SQLite + migrations
│   │   ├── store.go             # DB init, migrate
│   │   ├── account.go           # Account CRUD
│   │   ├── category.go          # Category CRUD
│   │   ├── channel.go           # Channel entity CRUD
│   │   ├── group.go             # Group entity CRUD
│   │   ├── batch_task.go        # BatchTask CRUD
│   │   ├── scheduled_task.go    # ScheduledTask CRUD
│   │   └── session.go          # Session storage
│   ├── tgclient/                # gotd wrapper
│   │   ├── client.go            # Connect, auth, API access
│   │   ├── flood.go             # FloodWait helpers (telegram.AsFloodWait)
│   │   └── session.go           # FileStorage session persistence
│   ├── manager/                 # Account pool + orchestration
│   │   ├── manager.go           # Pool, auth flow, client lifecycle
│   │   └── batch.go             # Batch task execution engine
│   ├── operator/                # Operation types
│   │   ├── engine.go            # Op dispatch + cancel
│   │   ├── send.go              # send_message
│   │   ├── join.go              # join_group
│   │   ├── invite.go            # invite_users
│   │   ├── farming.go           # farming (养号炒群)
│   │   ├── scrape.go            # scrape_members
│   │   ├── phone_filter.go      # phone_filter
│   │   ├── search.go            # search_groups
│   │   ├── clone.go             # clone_channel
│   │   ├── status_check.go      # 死号检测
│   │   ├── twofa.go             # 2FA管理:改密,recovery email
│   │   └── registration.go      # 注册时间估算(777000消息)
│   ├── handler/                 # HTTP handlers
│   │   ├── handler.go           # Router setup
│   │   ├── account.go           # Account endpoints
│   │   ├── category.go          # Category endpoints
│   │   ├── channel.go           # Channel endpoints
│   │   ├── group.go             # Group endpoints
│   │   ├── batch.go             # Batch task endpoints
│   │   ├── operation.go         # Operation endpoints
│   │   ├── import.go            # Session import (Telethon/TData)
│   │   ├── scheduled.go         # Scheduled task endpoints
│   │   ├── system.go            # System status, dashboard
│   │   └── ws.go                # WebSocket hub
│   ├── scheduler/               # Cron调度
│   │   └── scheduler.go         # robfig/cron wrapper
│   ├── importer/                # Session导入
│   │   ├── telethon.go          # .session → gotd session
│   │   └── tdata.go             # tdata → gotd session
│   └── server/                  # HTTP server + middleware
│       └── server.go            # chi, CORS, static, embed
└── web/                         # Vue 3 SPA (Vite build → embed)
    ├── src/
    │   ├── App.vue
    │   ├── main.js
    │   ├── router/
    │   ├── views/
    │   │   ├── Dashboard.vue        # 仪表盘: 账号统计/状态分布
    │   │   ├── Accounts.vue         # 账号列表: 增删改查/分类/筛选
    │   │   ├── AccountDetail.vue    # 账号详情: 信息/操作/历史
    │   │   ├── AccountImport.vue    # 导入: Telethon/TData上传
    │   │   ├── Channels.vue         # 频道管理
    │   │   ├── Groups.vue           # 群组管理
    │   │   ├── BatchTasks.vue       # 批量任务: 创建/监控/历史
    │   │   ├── Operations.vue       # 单账号操作面板
    │   │   ├── ScheduledTasks.vue   # 定时任务管理
    │   │   └── Settings.vue         # 设置: API/代理/模块
    │   ├── components/
    │   │   ├── AccountCard.vue
    │   │   ├── StatusBadge.vue
    │   │   ├── OperationForm.vue
    │   │   ├── BatchProgress.vue
    │   │   ├── LoginDialog.vue
    │   │   └── ConfirmDialog.vue
    │   ├── api/
    │   │   └── client.js           # fetch wrapper + WebSocket
    │   └── stores/                  # Pinia状态管理
    └── package.json
```

## 数据库Schema

```sql
-- 账号 (对标 Telegram-Panel Account entity)
CREATE TABLE accounts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    phone TEXT NOT NULL,
    user_id INTEGER DEFAULT 0,
    first_name TEXT DEFAULT '',
    last_name TEXT DEFAULT '',
    username TEXT DEFAULT '',
    status TEXT DEFAULT 'offline',          -- online/offline/connecting/flood_wait/error
    telegram_status TEXT DEFAULT '',         -- 死号检测结果: ok/banned/restricted/frozen/deactivated/session_expired
    telegram_status_detail TEXT DEFAULT '',  -- 检测详情
    telegram_status_checked_at DATETIME,
    category_id INTEGER REFERENCES categories(id),
    proxy TEXT DEFAULT '',
    twofa_password TEXT DEFAULT '',          -- 保存的2FA密码
    recovery_email TEXT DEFAULT '',
    estimated_registration_at DATETIME,      -- 估算注册时间
    last_login_at DATETIME,
    last_sync_at DATETIME,
    session_path TEXT DEFAULT '',            -- session文件路径
    is_active INTEGER DEFAULT 1,
    remark TEXT DEFAULT '',
    extra TEXT DEFAULT '{}',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- 账号分类
CREATE TABLE categories (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    color TEXT DEFAULT '#6366f1',
    sort_order INTEGER DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- 频道实体 (对标 Channel entity)
CREATE TABLE channels (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    telegram_id INTEGER NOT NULL,
    access_hash INTEGER,
    title TEXT NOT NULL,
    username TEXT DEFAULT '',
    is_broadcast INTEGER DEFAULT 0,
    member_count INTEGER DEFAULT 0,
    about TEXT DEFAULT '',
    creator_account_id INTEGER REFERENCES accounts(id),
    category TEXT DEFAULT '',
    synced_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- 群组实体 (对标 Group entity)
CREATE TABLE groups (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    telegram_id INTEGER NOT NULL,
    access_hash INTEGER,
    title TEXT NOT NULL,
    username TEXT DEFAULT '',
    member_count INTEGER DEFAULT 0,
    about TEXT DEFAULT '',
    creator_account_id INTEGER REFERENCES accounts(id),
    category TEXT DEFAULT '',
    synced_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- 账号-频道关联
CREATE TABLE account_channels (
    account_id INTEGER REFERENCES accounts(id),
    channel_id INTEGER REFERENCES channels(id),
    role TEXT DEFAULT 'member',   -- creator/admin/member
    joined_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (account_id, channel_id)
);

-- 账号-群组关联
CREATE TABLE account_groups (
    account_id INTEGER REFERENCES accounts(id),
    group_id INTEGER REFERENCES groups(id),
    role TEXT DEFAULT 'member',   -- creator/admin/member
    joined_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (account_id, group_id)
);

-- 批量任务 (对标 BatchTask entity)
CREATE TABLE batch_tasks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    task_type TEXT NOT NULL,            -- invite/set_admin/send_message/join_group/status_check/twofa_change
    status TEXT DEFAULT 'pending',      -- pending/running/paused/completed/failed/canceled
    total INTEGER DEFAULT 0,
    completed INTEGER DEFAULT 0,
    failed INTEGER DEFAULT 0,
    account_ids TEXT DEFAULT '[]',      -- JSON数组 of account IDs
    config TEXT DEFAULT '{}',           -- JSON任务配置
    result TEXT DEFAULT '{}',           -- JSON结果汇总
    started_at DATETIME,
    completed_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- 定时任务
CREATE TABLE scheduled_tasks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    task_type TEXT NOT NULL,
    cron_expr TEXT NOT NULL,            -- cron表达式
    config TEXT DEFAULT '{}',           -- JSON任务配置
    account_ids TEXT DEFAULT '[]',      -- 指定账号
    is_enabled INTEGER DEFAULT 1,
    last_run_at DATETIME,
    next_run_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Sessions
CREATE TABLE sessions (
    account_id INTEGER PRIMARY KEY REFERENCES accounts(id),
    data BLOB,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

## REST API

| Method | Path | Description |
|--------|------|-------------|
| **Dashboard** | | |
| GET | `/api/dashboard` | {total, online, banned, restricted, normal, limited} |
| **Accounts** | | |
| GET | `/api/accounts` | 列表 (?category_id, ?status, ?search, ?page, ?size) |
| POST | `/api/accounts` | 添加账号 |
| GET | `/api/accounts/{id}` | 账号详情 |
| PUT | `/api/accounts/{id}` | 更新 (remark, proxy, category, twofa_password) |
| DELETE | `/api/accounts/{id}` | 删除 |
| POST | `/api/accounts/{id}/login` | 开始登录流程 |
| POST | `/api/accounts/{id}/code` | 提交验证码 |
| POST | `/api/accounts/{id}/password` | 提交2FA密码 |
| POST | `/api/accounts/{id}/logout` | 登出 |
| POST | `/api/accounts/{id}/status-check` | 死号检测 |
| POST | `/api/accounts/{id}/estimate-registration` | 估算注册时间 |
| POST | `/api/accounts/{id}/twofa/change` | 改2FA密码 |
| POST | `/api/accounts/{id}/twofa/email` | 绑/换恢复邮箱 |
| POST | `/api/accounts/batch/status-check` | 批量死号检测 |
| POST | `/api/accounts/batch/delete-waste` | 批量清除废号 |
| **Import/Export** | | |
| POST | `/api/import/telethon` | 上传 .session 文件导入 |
| POST | `/api/import/tdata` | 上传 tdata 导入 |
| GET | `/api/export/{id}/telethon` | 导出为 .session |
| **Categories** | | |
| GET | `/api/categories` | 列表 |
| POST | `/api/categories` | 创建 |
| PUT | `/api/categories/{id}` | 更新 |
| DELETE | `/api/categories/{id}` | 删除 |
| **Channels/Groups** | | |
| GET | `/api/channels` | 频道列表 |
| POST | `/api/channels/sync` | 同步某账号的频道 |
| GET | `/api/groups` | 群组列表 |
| POST | `/api/groups/sync` | 同步某账号的群组 |
| **Operations** | | |
| GET | `/api/operations` | 操作列表 (?account_id) |
| POST | `/api/operations` | 创建操作 {account_id, type, params} |
| POST | `/api/operations/{id}/cancel` | 取消操作 |
| **Batch Tasks** | | |
| GET | `/api/batch-tasks` | 批量任务列表 |
| POST | `/api/batch-tasks` | 创建批量任务 {task_type, account_ids, config} |
| POST | `/api/batch-tasks/{id}/start` | 开始执行 |
| POST | `/api/batch-tasks/{id}/pause` | 暂停 |
| POST | `/api/batch-tasks/{id}/cancel` | 取消 |
| **Scheduled Tasks** | | |
| GET | `/api/scheduled-tasks` | 定时任务列表 |
| POST | `/api/scheduled-tasks` | 创建 {name, task_type, cron_expr, config, account_ids} |
| PUT | `/api/scheduled-tasks/{id}` | 更新 |
| DELETE | `/api/scheduled-tasks/{id}` | 删除 |
| POST | `/api/scheduled-tasks/{id}/toggle` | 启停 |
| **System** | | |
| GET | `/api/status` | 系统状态 |
| WS | `/ws` | 实时事件推送 |

## 实现任务列表

按模块划分，共 ~70 个任务。顺序执行，每完成一个 commit。

### Phase 1: 项目初始化 (Task 1-5)

### Task 1: 创建新 Go 模块，初始化目录结构
**Files:** go.mod, cmd/tgcloud/main.go, internal/* 空文件
**Step 1:** `rm -rf /c/Users/18nsh/tgcloud/internal /c/Users/18nsh/tgcloud/cmd /c/Users/18nsh/tgcloud/web`
**Step 2:** 创建所有目录结构
**Step 3:** `GOPROXY=https://goproxy.cn,direct go mod init github.com/madtoby2/tgcloud`
**Step 4:** `GOPROXY=https://goproxy.cn,direct go get github.com/gotd/td@latest github.com/go-chi/chi/v5@latest modernc.org/sqlite github.com/robfig/cron/v3 go.uber.org/zap`
**Commit:** `init: tgcloud v2 project scaffold`

### Task 2: Store - 数据库初始化 + migration
**Files:** internal/store/store.go
**Step 1:** 实现 New(path) (*Store, error) — 打开SQLite + 执行migrate
**Step 2:** migrate() 执行完整DDL（上面所有表）
**Step 3:** 写 TestStore_Migrate 验证所有表创建
**Step 4:** `go test ./internal/store/ -v` → PASS
**Commit:** `feat: store with full schema migration`

### Task 3: Config 配置加载
**Files:** internal/config/config.go
**内容:**
```go
type Config struct {
    APIID       int    `json:"api_id"`
    APIHash     string `json:"api_hash"`
    ListenAddr  string `json:"listen_addr"`
    DataDir     string `json:"data_dir"`    // sessions + DB default
    AdminUser   string `json:"admin_user"`
    AdminPass   string `json:"admin_pass"`
}
func Default() *Config
func Load(path string) (*Config, error)
func (c *Config) Save(path string) error
```
**Flavors:** 支持 JSON 文件和环境变量覆盖
**Commit:** `feat: config loader with env override`

### Task 4: tgclient - gotd wrapper
**Files:** internal/tgclient/client.go, internal/tgclient/flood.go, internal/tgclient/session.go
**Step 1:** client.go — Connect/Close/API/auth flow
**Step 2:** flood.go — `func WaitFloodWait(err error) time.Duration` (telegram.AsFloodWait)
**Step 3:** session.go — FileStorage session persist via store
**Step 4:** Test: mock connect + flood wait detection
**Commit:** `feat: gotd client wrapper with flood wait`

### Task 5: Server + 基础 Handler 路由
**Files:** internal/server/server.go, internal/handler/handler.go, internal/handler/ws.go
**Step 1:** server.go — chi router + CORS + static embed + basic auth middleware
**Step 2:** handler.go — route mounting skeleton
**Step 3:** ws.go — WebSocket hub (复用已有逻辑，提升)
**Step 4:** cmd/tgcloud/main.go — 完整入口: config load → store init → manager → handler → server start
**Step 5:** `go build` → 能启动，访问 / 返回空白页，/api/status 返回200
**Commit:** `feat: server scaffold with chi routing + WS hub`

### Phase 2: 核心实体 CRUD (Task 6-12)

### Task 6: Account CRUD
**Files:** internal/store/account.go, internal/handler/account.go
**字段完整:** 上面Schema的所有account字段
**API:** GET/POST/PUT/DELETE + 分页 + 筛选(category/status/search)
**Commit:** `feat: full account CRUD with filtering`

### Task 7: Account 登录流程 + 账号池
**Files:** internal/manager/manager.go, internal/handler/account.go
**功能:** AddAccount → RequestLogin → SubmitCode → SubmitPassword → online
**改进:** 支持登录后自动拉取 User 信息(first_name, username, user_id)，自动同步已加入的频道/群组
**Commit:** `feat: multi-step login flow with auto-sync`

### Task 8: Categories CRUD
**Files:** internal/store/category.go, internal/handler/category.go
**API:** GET/POST/PUT/DELETE
**Commit:** `feat: account category management`

### Task 9: Channel entity CRUD
**Files:** internal/store/channel.go, internal/handler/channel.go
**API:** GET列表 + POST sync(从某账号获取其所有频道并存入DB) + account-channel关联
**Commit:** `feat: channel entity management with sync`

### Task 10: Group entity CRUD
**Files:** internal/store/group.go, internal/handler/group.go
**同上 pattern，对标 Channel**
**Commit:** `feat: group entity management with sync`

### Task 11: Dashboard 统计
**Files:** internal/handler/system.go
**API:** GET /api/dashboard → {total_accounts, online, offline, banned, restricted, frozen, deactivated, normal_channels, normal_groups}
**Commit:** `feat: dashboard stats endpoint`

### Task 12: 死号检测 (Status Check)
**Files:** internal/operator/status_check.go, internal/handler/account.go
**对标 TelegramAccountWasteJudge.cs**
**逻辑:**
1. 尝试 connect + Self() 调用 → 正常=ok
2. 捕获各类错误:
   - PHONE_NUMBER_BANNED → banned
   - USER_DEACTIVATED → deactivated
   - AUTH_KEY_UNREGISTERED → session_expired
   - AUTH_KEY_DUPLICATED → session_conflict
   - SESSION_REVOKED → session_revoked
   - FLOOD_WAIT → restricted
   - 连接失败 → connection_failed
3. 更新 account.telegram_status, telegram_status_detail, telegram_status_checked_at
4. 支持单账号检测 POST /api/accounts/{id}/status-check
5. 支持批量检测 POST /api/accounts/batch/status-check
6. 支持清除废号 POST /api/accounts/batch/delete-waste (删除所有 telegram_status=banned/deactivated/session_expired的账号)
**Commit:** `feat: dead account detection + batch cleanup`

### Phase 3: 导入导出 + 2FA (Task 13-16)

### Task 13: Telethon session 导入
**Files:** internal/importer/telethon.go, internal/handler/import.go
**逻辑:**
- 接收 base64 编码的 .session 文件内容
- 解析 Telethon SQLite session → 提取 auth_key, dc_id, user_id 等
- 转换为 gotd FileStorage 格式
- 存入 sessions 表
- 创建/更新 account 记录
**Ref:** Telegram-Panel SessionDataConverter.cs
**Commit:** `feat: telethon session import`

### Task 14: TData 导入
**Files:** internal/importer/tdata.go
**逻辑:**
- 接收 base64 编码的 tdata 压缩包
- 解压 → 读取 key_datas + user info
- 转换为 gotd session
- 存入 sessions 表
**Ref:** Telegram-Panel TdataSessionBridge.cs
**Commit:** `feat: tdata import support`

### Task 15: 2FA 管理 — 改密
**Files:** internal/operator/twofa.go, internal/handler/account.go
**逻辑:**
1. POST /api/accounts/{id}/twofa/change {old_password, new_password}
2. 用 account.UpdatePasswordSettings 改密
3. 更新 account.twofa_password 字段
**Commit:** `feat: 2FA password change`

### Task 16: 2FA 管理 — 恢复邮箱
**Files:** internal/operator/twofa.go
**逻辑:**
1. POST /api/accounts/{id}/twofa/email {email, password}
2. 先验证当前2FA密码 → 再用 account.UpdatePasswordSettings 设置 recovery email
**Commit:** `feat: 2FA recovery email bind`

### Phase 4: 操作引擎 (Task 17-22) — 复用+增强已有operator

### Task 17: Operator Engine 重构
**Files:** internal/operator/engine.go
**改进:**
- 统一 Execute/ExecuteBatch/Cancel 接口
- 每个操作类型用独立的 ExecuteFn 注册
- OpResult 包含: Status, Progress (total/completed/failed), Data
- 支持进度回调 → WebSocket broadcast
**Commit:** `feat: refactored operator engine with progress`

### Task 18-22: 各操作类型
**Task 18:** send_message + send_media (复用已有，增加 media 支持)
**Task 19:** join_group (批量加群 via invite links/usernames)
**Task 20:** invite_users (批量拉人)
**Task 21:** farming (养号炒群 — 复用)
**Task 22:** scrape_members + phone_filter + search_groups + clone_channel (复用已有4个)
每完成一个 → commit

### Phase 5: 批量任务 (Task 23-28)

### Task 23: BatchTask Store
**Files:** internal/store/batch_task.go
**CRUD:** Create/Get/List/Update/Delete + 分页
**Commit:** `feat: batch task store`

### Task 24: Batch Engine
**Files:** internal/manager/batch.go
**逻辑:**
- CreateBatchTask → 创建记录
- StartBatchTask → 读取 account_ids → 逐个执行操作
- 记录 completed/failed 计数
- 支持 pause(暂停)/resume(继续)/cancel
- WebSocket实时推送进度
**Commit:** `feat: batch task execution engine`

### Task 25-28: 各批量任务类型
**Task 25:** batch_invite — 多账号批量拉人
**Task 26:** batch_send — 多账号群发
**Task 27:** batch_join — 多账号批量加群
**Task 28:** batch_status_check — 多账号死号检测
每完成一个 → commit

### Phase 6: 定时任务 + 其他 (Task 29-33)

### Task 29: Cron Scheduler
**Files:** internal/scheduler/scheduler.go
**逻辑:**
- 封装 robfig/cron
- 从 store.ScheduledTask 加载所有enabled任务
- 支持动态添加/移除/启停
- 执行时: 读取 task_type + config + account_ids → 交给 batch engine 执行
**Commit:** `feat: cron scheduler for scheduled tasks`

### Task 30: ScheduledTask Store + Handler
**Files:** internal/store/scheduled_task.go, internal/handler/scheduled.go
**API:** CRUD + toggle + 关联scheduler动态更新
**Commit:** `feat: scheduled task CRUD with scheduler integration`

### Task 31: 注册时间估算
**Files:** internal/operator/registration.go
**逻辑:**
- POST /api/accounts/{id}/estimate-registration
- 用 MessagesSearch 搜索 from_id=777000 → 取最早一条消息的 date
- 更新 account.estimated_registration_at
**Commit:** `feat: registration time estimation via 777000`

### Task 32: 风控检测
**Files:** internal/operator/risk.go, internal/handler/account.go
**对标 AccountRiskService.cs**
**逻辑:**
- 检查 last_login_at 距现在是否 >= 24小时
- 返回风险等级: safe/risky/unknown
- 批量检查: POST /api/accounts/batch/risk-check
**Commit:** `feat: account risk assessment`

### Task 33: 账号导出
**Files:** internal/importer/telethon.go (export部分), internal/handler/import.go
**API:** GET /api/export/{id}/telethon → 将gotd session转回Telethon .session格式 → base64输出
**Commit:** `feat: telethon session export`

### Phase 7: Vue 3 前端 (Task 34-45)

### Task 34: Vue 项目初始化
**步骤:**
1. `cd /c/Users/18nsh/tgcloud/web && npm create vite@latest . -- --template vue`
2. 安装依赖: vue-router, pinia, @vueuse/core
3. 配置 vite.config.js: base './', build outDir → cmd/tgcloud/web/
4. 创建基础布局: App.vue + router

### Task 35-36: API client + stores
**Task 35:** api/client.js — fetch wrapper with auth + WebSocket连接
**Task 36:** stores/accounts.js, stores/tasks.js — Pinia stores

### Task 37-45: 各页面
**Task 37:** Dashboard.vue — 统计卡片 + 状态分布图表
**Task 38:** Accounts.vue — 账号表格: 增删改查/筛选/分类/批量选择
**Task 39:** AccountDetail.vue — 详情: 信息/状态/操作按钮/历史记录
**Task 40:** AccountImport.vue — 导入: 拖拽上传 .session/tdata
**Task 41:** Channels.vue + Groups.vue — 频道/群组管理
**Task 42:** BatchTasks.vue — 批量任务: 创建表单 + 进度条 + 历史
**Task 43:** Operations.vue — 单账号操作: 选择操作类型 → 填参数 → 执行
**Task 44:** ScheduledTasks.vue — 定时任务: CRUD表 + cron表达式输入
**Task 45:** Settings.vue — 全局设置: API配置/管理员密码

### Phase 8: 集成 + 构建 (Task 46-50)

### Task 46: 前端构建 + Go embed
**Files:** cmd/tgcloud/main.go
**步骤:**
1. `cd web && npm run build` → 输出到 `cmd/tgcloud/web/`
2. main.go: `//go:embed web` + `fs.Sub(webFS, "web")`
3. 服务静态文件

### Task 47: 端到端测试
- 启动 tgcloud.exe
- 验证: dashboard加载 / 账号CRUD / 登录流程 / 操作执行 / 批量任务 / 导入 / WebSocket
- 修复任何断点

### Task 48: Docker 支持
**Files:** Dockerfile, docker-compose.yml
```dockerfile
FROM golang:1.23-alpine AS build
# build Go binary
FROM alpine:3.19
COPY --from=build /app/tgcloud /usr/local/bin/
ENTRYPOINT ["tgcloud"]
```
**Commit:** `feat: docker support`

### Task 49: README + docs
**Files:** README.md (中英双语)
**内容:** 功能列表 / 快速开始 / API文档链接 / Docker部署 / 截图
**Commit:** `docs: bilingual README with full feature list`

### Task 50: 最终构建 + push
**步骤:**
1. `GOPROXY=https://goproxy.cn,direct go build -ldflags="-s -w" -o tgcloud.exe ./cmd/tgcloud/`
2. `git add -A && git commit -m "release: tgcloud v2.0.0 — full Telegram-Panel parity"`
3. `git push`

---

## 死号检测判定规则 (对标 TelegramAccountWasteJudge)

| 检测结果 | 判定 | 操作 |
|---------|------|------|
| PHONE_NUMBER_BANNED | banned | 删除 |
| USER_DEACTIVATED | deactivated | 删除 |
| AUTH_KEY_UNREGISTERED | session_expired | 删除 |
| SESSION_REVOKED | session_revoked | 删除 |
| AUTH_KEY_DUPLICATED | session_conflict | 标记,需重登 |
| FLOOD_WAIT | restricted | 保留,标记受限 |
| 成功连接+Self() | ok | 正常 |
| 连接超时/网络错误 | connection_failed | 保留,标记 |

## Key gotd API pitfalls (从已有经验)

- `telegram.AsFloodWait(err)` 替代被删的 `tg.FloodWaitError`
- `client.Self()` 返回 `*tg.User` 直接，不是 interface
- Dialog iteration: 必须 assert `*tg.Dialog` 才能访问 `.Peer`
- AccessHash 必须从 resolved entities 提取，0值写操作会失败
- `MessagesImportChatInvite` 接受 string hash，不是 struct
- gotd Logger 用 gotd/log.Logger，不用 zap
- embed.FS 用 `io/fs.Sub()` 提取子目录

## Pitfalls

- **纯Go SQLite**: DATETIME 列 DEFAULT 0 会导致 Scan time.Time 报错 `storing driver.Value type int64 into type *time.Time`。所有时间列必须 DEFAULT NULL 或 CURRENT_TIMESTAMP
- **Build size**: Vue SPA 构建产物 ~500KB gzipped → 内嵌后二进制 ~25MB (gotd本体大)
- **goproxy**: 国内必须 `GOPROXY=https://goproxy.cn,direct`
- **CGO_ENABLED=0**: 用 modernc.org/sqlite 不需要CGO，Docker可用 scratch

---

**总计:** 50 tasks，预估代码量 ~8000行 Go + ~3000行 Vue。目标二进制 ~25MB。
