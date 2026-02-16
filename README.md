# ztrade-mcp

[ztrade](https://github.com/ztrade/ztrade) 量化交易框架的 MCP (Model Context Protocol) Server 实现，使 AI 助手能通过标准化协议调用 ztrade 的核心功能。

## 功能特性

- **9 个 MCP Tools** — K线数据管理、策略回测、实盘交易、策略构建
- **2 个 Resources** — 策略开发文档、Engine API 参考
- **2 个 Prompts** — 策略开发引导、回测分析引导
- **双传输模式** — stdio（本地 IDE）+ Streamable HTTP（远程 / Docker）
- **认证控制** — Bearer Token / API Key 两种模式，RBAC 角色权限
- **Docker 支持** — 多阶段构建，一键部署

## 快速开始

### 编译

```bash
go build -o ztrade-mcp .
```

### stdio 模式（本地 IDE）

```bash
./ztrade-mcp --config /path/to/ztrade.yaml
```

### HTTP 模式（远程访问）

```bash
./ztrade-mcp --config /path/to/ztrade.yaml --transport http --listen :8080
```

### 命令行参数

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `--config` | 自动搜索 | 配置文件路径 |
| `--transport` | `stdio` | 传输模式：`stdio` 或 `http` |
| `--listen` | `:8080` | HTTP 模式监听地址 |
| `--debug` | `false` | 启用调试日志 |

配置文件自动搜索路径：`$HOME/.configs/ztrade.yaml`、`./configs/ztrade.yaml`、可执行文件同级 `configs/ztrade.yaml`。

## MCP Tools

### list_data — 查询本地数据

列出本地数据库中已有的 K 线数据集。

| 参数 | 类型 | 必填 | 说明 |
|------|------|:----:|------|
| exchange | string | | 过滤交易所 (binance, okx) |
| symbol | string | | 过滤交易对 (BTCUSDT) |

### query_kline — 查询 K 线

从本地数据库查询 OHLCV 数据，供 AI 分析行情。

| 参数 | 类型 | 必填 | 说明 |
|------|------|:----:|------|
| exchange | string | ✅ | 交易所名称 |
| symbol | string | ✅ | 交易对 |
| binSize | string | | K 线周期 (1m/5m/15m/1h/1d)，默认 1m |
| start | string | ✅ | 开始时间 `2006-01-02 15:04:05` |
| end | string | ✅ | 结束时间 |
| limit | number | | 最大返回条数，默认 500，上限 5000 |

### download_kline — 下载 K 线

从交易所下载历史 K 线数据到本地数据库。

| 参数 | 类型 | 必填 | 说明 |
|------|------|:----:|------|
| exchange | string | ✅ | 交易所名称 |
| symbol | string | ✅ | 交易对 |
| binSize | string | | K 线周期，默认 1m |
| start | string | | 开始时间（auto=false 时必填） |
| end | string | | 结束时间（auto=false 时必填） |
| auto | boolean | | 自动从 DB 最新数据续接下载到当前 |

### run_backtest — 策略回测

使用策略脚本对历史数据进行回测，返回结构化结果。

| 参数 | 类型 | 必填 | 说明 |
|------|------|:----:|------|
| script | string | ✅ | 策略文件路径 (.go 或 .so) |
| exchange | string | ✅ | 交易所名称 |
| symbol | string | ✅ | 交易对 |
| start | string | ✅ | 回测开始时间 |
| end | string | ✅ | 回测结束时间 |
| balance | number | | 初始资金，默认 100000 |
| fee | number | | 手续费率，默认 0.0005 |
| lever | number | | 杠杆倍数，默认 1 |
| param | string | | 策略参数 JSON |

**返回指标**：总收益率、年化收益率、夏普比率、索提诺比率、最大回撤、胜率、盈亏比、卡玛比率、综合得分等。

### build_strategy — 编译策略

将 Go 策略源码编译为 plugin (.so)。

| 参数 | 类型 | 必填 | 说明 |
|------|------|:----:|------|
| script | string | ✅ | 策略源文件路径 (.go) |
| output | string | | 输出路径，默认同名 .so |

### create_strategy — 生成策略骨架

根据模板生成策略代码骨架，包含标准的 ztrade 策略接口方法。

| 参数 | 类型 | 必填 | 说明 |
|------|------|:----:|------|
| name | string | ✅ | 策略名称 (PascalCase，如 EmaGoldenCross) |
| description | string | | 策略描述 |
| outputPath | string | ✅ | 输出文件路径 |
| indicators | string | | 逗号分隔的指标，如 `EMA(9,26),MACD(12,26,9),BOLL(20,2)` |
| periods | string | | 逗号分隔的合并周期，如 `5m,15m,1h` |

### start_trade — 启动实盘

启动实盘交易实例。需要在配置中设置 `mcp.enableLiveTrade: true`。

| 参数 | 类型 | 必填 | 说明 |
|------|------|:----:|------|
| script | string | ✅ | 策略文件路径 |
| exchange | string | ✅ | 交易所名称 |
| symbol | string | ✅ | 交易对 |
| param | string | | 策略参数 JSON |
| recentDays | number | | 加载最近 N 天历史数据，默认 1 |

### stop_trade — 停止实盘

| 参数 | 类型 | 必填 | 说明 |
|------|------|:----:|------|
| tradeId | string | ✅ | start_trade 返回的交易实例 ID |

### trade_status — 交易状态

| 参数 | 类型 | 必填 | 说明 |
|------|------|:----:|------|
| tradeId | string | | 交易实例 ID，不传则返回所有实例 |

## MCP Resources

| URI | 说明 |
|-----|------|
| `ztrade://doc/strategy` | 策略开发指南：策略结构、Param/Init/OnCandle 用法、两种运行方式 |
| `ztrade://doc/engine` | Engine API 参考：交易操作、指标管理、K线合并、内置指标列表 |

## MCP Prompts

| 名称 | 说明 | 参数 |
|------|------|------|
| `create_strategy` | 策略开发引导模板 | strategyType, indicators, timeframe |
| `analyze_backtest` | 回测结果分析引导 | focus (overview/risk/returns/optimization) |

## 认证配置

### stdio 模式 — 无需认证

stdio 模式由本地 IDE 启动进程，天然隔离。

### Bearer Token 模式

```yaml
mcp:
  auth:
    enabled: true
    type: token
    tokens:
      - token: "sk-ztrade-xxxxxxxxxxxxxxxxxxxx"
        name: "admin"
        role: admin
      - token: "sk-ztrade-yyyyyyyyyyyyyyyyyyyy"
        name: "readonly-bot"
        role: reader
```

### API Key 模式

```yaml
mcp:
  auth:
    enabled: true
    type: apikey
    header: "X-API-Key"
    keys:
      - key: "ak-ztrade-xxxxxxxxxxxxxxxxxxxx"
        name: "bot-1"
        role: trader
```

### RBAC 角色权限

| 工具 | reader | trader | admin |
|------|:------:|:------:|:-----:|
| list_data | ✅ | ✅ | ✅ |
| query_kline | ✅ | ✅ | ✅ |
| download_kline | ❌ | ✅ | ✅ |
| run_backtest | ✅ | ✅ | ✅ |
| build_strategy | ❌ | ✅ | ✅ |
| create_strategy | ✅ | ✅ | ✅ |
| start_trade | ❌ | ✅ | ✅ |
| stop_trade | ❌ | ✅ | ✅ |
| trade_status | ✅ | ✅ | ✅ |

- **reader**：只读操作 + 回测 + 策略生成，不能执行有副作用的操作
- **trader**：reader 的全部权限 + 数据下载、策略编译、交易管理
- **admin**：全部权限

## 配置文件

复用 ztrade 的 `ztrade.yaml` 配置文件，额外支持 `mcp` 配置段：

```yaml
# 数据库
db:
  type: sqlite
  uri: /path/to/exchange.db

# 交易所
exchanges:
  binance:
    type: binance_futures
    key: "your-api-key"
    secret: "your-api-secret"

# MCP 专属配置
mcp:
  listen: ":8080"
  enableLiveTrade: false    # 实盘交易安全开关
  auth:
    enabled: false
    type: token             # token 或 apikey
    tokens: []
    keys: []
```

## Docker 部署

### 构建镜像

```bash
docker build -t ztrade/ztrade-mcp:latest .
```

### 使用 docker-compose

```bash
docker compose up -d
```

默认配置：
- 监听 `:8080`，端点 `/mcp`
- 配置文件挂载到 `/etc/ztrade/ztrade.yaml`
- 数据持久化到 `ztrade-data` volume
- 策略目录挂载为只读
- 健康检查端点 `/health`

### docker-compose.yml 说明

```yaml
services:
  ztrade-mcp:
    build: .
    ports:
      - "8080:8080"
    volumes:
      - ./configs/ztrade.yaml:/etc/ztrade/ztrade.yaml:ro
      - ztrade-data:/data/ztrade
      - ./strategies:/strategies:ro
    healthcheck:
      test: ["CMD", "wget", "--spider", "-q", "http://localhost:8080/health"]
      interval: 30s
```

## 客户端配置

### Claude Desktop — stdio

```json
{
  "mcpServers": {
    "ztrade": {
      "command": "/path/to/ztrade-mcp",
      "args": ["--config", "/path/to/ztrade.yaml"]
    }
  }
}
```

### VS Code — stdio

`.vscode/mcp.json`:

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

### VS Code — 远程 HTTP

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

## 使用场景

### AI 辅助策略开发

```
用户: 帮我写一个基于 EMA 金叉死叉的策略，快线9慢线26
AI:   [create_strategy] → 生成策略骨架
      [ztrade://doc/strategy] → 参考文档完善逻辑
      [run_backtest] → 回测 BTCUSDT 2024年数据
      → 分析结果：年化收益 15%，最大回撤 8%，夏普比率 1.2
```

### AI 辅助回测分析

```
用户: 回测我的 boll.so 策略
AI:   [list_data] → 确认数据充足
      [download_kline] → 补充缺失数据
      [run_backtest] → 执行回测
      → 分析结果并给出优化建议
```

### AI 管理实盘交易

```
用户: 把 ema_short.so 跑在币安 ETHUSDT 上
AI:   [start_trade] → 启动实盘，返回 tradeId
      [trade_status] → 监控运行状态
用户: 停掉
AI:   [stop_trade] → 停止交易
```

## 项目结构

```
ztrade-mcp/
├── main.go                # 入口：配置加载、DB 初始化、MCP Server 启动
├── go.mod
├── auth/
│   ├── auth.go            # User 模型、Config、RBAC 权限表、Token/APIKey 认证
│   └── middleware.go       # HTTP 认证中间件、ContextFunc、Tool 权限中间件
├── tools/
│   ├── register.go        # 注册全部 9 个 Tool
│   ├── list.go            # list_data
│   ├── kline.go           # query_kline
│   ├── download.go        # download_kline
│   ├── backtest.go        # run_backtest
│   ├── build.go           # build_strategy
│   ├── strategy.go        # create_strategy
│   └── trade.go           # start_trade / stop_trade / trade_status
├── resources/
│   ├── register.go        # 注册全部 Resource
│   ├── strategy_doc.go    # ztrade://doc/strategy
│   └── engine_doc.go      # ztrade://doc/engine
├── prompts/
│   ├── register.go        # 注册全部 Prompt
│   ├── strategy.go        # create_strategy prompt
│   └── backtest.go        # analyze_backtest prompt
├── Dockerfile             # 多阶段构建
├── docker-compose.yml     # 一键部署
└── .dockerignore
```

## 依赖

| 依赖 | 说明 |
|------|------|
| [mcp-go](https://github.com/mark3labs/mcp-go) v0.43.2 | MCP 协议 Go SDK |
| [ztrade](https://github.com/ztrade/ztrade) | 量化交易框架核心 |
| [ztrade/exchange](https://github.com/ztrade/exchange) | 交易所接口 (Binance, OKX 等) |
| [ztrade/trademodel](https://github.com/ztrade/trademodel) | 交易数据模型 |
| [viper](https://github.com/spf13/viper) | 配置管理 |
| [logrus](https://github.com/sirupsen/logrus) | 日志 |

## License

MIT
