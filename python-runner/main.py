import base64
import io
import json
import mimetypes
import os
import re
import sqlite3
import traceback
from contextlib import redirect_stderr, redirect_stdout
from dataclasses import dataclass
from typing import Any, Dict, List, Optional, Tuple
from urllib.parse import parse_qs, quote, urlencode, urlsplit, urlunsplit

import multiprocessing as mp

from fastapi import FastAPI, HTTPException, Request
from pydantic import BaseModel, Field


# Keep table-name validation aligned with ztrade/pkg/process/dbstore/db.go.
TBL_RE = re.compile(r"^([A-Za-z0-9\-]+)_([A-Za-z0-9_\-]+)_([A-Za-z0-9]+)$")


@dataclass(frozen=True)
class RunnerConfig:
    token: str
    readonly_type: str
    readonly_uri: str
    readonly_host: str
    readonly_port: int
    readonly_database: str
    readonly_user: str
    readonly_password: str
    max_rows: int
    max_code_bytes: int
    max_output_bytes: int
    default_timeout_sec: int
    max_images: int
    max_image_bytes: int


class ResearchRequest(BaseModel):
    exchange: str = Field(..., min_length=1)
    symbol: str = Field(..., min_length=1)
    binSize: str = Field("1m", min_length=1)
    start: int = Field(..., description="Unix timestamp (seconds)")
    end: int = Field(..., description="Unix timestamp (seconds)")
    limit: int = Field(0, ge=0, description="Max rows to load; 0 means use server default")
    timeoutSec: int = Field(0, ge=0, description="Execution timeout; 0 means use server default")
    code: str = Field(..., min_length=1)


def _read_int_env(name: str, default: int) -> int:
    raw = os.getenv(name, "").strip()
    if raw == "":
        return default
    try:
        return int(raw)
    except ValueError:
        return default


def _load_readonly_from_yaml(cfg_path: str) -> Dict[str, str]:
    import yaml

    with open(cfg_path, "r", encoding="utf-8") as f:
        data = yaml.safe_load(f) or {}

    readonly = ((data.get("db") or {}).get("readonly")) or {}
    return {
        "type": str(readonly.get("type") or "").strip(),
        "uri": str(readonly.get("uri") or "").strip(),
        "host": str(readonly.get("host") or "").strip(),
        "port": str(readonly.get("port") or "").strip(),
        "database": str(readonly.get("database") or "").strip(),
        "user": str(readonly.get("user") or "").strip(),
        "password": str(readonly.get("password") or "").strip(),
    }


def _infer_readonly_type(readonly_uri: str) -> str:
    dsn = readonly_uri.lower()
    if "@tcp(" in dsn:
        return "mysql"
    return "sqlite"


