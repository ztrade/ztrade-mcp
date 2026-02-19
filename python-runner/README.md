# python-runner (Scheme B: readonly DB direct read)

`python-runner` is a companion container for `ztrade-mcp` that executes Python research code in isolation.
It uses Scheme B: runner reads K-line data directly from DB (no large OHLCV payloads over HTTP).

## Runtime base

- Uses `continuumio/miniconda3` and a dedicated conda env (`pyrunner`)
- Preinstalls common quant/science libraries:
  - numpy / pandas / scipy
  - matplotlib / seaborn / plotly
  - statsmodels / scikit-learn / numba
  - ta-lib / ta / empyrical / backtrader

## Readonly DB policy

python-runner depends only on **readonly** DB config and does not read `db.type`/`db.uri`.

Provide readonly config from either `ztrade.yaml` or environment variables:

- `db.readonly.type` (`mysql` / `sqlite`)
- `db.readonly.uri` (recommended)
  - MySQL example: `exchange_readonly:pwd@tcp(mysql:3306)/exchange?charset=utf8mb4&parseTime=True&loc=Local`
  - SQLite example: `/data/ztrade/exchange.db`
- Or MySQL split fields instead of URI:
  - `db.readonly.host` / `db.readonly.port` / `db.readonly.database`
  - `db.readonly.user` / `db.readonly.password`

Equivalent env vars:

- `PYRUNNER_READONLY_TYPE`
- `PYRUNNER_READONLY_URI`
- `PYRUNNER_READONLY_HOST`
- `PYRUNNER_READONLY_PORT`
- `PYRUNNER_READONLY_DATABASE`
- `PYRUNNER_READONLY_USER`
- `PYRUNNER_READONLY_PASSWORD`

If readonly config is missing, runner startup fails.

## API

- `GET /health`
- `POST /v1/research/run`

Request body:

```json
{
  "exchange": "binance",
  "symbol": "BTCUSDT",
  "binSize": "1m",
  "start": 1735689600,
  "end": 1735776000,
  "limit": 10000,
  "timeoutSec": 30,
  "code": "result = {\"rows\": len(df)}"
}
```

Response body includes: `ok` / `error` / `stdout` / `stderr` / `result` / `meta` / `images`.

`images` is a list of base64 images:

```json
[
  {"name": "figure-1.png", "mimeType": "image/png", "data": "...base64..."}
]
```

## Image output from user code

You can return chart images in either way:

1. Set explicit variables:
   - `images = [{"path": "/tmp/a.png"}]`
   - `images = [{"data": "<base64>", "mimeType": "image/png"}]`
   - legacy: `image_path`, `image_paths`, `image_base64`
2. Or keep matplotlib figures open. Runner auto-captures current figures as PNG.

Limits are controlled by env vars:

- `PYRUNNER_MAX_IMAGES` (default `3`)
- `PYRUNNER_MAX_IMAGE_BYTES` (default `1048576`)

## User code context

Built-ins in execution scope:

- `df`: DataFrame for requested symbol/time range
- `pd`, `np`
- `load_kline(exchange, symbol, binSize, start, end, limit=0)`

Default columns in `df`:

- `start`, `open`, `high`, `low`, `close`, `volume`, `turnover`, `trades`, `time`
