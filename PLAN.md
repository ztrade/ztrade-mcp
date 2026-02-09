# ztrade MCP Server 实现计划

## 1. 项目概述

为 ztrade 量化交易框架实现一个 [Model Context Protocol (MCP)](https://modelcontextprotocol.io/) Server，使 AI 助手能够通过标准化协议调用 ztrade 的核心功能，包括：K线数据下载、策略回测、实盘交易管理、本地数据查询、策略构建等。

### 1.1 技术选型

| 项目 | 选择 | 说明 |
|------|------|------|
| MCP SDK | [mcp-go](https://github.com/mark3labs/mcp-go) | Go 语言 MCP SDK，支持 Tools/Resources/Prompts |
| 传输协议 | stdio (默认) + SSE (可选) | stdio 供本地 IDE 使用，SSE 供远程调用 |
| 配置管理 | viper | 与 ztrade 保持一致 |
| 日志 | logrus | 与 ztrade 保持一致 |
| 数据库 | 复用 ztrade 的 dbstore | 支持 sqlite/mysql/pg |

### 1.2 项目结构

```
ztrade-mcp/
├── main.go                  # 入口，启动 MCP Server
├── go.mod
├── go.sum
├── config.go                # 配置加载（复用 ztrade.yaml）
├── server.go                # MCP Server 初始化与注册
├── auth/
│   ├── auth.go              # 认证接口定义
│   ├── token.go             # Bearer Token 认证实现
│   ├── apikey.go            # API Key 认证实现
│   └── middleware.go        # HTTP 认证中间件
├── tools/
│   ├── download.go          # K线数据下载工具
│   ├── backtest.go          # 策略回测工具
│   ├── trade.go             # 实盘交易工具
│   ├── list.go              # 本地数据查询工具
│   ├── build.go             # 策略构建工具
│   ├── strategy.go          # 策略辅助工具(生成/分析)
│   └── kline.go             # K线数据查询工具
├── resources/
│   ├── config.go            # 暴露当前配置为 Resource
│   ├── strategy_doc.go      # 暴露策略文档为 Resource
│   └── report.go            # 暴露回测报告为 Resource
├── prompts/
│   ├── strategy.go           # 策略开发提示模板
│   └── backtest.go           # 回测分析提示模板
├── Dockerfile               # 多阶段构建 Docker 镜像
├── docker-compose.yml       # 一键部署编排
└── .dockerignore
```

---

## 2. 认证控制设计

mcp-go 支持多种认证方式，ztrade-mcp 将实现分层认证机制，覆盖 stdio 和 HTTP（SSE / Streamable HTTP）两种传输模式。

### 2.1 认证架构

```
┌─────────────────────────────────────────────────────┐
│                  MCP Client Request                 │
└──────────────┬──────────────────────────────────────┘
               │
       ┌───────▼───────┐
       │ Transport Layer│
       │  stdio / HTTP │
       └───────┬───────┘
               │
   ┌───────────▼───────────┐
   │  HTTP Auth Middleware │  ← Bearer Token / API Key 校验
   │  (仅 HTTP 模式生效)    │     401 拒绝未认证请求
   └───────────┬───────────┘
               │
   ┌───────────▼───────────┐
   │  WithHTTPContextFunc  │  ← 将认证信息注入 context
   │  (提取 user/role)     │
   └───────────┬───────────┘
               │
   ┌───────────▼───────────┐
   │   Tool Middleware     │  ← 按工具级别做权限控制
   │  (权限/角色检查)       │    如: 实盘交易需 admin 角色
   └───────────┬───────────┘
               │
       ┌───────▼───────┐
       │  Tool Handler  │
       └───────────────┘
```

### 2.2 认证模式

#### 模式 1：无认证（stdio 本地模式，默认）

stdio 模式下进程由本地 IDE 启动，天然隔离，无需认证。

```yaml
# ztrade.yaml
mcp:
  transport: stdio
  auth:
    enabled: false
```

#### 模式 2：Bearer Token 静态令牌（SSE / HTTP 模式）

适用于个人部署、内网使用，配置简单。

```yaml
mcp:
  transport: http        # 或 sse
  listen: ":8080"
  auth:
    enabled: true
    type: token
    tokens:
      - token: "sk-ztrade-xxxxxxxxxxxxxxxxxxxx"
        name: "admin"
        role: admin    # admin: 全部权限; reader: 只读(查询/回测); trader: 读+交易
      - token: "sk-ztrade-yyyyyyyyyyyyyyyyyyyy"
        name: "readonly-bot"
        role: reader
```

**实现要点**：
- 通过 HTTP 中间件拦截请求，校验 `Authorization: Bearer <token>` header
- 对于 SSE 模式，使用 `server.WithSSEContextFunc` 注入认证上下文
- 对于 Streamable HTTP 模式，使用 `server.WithHTTPContextFunc` 注入认证上下文
- Token 查找使用 O(1) map 匹配

#### 模式 3：API Key（HTTP Header / Query 参数）

适用于简单的 API 调用场景。

```yaml
mcp:
  transport: http
  listen: ":8080"
  auth:
    enabled: true
    type: apikey
    header: "X-API-Key"    # 也支持通过 query param ?api_key=xxx
    keys:
      - key: "ak-ztrade-xxxxxxxxxxxxxxxxxxxx"
        name: "bot-1"
        role: admin
```

### 2.3 角色权限控制 (RBAC)

| 角色 | list_data | query_kline | download_kline | run_backtest | build_strategy | create_strategy | start_trade | stop_trade | trade_status |
|------|-----------|-------------|----------------|--------------|----------------|-----------------|-------------|------------|-------------|
| **reader** | ✅ | ✅ | ❌ | ✅ | ❌ | ✅ | ❌ | ❌ | ✅ |
| **trader** | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| **admin** | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |

reader 与 trader 的区别：reader 不能执行会产生副作用的操作（下载数据、编译策略、交易）。

**实现方式**：通过 `MCPServer.AddToolMiddleware` 添加全局 Tool 中间件：

```go
mcpServer.AddToolMiddleware(func(next server.ToolHandler) server.ToolHandler {
    return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
        user := auth.UserFromContext(ctx)
        if user == nil && authEnabled {
            return nil, fmt.Errorf("authentication required")
        }
        if !auth.HasPermission(user.Role, req.Params.Name) {
            return nil, fmt.Errorf("permission denied: role '%s' cannot use tool '%s'", user.Role, req.Params.Name)
        }
        return next(ctx, req)
    }
})
```

### 2.4 认证相关代码实现

#### `auth/auth.go` — 核心接口

```go
package auth

type User struct {
    Name  string
    Role  string // "admin" | "trader" | "reader"
    Token string
}

type Authenticator interface {
    // Authenticate 从 HTTP 请求中验证身份，返回 User 或 error
    Authenticate(r *http.Request) (*User, error)
}

// UserFromContext 从 context 中提取 User
func UserFromContext(ctx context.Context) *User { ... }

// HasPermission 检查角色是否有权限使用指定工具
func HasPermission(role, toolName string) bool { ... }
```

#### `auth/middleware.go` — HTTP 认证中间件

```go
func AuthMiddleware(authenticator Authenticator) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            user, err := authenticator.Authenticate(r)
            if err != nil {
                http.Error(w, "Unauthorized", http.StatusUnauthorized)
                return
            }
            ctx := context.WithValue(r.Context(), userContextKey, user)
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}
```

---

## 3. Docker 支持

### 3.1 Dockerfile（多阶段构建）

```dockerfile
# ====== Build Stage ======
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git gcc musl-dev

WORKDIR /app

# 依赖缓存层
COPY go.mod go.sum ./
RUN go mod download

# 构建
COPY . .
RUN CGO_ENABLED=1 go build -ldflags="-s -w" -o /ztrade-mcp .

# ====== Runtime Stage ======
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /ztrade-mcp /usr/local/bin/ztrade-mcp

# 默认配置目录
RUN mkdir -p /etc/ztrade /data/ztrade

VOLUME ["/etc/ztrade", "/data/ztrade"]

# 默认暴露 HTTP 端口
EXPOSE 8080

ENTRYPOINT ["ztrade-mcp"]
CMD ["--config", "/etc/ztrade/ztrade.yaml", "--transport", "http", "--listen", ":8080"]
```

### 3.2 docker-compose.yml

```yaml
version: "3.8"

services:
  ztrade-mcp:
    build: .
    image: ztrade/ztrade-mcp:latest
    container_name: ztrade-mcp
    restart: unless-stopped
    ports:
      - "8080:8080"
    volumes:
      - ./configs/ztrade.yaml:/etc/ztrade/ztrade.yaml:ro
      - ztrade-data:/data/ztrade          # K线数据库持久化
      - ./strategies:/strategies:ro        # 策略文件挂载（只读）
    environment:
      - TZ=Asia/Shanghai
      - ZTRADE_LOG_LEVEL=info
    healthcheck:
      test: ["CMD", "wget", "--spider", "-q", "http://localhost:8080/health"]
      interval: 30s
      timeout: 5s
      retries: 3

  # 可选：使用 MySQL 替代 SQLite
  # mysql:
  #   image: mysql:8.0
  #   container_name: ztrade-mysql
  #   restart: unless-stopped
  #   environment:
  #     MYSQL_ROOT_PASSWORD: password
  #     MYSQL_DATABASE: exchange
  #   volumes:
  #     - mysql-data:/var/lib/mysql
  #   ports:
  #     - "3306:3306"

volumes:
  ztrade-data:
  # mysql-data:
```

### 3.3 .dockerignore

```
*.md
.git
.gitignore
.vscode
*.so
*.log
report.html
```

### 3.4 Docker 使用说明

#### 构建镜像

```bash
# 本地构建
docker build -t ztrade/ztrade-mcp:latest .

# 多平台构建（amd64 + arm64）
docker buildx build --platform linux/amd64,linux/arm64 \
  -t ztrade/ztrade-mcp:latest --push .
```

#### 快速启动

```bash
# 使用 docker run
docker run -d \
  --name ztrade-mcp \
  -p 8080:8080 \
  -v $(pwd)/configs/ztrade.yaml:/etc/ztrade/ztrade.yaml:ro \
  -v ztrade-data:/data/ztrade \
  ztrade/ztrade-mcp:latest

# 使用 docker-compose
docker compose up -d
```

#### 使用 SQLite（默认，适合个人使用）

数据库文件自动存储在 `/data/ztrade/` 目录，通过 volume 持久化。

配置文件中设置：
```yaml
db:
  type: sqlite
  uri: /data/ztrade/exchange.db
```

#### 使用 MySQL（适合多人/生产环境）

取消 docker-compose.yml 中 mysql 服务的注释，配置文件中设置：
```yaml
db:
  type: mysql
  uri: root:password@tcp(mysql:3306)/exchange
```

### 3.5 健康检查端点

MCP Server 在 HTTP 模式下额外提供 `/health` 端点：

```go
mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]string{
        "status": "ok",
        "version": Version,
    })
})
```

---

## 4. MCP Tools 设计（核心功能）

### 4.1 K线数据下载 — `download_kline`

**功能**：从交易所下载历史K线数据到本地数据库。

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| exchange | string | 是 | - | 交易所名称 (binance/okx) |
| symbol | string | 是 | - | 交易对 (如 BTCUSDT) |
| binSize | string | 否 | "1m" | K线周期 (1m/5m/15m/1h/1d) |
| start | string | 是 | - | 开始时间 "2020-01-01 08:00:00" |
| end | string | 是 | - | 结束时间 |
| auto | bool | 否 | false | 自动续接下载（从DB最新时间到当前） |

**返回**：下载状态、下载数量、时间范围。

**实现要点**：
- 复用 `ctl.NewDataDownload` / `ctl.NewDataDownloadAuto`
- 需要先通过 config 初始化 DB 和交易所配置

### 4.2 策略回测 — `run_backtest`

**功能**：使用指定策略脚本对历史数据进行回测。

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| script | string | 是 | - | 策略文件路径 (.go/.so) |
| exchange | string | 是 | - | 交易所名称 |
| symbol | string | 是 | - | 交易对 |
| start | string | 是 | - | 回测开始时间 |
| end | string | 是 | - | 回测结束时间 |
| balance | float | 否 | 100000 | 初始资金 |
| fee | float | 否 | 0.0001 | 手续费率 |
| lever | float | 否 | 1 | 杠杆倍数 |
| param | string | 否 | "" | 策略参数 JSON |

**返回**：`ReportResult` 结构化数据，包含：
- 总收益率、年化收益率、夏普比率、索提诺比率
- 最大回撤、胜率、盈亏比
- 交易次数（多/空）、综合得分
- 交易记录明细

**实现要点**：
- 复用 `ctl.NewBacktest` + `report.NewReportSimple`
- 回测完成后调用 `Backtest.Result()` 获取结构化结果
- 同步执行，使用 `Backtest.Run()` 而非 `Start()`

### 4.3 实盘交易管理 — `start_trade` / `stop_trade` / `trade_status`

#### 4.3.1 `start_trade` — 启动实盘

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| script | string | 是 | - | 策略文件路径 |
| exchange | string | 是 | - | 交易所名称 |
| symbol | string | 是 | - | 交易对 |
| param | string | 否 | "" | 策略参数 JSON |
| recentDays | int | 否 | 1 | 加载最近N天历史数据 |

**返回**：交易实例 ID、启动状态。

#### 4.3.2 `stop_trade` — 停止实盘

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| tradeId | string | 是 | 交易实例 ID |

**返回**：停止状态。

#### 4.3.3 `trade_status` — 查询实盘状态

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| tradeId | string | 否 | 不传则返回所有实例状态 |

**返回**：运行中的交易实例列表及其状态。

**实现要点**：
- 需要维护一个 `map[string]*ctl.Trade` 来管理多个交易实例
- 使用 `Trade.Start()` / `Trade.Stop()` 异步管理生命周期
- 交易实例 ID 使用 `exchange_symbol_timestamp` 格式

### 4.4 本地数据查询 — `list_data`

**功能**：列出本地数据库中已有的K线数据信息。

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| exchange | string | 否 | 过滤交易所 |
| symbol | string | 否 | 过滤交易对 |

**返回**：数据列表，每条包含交易所、交易对、K线周期、起止时间。

**实现要点**：
- 复用 `ctl.NewLocalData` + `ListAll()`

### 4.5 策略构建 — `build_strategy`

**功能**：将 Go 策略源码编译为 plugin (.so)。

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| script | string | 是 | 策略源文件路径 |
| output | string | 否 | 输出路径，默认同名 .so |

**返回**：构建结果（成功/失败及错误信息）。

**实现要点**：
- 复用 `ctl.NewBuilder`

### 4.6 K线数据查询 — `query_kline`

**功能**：从本地数据库查询K线数据（供 AI 分析行情用）。

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| exchange | string | 是 | - | 交易所 |
| symbol | string | 是 | - | 交易对 |
| binSize | string | 否 | "1m" | K线周期 |
| start | string | 是 | - | 开始时间 |
| end | string | 是 | - | 结束时间 |
| limit | int | 否 | 500 | 最大返回条数 |

**返回**：K线数据数组 (时间、开、高、低、收、成交量)。

**实现要点**：
- 通过 `dbstore.KlineTbl` 直接查询
- 限制单次返回条数，避免数据过大

### 4.7 策略生成辅助 — `create_strategy`

**功能**：根据 AI 描述生成策略代码骨架。

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| name | string | 是 | 策略名称（作为 struct 名） |
| description | string | 否 | 策略描述 |
| indicators | string[] | 否 | 使用的指标列表 |
| outputPath | string | 是 | 输出文件路径 |

**返回**：生成的策略文件路径和内容摘要。

**实现要点**：
- 使用模板生成标准策略骨架代码
- 包含 Param/Init/OnCandle/OnPosition 等必要方法
- 根据 indicators 自动添加指标初始化代码

---

## 5. MCP Resources 设计

### 5.1 `ztrade://config` — 当前配置

暴露当前加载的 ztrade 配置（隐藏敏感信息如 API key/secret）。

### 5.2 `ztrade://doc/strategy` — 策略开发文档

暴露策略开发参考文档，供 AI 在生成策略时参考。内容来源于 `doc/strategy.md`。

### 5.3 `ztrade://doc/engine` — Engine 接口文档

暴露 Engine API 文档，包含所有可用方法和指标列表。

### 5.4 `ztrade://report/{id}` — 回测报告

使用 Resource Template，暴露历史回测报告结果。

---

## 6. MCP Prompts 设计

### 6.1 `create_strategy_prompt`

提供策略开发的引导提示模板，包含：
- ztrade 策略的基本结构要求
- Engine API 可用方法说明
- 内置指标列表及用法
- 参数定义规范
- 注意事项（1m 为基础数据、Merge 合成大周期等）

### 6.2 `analyze_backtest_prompt`

提供回测结果分析的引导提示模板，包含：
- 各项指标的含义解释
- 评估标准参考值
- 常见优化方向建议

---

## 7. 实现计划（分阶段）

### Phase 1：基础框架搭建（Day 1）

- [x] 计划制定
- [ ] 初始化 go.mod，引入 mcp-go 和 ztrade 依赖
- [ ] 实现 `config.go`：加载 ztrade.yaml 配置，初始化 DB
- [ ] 实现 `server.go`：创建 MCP Server，支持 stdio / HTTP 双模式
- [ ] 实现 `main.go`：启动入口，支持 `--transport` 参数

### Phase 2：认证与安全（Day 2）

- [ ] 实现 `auth/auth.go` — 认证接口、User 模型、RBAC 权限表
- [ ] 实现 `auth/token.go` — Bearer Token 认证
- [ ] 实现 `auth/apikey.go` — API Key 认证
- [ ] 实现 `auth/middleware.go` — HTTP 认证中间件 + ContextFunc
- [ ] 在 `server.go` 中集成认证中间件和 Tool 权限中间件

### Phase 3：核心 Tools 实现（Day 3-4）

- [ ] 实现 `tools/list.go` — `list_data` 工具（最简单，用于验证框架）
- [ ] 实现 `tools/download.go` — `download_kline` 工具
- [ ] 实现 `tools/backtest.go` — `run_backtest` 工具（核心功能）
- [ ] 实现 `tools/kline.go` — `query_kline` 工具

### Phase 4：交易与构建 Tools（Day 5）

- [ ] 实现 `tools/trade.go` — `start_trade` / `stop_trade` / `trade_status`
- [ ] 实现 `tools/build.go` — `build_strategy` 工具
- [ ] 实现 `tools/strategy.go` — `create_strategy` 工具

### Phase 5：Resources & Prompts（Day 6）

- [ ] 实现 `resources/config.go`
- [ ] 实现 `resources/strategy_doc.go`
- [ ] 实现 `resources/report.go`
- [ ] 实现 `prompts/strategy.go`
- [ ] 实现 `prompts/backtest.go`

### Phase 6：Docker 化与部署（Day 7）

- [ ] 编写 Dockerfile（多阶段构建）
- [ ] 编写 docker-compose.yml（含可选 MySQL）
- [ ] 编写 .dockerignore
- [ ] 实现 `/health` 健康检查端点
- [ ] 测试 Docker 镜像构建和运行
- [ ] 测试多平台构建（amd64 / arm64）

### Phase 7：测试与集成（Day 8）

- [ ] 编写各 Tool 的单元测试
- [ ] 编写认证模块单元测试
- [ ] 与 Claude Desktop / VS Code Copilot 集成测试
- [ ] Docker 容器化集成测试
- [ ] 编写 MCP 配置示例 (`mcp.json`)
- [ ] 完善 README 文档

---

## 8. 关键设计决策

### 8.1 配置复用

MCP Server 复用 ztrade 的配置文件 `ztrade.yaml`，通过 `--config` 参数或默认路径加载。这样无需额外配置交易所连接信息和数据库配置。

### 8.2 回测结果传递

回测工具返回结构化 JSON（`ReportResult`），而非 HTML 报告。AI 可以直接解析各项指标进行分析和对比。如需要 HTML 报告，可通过 Resource 访问。

### 8.3 实盘交易安全

- 实盘交易工具默认需要在配置中显式开启 (`mcp.enableLiveTrade: true`)
- 启动实盘前要求 AI 确认交易参数
- 提供 `trade_status` 方便随时监控
- HTTP 模式下必须通过认证才能执行交易操作
- Docker 部署时策略目录挂载为只读，防止意外修改

### 8.4 策略文件管理

- 策略文件路径支持绝对路径和相对路径（相对于配置文件所在目录）
- 生成的策略文件默认写入用户配置的策略目录

### 8.5 并发控制

- 回测为同步操作（单次调用阻塞等待结果）
- 实盘交易为异步操作（返回交易 ID，后续查询状态）
- 数据下载为同步操作

### 8.6 传输模式选择

| 场景 | 推荐传输模式 | 认证方式 |
|------|-------------|----------|
| 本地 IDE (VS Code / Cursor) | stdio | 无需认证 |
| 本地 Claude Desktop | stdio | 无需认证 |
| 个人服务器远程访问 | Streamable HTTP | Bearer Token |
| 团队共享服务 | Streamable HTTP + Docker | API Key + RBAC |
| 公网暴露（不推荐） | Streamable HTTP + HTTPS | Bearer Token + TLS |

---

## 9. MCP 客户端配置示例

### 9.1 stdio 模式 — Claude Desktop (`claude_desktop_config.json`)

```json
{
  "mcpServers": {
    "ztrade": {
      "command": "/path/to/ztrade-mcp",
      "args": ["--config", "/path/to/ztrade.yaml"],
      "env": {}
    }
  }
}
```

### 9.2 stdio 模式 — VS Code (`.vscode/mcp.json`)

```json
{
  "servers": {
    "ztrade": {
      "type": "stdio",
      "command": "/path/to/ztrade-mcp",
      "args": ["--config", "/path/to/ztrade.yaml"]
    }
  }
}
```

### 9.3 HTTP 模式 — VS Code 远程连接

```json
{
  "servers": {
    "ztrade-remote": {
      "type": "http",
      "url": "http://your-server:8080/mcp",
      "headers": {
        "Authorization": "Bearer sk-ztrade-xxxxxxxxxxxxxxxxxxxx"
      }
    }
  }
}
```

### 9.4 Docker 部署 — 远程客户端配置

```json
{
  "mcpServers": {
    "ztrade-docker": {
      "url": "http://your-server:8080/mcp",
      "headers": {
        "Authorization": "Bearer sk-ztrade-xxxxxxxxxxxxxxxxxxxx"
      }
    }
  }
}
```

---

## 10. 典型使用场景

### 场景 1：AI 辅助策略开发

```
用户: 帮我写一个基于 EMA 金叉死叉的策略，快线9慢线26
AI:   [调用 create_strategy] → 生成策略骨架
AI:   [读取 ztrade://doc/strategy] → 参考文档完善逻辑
AI:   [调用 run_backtest] → 回测 2023 年 BTCUSDT 数据
AI:   分析结果：年化收益 15%，最大回撤 8%，夏普比率 1.2...
```

### 场景 2：AI 辅助回测分析

```
用户: 回测我的 boll.go 策略在 2024 年的表现
AI:   [调用 list_data] → 确认数据是否足够
AI:   [调用 download_kline] → 补充缺失数据
AI:   [调用 run_backtest] → 执行回测
AI:   分析结果并给出优化建议
```

### 场景 3：AI 监控实盘

```
用户: 把 ema_short.so 策略跑在币安 ETHUSDT 上
AI:   [调用 start_trade] → 启动实盘
AI:   [定期调用 trade_status] → 监控运行状态
用户: 停掉 ETHUSDT 的策略
AI:   [调用 stop_trade] → 停止交易
```

---

## 11. 依赖关系

```
ztrade-mcp
├── github.com/mark3labs/mcp-go        # MCP 协议实现
├── github.com/ztrade/ztrade/pkg/ctl   # 核心控制逻辑
├── github.com/ztrade/ztrade/pkg/report # 回测报告
├── github.com/ztrade/ztrade/pkg/process/dbstore  # 数据库
├── github.com/ztrade/exchange         # 交易所接口
├── github.com/ztrade/trademodel       # 数据模型
├── github.com/spf13/viper             # 配置
└── github.com/sirupsen/logrus         # 日志
```