def load_config() -> RunnerConfig:
    token = os.getenv("PYRUNNER_TOKEN", "").strip()

    yaml_readonly: Dict[str, str] = {}
    cfg_path = os.getenv("PYRUNNER_ZTRADE_CONFIG", "/etc/ztrade/ztrade.yaml").strip()
    if cfg_path and os.path.exists(cfg_path):
        yaml_readonly = _load_readonly_from_yaml(cfg_path)

    readonly_type = os.getenv("PYRUNNER_READONLY_TYPE", yaml_readonly.get("type", "")).strip().lower()
    readonly_uri = os.getenv("PYRUNNER_READONLY_URI", yaml_readonly.get("uri", "")).strip()
    readonly_host = os.getenv("PYRUNNER_READONLY_HOST", yaml_readonly.get("host", "")).strip()
    readonly_database = os.getenv("PYRUNNER_READONLY_DATABASE", yaml_readonly.get("database", "")).strip()
    readonly_user = os.getenv("PYRUNNER_READONLY_USER", yaml_readonly.get("user", "")).strip()
    readonly_password = os.getenv("PYRUNNER_READONLY_PASSWORD", yaml_readonly.get("password", "")).strip()

    readonly_port_str = os.getenv("PYRUNNER_READONLY_PORT", yaml_readonly.get("port", "")).strip()
    if readonly_port_str:
        try:
            readonly_port = int(readonly_port_str)
        except ValueError:
            readonly_port = 3306
    else:
        readonly_port = 3306

    if not readonly_type:
        if readonly_uri:
            readonly_type = _infer_readonly_type(readonly_uri)
        else:
            raise RuntimeError("missing readonly.type (or readonly.uri)")

    if readonly_type == "mysql":
        if not readonly_uri and (not readonly_host or not readonly_database or not readonly_user):
            raise RuntimeError(
                "mysql readonly config required: set db.readonly.uri or db.readonly.host/database/user"
            )
    elif readonly_type == "sqlite":
        if not readonly_uri:
            raise RuntimeError("sqlite readonly config required: set db.readonly.uri")
    else:
        raise RuntimeError(f"unsupported readonly.type: {readonly_type}")

    max_rows = _read_int_env("PYRUNNER_MAX_ROWS", 200_000)
    if max_rows <= 0:
        max_rows = 200_000

    max_code_bytes = _read_int_env("PYRUNNER_MAX_CODE_BYTES", 200_000)
    if max_code_bytes <= 0:
        max_code_bytes = 200_000

    max_output_bytes = _read_int_env("PYRUNNER_MAX_OUTPUT_BYTES", 4 << 20)
    if max_output_bytes <= 0:
        max_output_bytes = 4 << 20

    default_timeout_sec = _read_int_env("PYRUNNER_DEFAULT_TIMEOUT_SEC", 60)
    if default_timeout_sec <= 0:
        default_timeout_sec = 60

    max_images = _read_int_env("PYRUNNER_MAX_IMAGES", 3)
    if max_images <= 0:
        max_images = 3

    max_image_bytes = _read_int_env("PYRUNNER_MAX_IMAGE_BYTES", 1 << 20)
    if max_image_bytes <= 0:
        max_image_bytes = 1 << 20

    return RunnerConfig(
        token=token,
        readonly_type=readonly_type,
        readonly_uri=readonly_uri,
        readonly_host=readonly_host,
        readonly_port=readonly_port,
        readonly_database=readonly_database,
        readonly_user=readonly_user,
        readonly_password=readonly_password,
        max_rows=max_rows,
        max_code_bytes=max_code_bytes,
        max_output_bytes=max_output_bytes,
        default_timeout_sec=default_timeout_sec,
        max_images=max_images,
        max_image_bytes=max_image_bytes,
    )


def _truncate_text_bytes(s: str, max_bytes: int) -> Tuple[str, bool]:
    if max_bytes <= 0:
        return s, False
    b = s.encode("utf-8", errors="replace")
    if len(b) <= max_bytes:
        return s, False
    cut = b[:max_bytes]
    return cut.decode("utf-8", errors="replace"), True


def _json_default(obj: Any) -> Any:
    # Minimal conversions to make common scientific types JSON-friendly.
    try:
        import numpy as np

        if isinstance(obj, (np.integer, np.floating)):
            return obj.item()
        if isinstance(obj, np.ndarray):
            return obj.tolist()
    except Exception:
        pass

    try:
        import pandas as pd

        if isinstance(obj, pd.DataFrame):
            head = obj.head(50)
            return {
                "type": "DataFrame",
                "shape": [int(obj.shape[0]), int(obj.shape[1])],
                "columns": [str(c) for c in obj.columns],
                "head": head.to_dict(orient="records"),
            }
        if isinstance(obj, pd.Series):
            head = obj.head(50)
            return {
                "type": "Series",
                "shape": [int(obj.shape[0])],
                "name": str(obj.name),
                "head": head.tolist(),
            }
    except Exception:
        pass

    return str(obj)


def _mk_table(exchange: str, symbol: str, bin_size: str) -> str:
    tbl = f"{exchange}_{symbol}_{bin_size}"
    if not TBL_RE.match(tbl):
        raise ValueError(f"invalid table name: {tbl}")
    return tbl


def _parse_go_mysql_dsn(dsn: str) -> Tuple[str, str, str, int, str]:
    # Best-effort parser for go-sql-driver/mysql DSN.
    # Example: user:pass@tcp(mysql:3306)/exchange?charset=utf8mb4&parseTime=True&loc=Local
    m = re.match(
        r"^(?P<user>[^:@/]+)(:(?P<pw>[^@/]*))?@tcp\((?P<host>[^):]+)(:(?P<port>[0-9]+))?\)/(?P<db>[^?]+)",
        dsn,
    )
    if not m:
        raise ValueError("unsupported mysql dsn format")
    user = m.group("user")
    pw = m.group("pw") or ""
    host = m.group("host")
    port = int(m.group("port") or "3306")
    db = m.group("db")
    return user, pw, host, port, db


def _mysql_params_from_config(cfg: RunnerConfig) -> Tuple[str, str, str, int, str]:
    if cfg.readonly_uri:
        user, pw, host, port, db = _parse_go_mysql_dsn(cfg.readonly_uri)
    else:
        user = cfg.readonly_user
        pw = cfg.readonly_password
        host = cfg.readonly_host
        port = cfg.readonly_port
        db = cfg.readonly_database

    # Explicit env/yaml fields override URI credentials if both are present.
    if cfg.readonly_user:
        user = cfg.readonly_user
    if cfg.readonly_password:
        pw = cfg.readonly_password

    return user, pw, host, port, db


def _sqlite_readonly_uri(raw: str) -> str:
    if raw.startswith("file:"):
        parts = urlsplit(raw)
        q = parse_qs(parts.query)
        q["mode"] = ["ro"]
        return urlunsplit((parts.scheme, parts.netloc, parts.path, urlencode(q, doseq=True), parts.fragment))

    # Plain filesystem path -> sqlite URI with readonly mode.
    return f"file:{quote(raw, safe='/')}?mode=ro"


def _load_kline_df(cfg: RunnerConfig, exchange: str, symbol: str, bin_size: str, start: int, end: int, limit: int):
    import pandas as pd

    tbl = _mk_table(exchange, symbol, bin_size)

    final_limit = limit if limit > 0 else cfg.max_rows
    if final_limit > cfg.max_rows:
        final_limit = cfg.max_rows

    cols = ["start", "open", "high", "low", "close", "volume", "turnover", "trades"]

    if cfg.readonly_type == "sqlite":
        q = (
            f"SELECT start, open, high, low, close, volume, turnover, trades "
            f"FROM \"{tbl}\" WHERE start >= ? AND start <= ? ORDER BY start ASC LIMIT ?"
        )
        sqlite_uri = _sqlite_readonly_uri(cfg.readonly_uri)
        with sqlite3.connect(sqlite_uri, uri=True) as conn:
            cur = conn.execute(q, (start, end, final_limit))
            rows = cur.fetchall()
        df = pd.DataFrame(rows, columns=cols)
    elif cfg.readonly_type == "mysql":
        import pymysql

        user, pw, host, port, db = _mysql_params_from_config(cfg)
        q = (
            f"SELECT start, open, high, low, close, volume, turnover, trades "
            f"FROM `{tbl}` WHERE start >= %s AND start <= %s ORDER BY start ASC LIMIT %s"
        )
        conn = pymysql.connect(host=host, user=user, password=pw, database=db, port=port)
        try:
            with conn.cursor() as cur:
                cur.execute(q, (start, end, final_limit))
                rows = cur.fetchall()
        finally:
            conn.close()
        df = pd.DataFrame(rows, columns=cols)
    else:
        raise ValueError(f"unsupported readonly.type: {cfg.readonly_type}")

    if not df.empty:
        df["time"] = pd.to_datetime(df["start"], unit="s", utc=True)

    return df


def _get_var(loc: Dict[str, Any], glb: Dict[str, Any], name: str) -> Any:
    if name in loc:
        return loc[name]
    return glb.get(name)


def _image_from_bytes(raw: bytes, mime_type: str, name: str, cfg: RunnerConfig) -> Optional[Dict[str, str]]:
    if len(raw) == 0:
        return None
    if len(raw) > cfg.max_image_bytes:
        return None
    if not mime_type:
        mime_type = "image/png"
    if not mime_type.startswith("image/"):
        return None
    return {
        "data": base64.b64encode(raw).decode("ascii"),
        "mimeType": mime_type,
        "name": name,
    }


def _image_from_path(path: str, mime_type: str, cfg: RunnerConfig) -> Optional[Dict[str, str]]:
    if not path:
        return None
    try:
        with open(path, "rb") as f:
            raw = f.read()
    except Exception:
        return None

    guessed, _ = mimetypes.guess_type(path)
    if not mime_type:
        mime_type = guessed or "image/png"

    name = os.path.basename(path)
    return _image_from_bytes(raw, mime_type, name, cfg)


def _normalize_user_image(item: Any, cfg: RunnerConfig) -> Optional[Dict[str, str]]:
    if isinstance(item, str):
        # A plain string is treated as file path.
        return _image_from_path(item.strip(), "", cfg)

    if not isinstance(item, dict):
        return None

    data = str(item.get("data") or "").strip()
    path = str(item.get("path") or "").strip()
    mime_type = str(item.get("mimeType") or item.get("mime_type") or "").strip()
    name = str(item.get("name") or "").strip()

    if data:
        try:
            raw = base64.b64decode(data, validate=True)
        except Exception:
            return None
        return _image_from_bytes(raw, mime_type, name, cfg)

    if path:
        return _image_from_path(path, mime_type, cfg)

    return None


def _collect_images_from_vars(loc: Dict[str, Any], glb: Dict[str, Any], cfg: RunnerConfig) -> List[Dict[str, str]]:
    images: List[Dict[str, str]] = []

    # Preferred: images = [{data/mimeType} | {path/mimeType}]
    images_var = _get_var(loc, glb, "images")
    if images_var is not None:
        if isinstance(images_var, list):
            for item in images_var:
                img = _normalize_user_image(item, cfg)
                if img is not None:
                    images.append(img)
                    if len(images) >= cfg.max_images:
                        return images
        else:
            img = _normalize_user_image(images_var, cfg)
            if img is not None:
                images.append(img)
                if len(images) >= cfg.max_images:
                    return images

    # Legacy single-image variables.
    image_base64 = _get_var(loc, glb, "image_base64")
    image_mime_type = _get_var(loc, glb, "image_mime_type") or "image/png"
    if isinstance(image_base64, str) and image_base64.strip():
        img = _normalize_user_image({"data": image_base64, "mimeType": str(image_mime_type)}, cfg)
        if img is not None:
            images.append(img)
            if len(images) >= cfg.max_images:
                return images

    image_path = _get_var(loc, glb, "image_path")
    if isinstance(image_path, str) and image_path.strip():
        img = _normalize_user_image({"path": image_path}, cfg)
        if img is not None:
            images.append(img)
            if len(images) >= cfg.max_images:
                return images

    image_paths = _get_var(loc, glb, "image_paths")
    if isinstance(image_paths, list):
        for p in image_paths:
            if not isinstance(p, str):
                continue
            img = _normalize_user_image({"path": p}, cfg)
            if img is not None:
                images.append(img)
                if len(images) >= cfg.max_images:
                    return images

    return images


def _collect_images_from_matplotlib(cfg: RunnerConfig) -> List[Dict[str, str]]:
    try:
        import matplotlib.pyplot as plt
    except Exception:
        return []

    fig_nums = plt.get_fignums()
    if not fig_nums:
        return []

    images: List[Dict[str, str]] = []
    for fig_num in fig_nums:
        if len(images) >= cfg.max_images:
            break
        try:
            fig = plt.figure(fig_num)
            buf = io.BytesIO()
            fig.savefig(buf, format="png", bbox_inches="tight")
            img = _image_from_bytes(buf.getvalue(), "image/png", f"figure-{fig_num}.png", cfg)
            if img is not None:
                images.append(img)
        except Exception:
            continue

    return images


def _run_user_code(cfg: RunnerConfig, req: Dict[str, Any]) -> Dict[str, Any]:
    import numpy as np
    import pandas as pd

    exchange = req["exchange"]
    symbol = req["symbol"]
    bin_size = req["binSize"]
    start = int(req["start"])
    end = int(req["end"])
    limit = int(req.get("limit") or 0)
    code = req["code"]

    df = _load_kline_df(cfg, exchange, symbol, bin_size, start, end, limit)

    def load_kline(exchange: str, symbol: str, binSize: str, start: int, end: int, limit: int = 0) -> pd.DataFrame:
        return _load_kline_df(cfg, exchange, symbol, binSize, int(start), int(end), int(limit))

    stdout_buf = io.StringIO()
    stderr_buf = io.StringIO()

    glb: Dict[str, Any] = {
        "__name__": "__research__",
        "pd": pd,
        "np": np,
        "df": df,
        "load_kline": load_kline,
    }
    loc: Dict[str, Any] = {}

    ok = True
    err_text: Optional[str] = None
    result: Any = None

    with redirect_stdout(stdout_buf), redirect_stderr(stderr_buf):
        try:
            exec(code, glb, loc)
            result = loc.get("result", glb.get("result"))
        except Exception:
            ok = False
            err_text = traceback.format_exc()

    stdout, stdout_trunc = _truncate_text_bytes(stdout_buf.getvalue(), cfg.max_output_bytes)
    stderr, stderr_trunc = _truncate_text_bytes(stderr_buf.getvalue(), cfg.max_output_bytes)

    # Try to JSON-serialize the result; fall back to string conversion.
    try:
        json.dumps(result, default=_json_default)
        safe_result = result
    except Exception:
        safe_result = _json_default(result)

    images = _collect_images_from_vars(loc, glb, cfg)
    if not images:
        # If user did not provide explicit image outputs, try open matplotlib figures.
        images = _collect_images_from_matplotlib(cfg)

    return {
        "ok": ok,
        "error": err_text,
        "meta": {
            "exchange": exchange,
            "symbol": symbol,
            "binSize": bin_size,
            "start": start,
            "end": end,
            "rows": int(df.shape[0]),
            "dbType": cfg.readonly_type,
            "dbReadonly": True,
        },
        "stdout": stdout,
        "stdoutTruncated": stdout_trunc,
        "stderr": stderr,
        "stderrTruncated": stderr_trunc,
        "result": safe_result,
        "images": images,
    }


def _worker(req: Dict[str, Any], cfg_dict: Dict[str, Any], conn) -> None:
    try:
        cfg = RunnerConfig(**cfg_dict)
        res = _run_user_code(cfg, req)
    except Exception:
        res = {
            "ok": False,
            "error": traceback.format_exc(),
            "meta": {},
            "stdout": "",
            "stdoutTruncated": False,
            "stderr": "",
            "stderrTruncated": False,
            "result": None,
            "images": [],
        }

    try:
        conn.send(res)
    finally:
        try:
            conn.close()
        except Exception:
            pass


def _run_in_subprocess(req: Dict[str, Any], cfg: RunnerConfig, timeout_sec: int) -> Dict[str, Any]:
    ctx = mp.get_context("spawn")
    parent_conn, child_conn = ctx.Pipe(duplex=False)

    p = ctx.Process(target=_worker, args=(req, cfg.__dict__, child_conn))
    p.daemon = True
    p.start()

    p.join(timeout=timeout_sec)
    if p.is_alive():
        p.terminate()
        p.join(timeout=5)
        return {
            "ok": False,
            "error": f"timeout after {timeout_sec}s",
            "meta": {
                "exchange": req.get("exchange"),
                "symbol": req.get("symbol"),
                "binSize": req.get("binSize"),
                "start": req.get("start"),
                "end": req.get("end"),
                "dbType": cfg.readonly_type,
            },
            "stdout": "",
            "stdoutTruncated": False,
            "stderr": "",
            "stderrTruncated": False,
            "result": None,
            "images": [],
        }

    if not parent_conn.poll(1):
        return {
            "ok": False,
            "error": "no response from worker",
            "meta": {"dbType": cfg.readonly_type},
            "stdout": "",
            "stdoutTruncated": False,
            "stderr": "",
            "stderrTruncated": False,
            "result": None,
            "images": [],
        }

    return parent_conn.recv()


app = FastAPI(title="python-runner", version="0.4.0")


@app.get("/health")
def health() -> Dict[str, str]:
    return {"status": "ok"}


@app.post("/v1/research/run")
def run_research(req: ResearchRequest, http_req: Request) -> Dict[str, Any]:
    try:
        cfg = load_config()
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))

    if cfg.token:
        auth = (http_req.headers.get("authorization") or "").strip()
        if auth != f"Bearer {cfg.token}":
            raise HTTPException(status_code=401, detail="unauthorized")

    if req.start > req.end:
        raise HTTPException(status_code=400, detail="start must be <= end")

    code_bytes = req.code.encode("utf-8", errors="replace")
    if len(code_bytes) > cfg.max_code_bytes:
        raise HTTPException(status_code=400, detail=f"code too large: {len(code_bytes)} bytes")

    # Enforce an upper bound on limit to avoid OOM.
    limit = int(req.limit or 0)
    if limit < 0:
        limit = 0
    if limit > cfg.max_rows:
        limit = cfg.max_rows

    timeout_sec = int(req.timeoutSec or 0)
    if timeout_sec <= 0:
        timeout_sec = cfg.default_timeout_sec

    # Normalize payload for subprocess.
    payload = {
        "exchange": req.exchange,
        "symbol": req.symbol,
        "binSize": req.binSize,
        "start": int(req.start),
        "end": int(req.end),
        "limit": int(limit),
        "timeoutSec": int(timeout_sec),
        "code": req.code,
    }

    # Keep user-code exceptions in response body (ok=false) so MCP can show stdout/stderr.
    return _run_in_subprocess(payload, cfg, timeout_sec)
