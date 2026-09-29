#!/usr/bin/env python3
"""Local 联调 / 部署：Gateway + Slave（可选 stub）+ 按 ai/index 启动 DAG。

用法（在仓库根目录）:
  python run.py plans                     # 列出 ai/index.json 中的计划
  python run.py up                        # stub + 索引 default
  python run.py up --plan flutter-chat    # 按计划 id 启动
  python run.py up --agent --plan sample-dag
  python run.py workflow --plan flutter-chat
  python run.py down
  python run.py status
  python run.py test
  python run.py app

密钥只走环境变量；勿把 token / API key 提交进 git。
详见 doc/deploy.md、ai/INDEX.md。
"""

from __future__ import annotations

import argparse
import json
import os
import signal
import subprocess
import sys
import time
import urllib.error
import urllib.request
from pathlib import Path
from typing import Any

if hasattr(sys.stdout, "reconfigure"):
    try:
        sys.stdout.reconfigure(encoding="utf-8")
        sys.stderr.reconfigure(encoding="utf-8")
    except Exception:
        pass

ROOT = Path(__file__).resolve().parent
LOCAL = ROOT / ".local"
PIDS_FILE = LOCAL / "deploy.pids.json"
TOKEN_FILE = LOCAL / "gateway.token"
SLAVE_CONFIG = ROOT / "slave" / "config.yaml"
SLAVE_CONFIG_EXAMPLE = ROOT / "slave" / "config.example.yaml"
AI_INDEX = ROOT / "ai" / "index.json"
DEFAULT_BUNDLE = ROOT / "examples" / "sample-dag.json"
STUB_BUNDLE = ROOT / "ai" / "bundles" / "m05-p02-two-node.json"
if not STUB_BUNDLE.is_file():
    STUB_BUNDLE = ROOT / "slave" / "fixtures" / "dag-two-node.json"
DEFAULT_PAIR = "ABCD-EFGH"
DEFAULT_BASE = "http://127.0.0.1:8080"


def load_plan_index() -> dict[str, Any]:
    if not AI_INDEX.is_file():
        die(f"missing plan index: {AI_INDEX.relative_to(ROOT)}（见 ai/INDEX.md）")
    try:
        data = json.loads(AI_INDEX.read_text(encoding="utf-8"))
    except json.JSONDecodeError as e:
        die(f"invalid {AI_INDEX.name}: {e}")
    if not isinstance(data.get("plans"), list):
        die(f"{AI_INDEX.name}: plans[] required")
    return data


def list_plans() -> list[dict[str, Any]]:
    return list(load_plan_index().get("plans") or [])


def find_plan(plan_id: str) -> dict[str, Any]:
    plans = list_plans()
    for p in plans:
        if p.get("id") == plan_id:
            return p
    ids = ", ".join(str(p.get("id")) for p in plans) or "(empty)"
    die(f"unknown plan id={plan_id!r}. Available: {ids}")


def resolve_bundle_path(raw: str) -> Path:
    p = Path(raw)
    if not p.is_absolute():
        p = (ROOT / p).resolve()
    else:
        p = p.resolve()
    return p


def resolve_plan_bundle(plan_id: str | None, *, stub: bool) -> tuple[Path, dict[str, Any] | None]:
    """Return (bundle_path, plan_entry_or_None)."""
    if plan_id:
        plan = find_plan(plan_id)
        bundle = plan.get("bundle")
        if not bundle:
            die(f"plan {plan_id!r} missing bundle path")
        path = resolve_bundle_path(str(bundle))
        if not path.is_file():
            die(f"plan {plan_id!r} bundle not found: {path}")
        return path, plan

    if AI_INDEX.is_file():
        idx = load_plan_index()
        default_id = idx.get("default")
        if default_id:
            for p in idx.get("plans") or []:
                if p.get("id") != default_id:
                    continue
                raw = p.get("bundle")
                if not raw:
                    break
                path = resolve_bundle_path(str(raw))
                if path.is_file():
                    return path, p
                break

    if stub:
        return STUB_BUNDLE, None
    return DEFAULT_BUNDLE, None


def bundle_ref_for(bundle_path: Path) -> str:
    try:
        return str(bundle_path.resolve().relative_to(ROOT)).replace("\\", "/")
    except ValueError:
        return str(bundle_path.resolve()).replace("\\", "/")


def die(msg: str, code: int = 1) -> None:
    print(f"error: {msg}", file=sys.stderr)
    raise SystemExit(code)


def info(msg: str) -> None:
    print(msg)


def ensure_local() -> None:
    LOCAL.mkdir(parents=True, exist_ok=True)


def http_json(
    method: str,
    url: str,
    *,
    body: dict[str, Any] | None = None,
    token: str | None = None,
    timeout: float = 15.0,
) -> Any:
    data = None
    headers = {"Accept": "application/json"}
    if body is not None:
        data = json.dumps(body).encode("utf-8")
        headers["Content-Type"] = "application/json"
    if token:
        headers["Authorization"] = f"Bearer {token}"
    req = urllib.request.Request(url, data=data, headers=headers, method=method)
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            raw = resp.read()
            if not raw:
                return None
            return json.loads(raw.decode("utf-8"))
    except urllib.error.HTTPError as e:
        err_body = e.read().decode("utf-8", errors="replace")
        die(f"{method} {url} -> HTTP {e.code}: {err_body}")
    except urllib.error.URLError as e:
        die(f"{method} {url} failed: {e.reason}")


def wait_health(base: str, timeout: float = 30.0) -> None:
    deadline = time.time() + timeout
    url = f"{base.rstrip('/')}/v1/health"
    while time.time() < deadline:
        try:
            with urllib.request.urlopen(url, timeout=2) as resp:
                if resp.status == 200:
                    return
        except Exception:
            pass
        time.sleep(0.3)
    die(f"Gateway health check timed out: {url}")


def load_pids() -> dict[str, Any]:
    if not PIDS_FILE.is_file():
        return {}
    try:
        return json.loads(PIDS_FILE.read_text(encoding="utf-8"))
    except json.JSONDecodeError:
        return {}


def save_pids(data: dict[str, Any]) -> None:
    ensure_local()
    PIDS_FILE.write_text(json.dumps(data, indent=2) + "\n", encoding="utf-8")


def save_token(token: str) -> None:
    ensure_local()
    TOKEN_FILE.write_text(token.strip() + "\n", encoding="utf-8")
    try:
        os.chmod(TOKEN_FILE, 0o600)
    except OSError:
        pass


def load_token() -> str | None:
    if TOKEN_FILE.is_file():
        t = TOKEN_FILE.read_text(encoding="utf-8").strip()
        return t or None
    return os.environ.get("GATEWAY_TOKEN")


def process_alive(pid: int) -> bool:
    if pid <= 0:
        return False
    try:
        if sys.platform == "win32":
            out = subprocess.run(
                ["tasklist", "/FI", f"PID eq {pid}", "/NH"],
                capture_output=True,
                text=True,
                check=False,
            )
            return str(pid) in (out.stdout or "")
        os.kill(pid, 0)
        return True
    except OSError:
        return False


def kill_pid(pid: int) -> None:
    if not process_alive(pid):
        return
    if sys.platform == "win32":
        subprocess.run(
            ["taskkill", "/PID", str(pid), "/T", "/F"],
            capture_output=True,
            check=False,
        )
    else:
        try:
            os.kill(pid, signal.SIGTERM)
        except OSError:
            pass
        time.sleep(0.4)
        if process_alive(pid):
            try:
                os.kill(pid, signal.SIGKILL)
            except OSError:
                pass


def cwd_for_yaml(path: Path) -> str:
    # Prefer forward slashes in YAML (matches config.example.yaml on Windows).
    return path.resolve().as_posix()


def ensure_slave_config() -> None:
    if SLAVE_CONFIG.is_file():
        return
    if not SLAVE_CONFIG_EXAMPLE.is_file():
        die(f"missing {SLAVE_CONFIG_EXAMPLE}")
    text = SLAVE_CONFIG_EXAMPLE.read_text(encoding="utf-8")
    repo_cwd = cwd_for_yaml(ROOT)
    # Replace example cwd with this checkout's absolute path.
    lines: list[str] = []
    for line in text.splitlines(keepends=True):
        if line.lstrip().startswith("cwd:"):
            indent = line[: len(line) - len(line.lstrip())]
            lines.append(f"{indent}cwd: {repo_cwd}\n")
        else:
            lines.append(line)
    SLAVE_CONFIG.write_text("".join(lines), encoding="utf-8")
    info(f"wrote {SLAVE_CONFIG.relative_to(ROOT)} (cwd={repo_cwd})")


def start_gateway(base: str, pair_code: str, audit_log: Path | None) -> int:
    addr = base.replace("http://", "").replace("https://", "")
    if not addr.startswith(":"):
        # 127.0.0.1:8080 -> :8080 for go flag (listen all / local)
        if ":" in addr:
            addr = ":" + addr.rsplit(":", 1)[-1]
        else:
            addr = ":8080"
    ensure_local()
    state_file = LOCAL / "gateway" / "state.db"
    state_file.parent.mkdir(parents=True, exist_ok=True)
    args = [
        "go",
        "run",
        ".",
        "-pair-code",
        pair_code,
        "-addr",
        addr,
        "-state-file",
        str(state_file),
    ]
    if audit_log is not None:
        args.extend(["-audit-log", str(audit_log)])
    log_path = LOCAL / "gateway.log"
    logf = open(log_path, "w", encoding="utf-8")
    creationflags = 0
    if sys.platform == "win32":
        creationflags = subprocess.CREATE_NEW_PROCESS_GROUP  # type: ignore[attr-defined]
    proc = subprocess.Popen(
        args,
        cwd=ROOT / "gateway",
        stdout=logf,
        stderr=subprocess.STDOUT,
        creationflags=creationflags,
    )
    info(f"Gateway starting pid={proc.pid} pair={pair_code} state={state_file.relative_to(ROOT)} log={log_path.relative_to(ROOT)}")
    return proc.pid


def _npm() -> str:
    return "npm.cmd" if sys.platform == "win32" else "npm"


def _flutter() -> str:
    return "flutter.bat" if sys.platform == "win32" else "flutter"


def start_slave(*, stub: bool, token: str, cursor_key: str | None) -> int:
    ensure_slave_config()
    env = os.environ.copy()
    env["GATEWAY_TOKEN"] = token
    if cursor_key:
        env["CURSOR_API_KEY"] = cursor_key
    ensure_local()
    env["SLAVE_STATE_FILE"] = str(LOCAL / "slave-runtime.json")
    npm = _npm()
    if stub:
        args = [npm, "run", "dev", "--", "--stub"]
    else:
        if not env.get("CURSOR_API_KEY"):
            die("真 Agent 模式需要环境变量 CURSOR_API_KEY（或 --cursor-key）")
        # Always tsx/dev — `npm start` uses stale slave/dist and skips approve→git commit.
        args = [npm, "run", "dev"]
    log_path = LOCAL / "slave.log"
    ensure_local()
    logf = open(log_path, "w", encoding="utf-8")
    creationflags = 0
    if sys.platform == "win32":
        creationflags = subprocess.CREATE_NEW_PROCESS_GROUP  # type: ignore[attr-defined]
    proc = subprocess.Popen(
        args,
        cwd=ROOT / "slave",
        env=env,
        stdout=logf,
        stderr=subprocess.STDOUT,
        creationflags=creationflags,
    )
    mode = "stub" if stub else "agent"
    info(f"Slave starting pid={proc.pid} mode={mode} log={log_path.relative_to(ROOT)}")
    return proc.pid


def pair(base: str, pair_code: str) -> str:
    data = http_json(
        "POST",
        f"{base.rstrip('/')}/v1/auth/pair",
        body={"pairCode": pair_code},
    )
    token = (data or {}).get("token")
    if not token:
        die(f"pair response missing token: {data}")
    save_token(token)
    info(f"paired; token saved to {TOKEN_FILE.relative_to(ROOT)}")
    return token


def create_and_start_workflow(
    base: str,
    token: str,
    bundle_path: Path,
    *,
    start: bool = True,
) -> dict[str, Any]:
    if not bundle_path.is_file():
        die(f"bundle not found: {bundle_path}")
    bundle = json.loads(bundle_path.read_text(encoding="utf-8"))
    body = {
        "bundleId": bundle["id"],
        "bundleRef": bundle_ref_for(bundle_path),
        "slaveId": bundle.get("preferredSlaveId") or "slave_devpc",
        "repoId": bundle["repoId"],
        "progressDoc": bundle.get("progressDoc") or "",
        "nodes": bundle["nodes"],
    }
    wf = http_json(
        "POST",
        f"{base.rstrip('/')}/v1/workflows",
        body=body,
        token=token,
    )
    wid = wf["id"]
    info(f"created workflow id={wid} bundle={body['bundleId']}")
    if start:
        http_json(
            "POST",
            f"{base.rstrip('/')}/v1/workflows/{wid}/start",
            token=token,
        )
        info(f"started workflow id={wid}")
    return wf


def wait_nodes(
    base: str,
    token: str,
    workflow_id: str,
    *,
    want: str = "awaiting_review",
    timeout: float = 120.0,
) -> list[dict[str, Any]]:
    deadline = time.time() + timeout
    url = f"{base.rstrip('/')}/v1/workflows/{workflow_id}"
    while time.time() < deadline:
        wf = http_json("GET", url, token=token)
        nodes = wf.get("nodes") or []
        statuses = {n.get("id"): n.get("status") for n in nodes}
        info(f"  nodes: {statuses}")
        if any(n.get("status") == want for n in nodes):
            return nodes
        if any(n.get("status") in ("failed", "rejected") for n in nodes):
            die(f"node entered terminal failure: {statuses}")
        time.sleep(1.5)
    die(f"timeout waiting for status={want} on {workflow_id}")


def cmd_up(args: argparse.Namespace) -> None:
    ensure_local()
    existing = load_pids()
    for name in ("gateway", "slave"):
        pid = existing.get(name)
        if isinstance(pid, int) and process_alive(pid):
            die(f"{name} already running (pid={pid}). Run: python run.py down")

    stub = not args.agent
    pair_code = args.pair_code
    base = args.base.rstrip("/")
    audit = LOCAL / "audit.jsonl" if args.audit else None

    gpid = start_gateway(base, pair_code, audit)
    save_pids({"gateway": gpid, "slave": None, "pairCode": pair_code, "base": base})
    wait_health(base)
    token = pair(base, pair_code)
    os.environ["GATEWAY_TOKEN"] = token

    spid = start_slave(stub=stub, token=token, cursor_key=args.cursor_key)
    pids = load_pids()
    pids["slave"] = spid
    pids["stub"] = stub
    save_pids(pids)

    # Give slave a moment to register WS.
    time.sleep(1.5)

    plan_id = getattr(args, "plan", None)
    if args.bundle:
        bundle = Path(args.bundle)
        if not bundle.is_absolute():
            bundle = ROOT / bundle
        plan = None
    else:
        bundle, plan = resolve_plan_bundle(plan_id, stub=stub)
    if plan and plan.get("cwdHint"):
        info(f"plan {plan.get('id')}: Slave cwd 应为 {plan['cwdHint']} (repoId={plan.get('repoId')})")
    info(f"bundle: {bundle_ref_for(bundle)}")

    wf = None
    if not args.no_workflow:
        wf = create_and_start_workflow(base, token, bundle, start=not args.no_start)
        if args.wait and not args.no_start:
            info(f"waiting for awaiting_review (timeout={args.wait}s)...")
            wait_nodes(base, token, wf["id"], timeout=float(args.wait))

    info("")
    info("=== 联调就绪 ===")
    info(f"  Gateway : {base}")
    info(f"  Pair    : {pair_code}")
    info(f"  Token   : {TOKEN_FILE.relative_to(ROOT)}")
    info(f"  Slave   : {'stub' if stub else 'agent'}")
    if wf:
        info(f"  Workflow: {wf['id']}")
    info("  App     : cd app && flutter run")
    info("           URL http://127.0.0.1:8080  (Android 模拟器用 http://10.0.2.2:8080)")
    info(f"           Pair code {pair_code}")
    info("  Logs    : .local/gateway.log  .local/slave.log")
    info("  Stop    : python run.py down")


def cmd_down(_: argparse.Namespace) -> None:
    pids = load_pids()
    for name in ("slave", "gateway"):
        pid = pids.get(name)
        if isinstance(pid, int):
            info(f"stopping {name} pid={pid}")
            kill_pid(pid)
    if PIDS_FILE.is_file():
        PIDS_FILE.unlink()
    info("stopped")


def cmd_status(args: argparse.Namespace) -> None:
    pids = load_pids()
    base = args.base or pids.get("base") or DEFAULT_BASE
    base = str(base).rstrip("/")
    for name in ("gateway", "slave"):
        pid = pids.get(name)
        if isinstance(pid, int):
            alive = process_alive(pid)
            info(f"{name}: pid={pid} {'alive' if alive else 'dead'}")
        else:
            info(f"{name}: not tracked")
    try:
        with urllib.request.urlopen(f"{base}/v1/health", timeout=3) as resp:
            info(f"health: HTTP {resp.status} ({base})")
    except Exception as e:
        info(f"health: unreachable ({e})")
        return
    token = load_token()
    if not token:
        info("token: missing (pair first or run up)")
        return
    info(f"token: …{token[-6:]}")
    try:
        wfs = http_json("GET", f"{base}/v1/workflows", token=token)
    except SystemExit:
        raise
    if isinstance(wfs, list):
        info(f"workflows: {len(wfs)}")
        for w in wfs[:10]:
            nodes = w.get("nodes") or []
            st = {n.get("id"): n.get("status") for n in nodes}
            info(f"  {w.get('id')} status={w.get('status')} nodes={st}")
    else:
        info(f"workflows: {wfs}")


def cmd_workflow(args: argparse.Namespace) -> None:
    base = args.base.rstrip("/")
    token = load_token()
    if not token:
        die("no token; run `python run.py up` or set GATEWAY_TOKEN")
    if args.bundle:
        bundle = Path(args.bundle)
        if not bundle.is_absolute():
            bundle = ROOT / bundle
        plan = None
    else:
        bundle, plan = resolve_plan_bundle(args.plan, stub=False)
    if plan and plan.get("cwdHint"):
        info(f"plan {plan.get('id')}: Slave cwd 应为 {plan['cwdHint']}")
    info(f"bundle: {bundle_ref_for(bundle)}")
    wf = create_and_start_workflow(base, token, bundle, start=not args.no_start)
    if args.wait and not args.no_start:
        wait_nodes(base, token, wf["id"], timeout=float(args.wait))


def cmd_plans(_: argparse.Namespace) -> None:
    idx = load_plan_index()
    default = idx.get("default")
    plans = list_plans()
    info(f"index: {AI_INDEX.relative_to(ROOT)}  default={default!r}  count={len(plans)}")
    info("")
    for p in plans:
        pid = p.get("id")
        mark = " *" if pid == default else "  "
        tags = ",".join(p.get("tags") or []) or "-"
        info(f"{mark}{pid}")
        info(f"    title : {p.get('title')}")
        info(f"    bundle: {p.get('bundle')}")
        info(f"    repoId: {p.get('repoId')}  tags: {tags}")
        if p.get("cwdHint"):
            info(f"    cwd   : {p.get('cwdHint')}")
        if p.get("notes"):
            info(f"    notes : {p.get('notes')}")
        info("")
    info("启动: python run.py up --plan <id>")
    info("文档: ai/INDEX.md")


def cmd_test(_: argparse.Namespace) -> None:
    steps = [
        (["go", "test", "./..."], ROOT / "gateway"),
        ([_npm(), "test"], ROOT / "slave"),
        ([_flutter(), "test"], ROOT / "app"),
    ]
    for cmd, cwd in steps:
        info(f"\n>>> {' '.join(cmd)}  ({cwd.relative_to(ROOT)})")
        r = subprocess.run(cmd, cwd=cwd)
        if r.returncode != 0:
            die(f"failed: {' '.join(cmd)} exit={r.returncode}")
    info("\nall tests passed")


def cmd_app(args: argparse.Namespace) -> None:
    pids = load_pids()
    pair_code = pids.get("pairCode") or args.pair_code or DEFAULT_PAIR
    info("启动 App:")
    info("  cd app")
    info("  flutter run")
    info("")
    info("配对:")
    info("  Gateway URL (本机/Chrome/iOS sim): http://127.0.0.1:8080")
    info("  Gateway URL (Android 模拟器):      http://10.0.2.2:8080")
    info(f"  Pair code: {pair_code}")
    info("")
    info("若模拟器连不上 Gateway，先确认本机健康检查:")
    info("  curl http://127.0.0.1:8080/v1/health")
    info("再试端口转发后用 127.0.0.1:")
    info("  adb reverse tcp:8080 tcp:8080")
    info("  App URL → http://127.0.0.1:8080")
    info("")
    info("路径: Workflows → 详情 → Start（若未自动）→ Diff / Review / Logs")


def build_parser() -> argparse.ArgumentParser:
    p = argparse.ArgumentParser(
        description="cloud-agent 本机联调与部署",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog=__doc__,
    )
    p.add_argument("--base", default=DEFAULT_BASE, help=f"Gateway HTTP base (default {DEFAULT_BASE})")
    p.add_argument("--pair-code", default=DEFAULT_PAIR, help=f"pair code (default {DEFAULT_PAIR})")

    sub = p.add_subparsers(dest="cmd", required=True)

    up = sub.add_parser("up", help="启动 Gateway + 配对 + Slave，并按计划创建/Start 工作流")
    up.add_argument("--agent", action="store_true", help="真 Local Agent（默认 stub）")
    up.add_argument("--stub", action="store_true", help="显式 stub（默认）")
    up.add_argument("--plan", help="ai/index.json 中的计划 id（推荐）")
    up.add_argument("--bundle", help="直接指定 DAG JSON 路径（覆盖 --plan）")
    up.add_argument("--no-workflow", action="store_true", help="不创建工作流")
    up.add_argument("--no-start", action="store_true", help="创建工作流但不 Start")
    up.add_argument("--wait", type=int, default=90, help="等待 awaiting_review 秒数；0=不等待")
    up.add_argument("--audit", action="store_true", help="写 .local/audit.jsonl")
    up.add_argument("--cursor-key", help="写入 CURSOR_API_KEY（仅进程环境，不落盘）")
    up.set_defaults(func=cmd_up)

    down = sub.add_parser("down", help="停止由本脚本启动的 Gateway/Slave")
    down.set_defaults(func=cmd_down)

    st = sub.add_parser("status", help="进程与工作流状态")
    st.set_defaults(func=cmd_status)

    pl = sub.add_parser("plans", help="列出 ai/index.json 中的可启动计划")
    pl.set_defaults(func=cmd_plans)

    wf = sub.add_parser("workflow", help="仅创建并 Start 工作流（需已 up）")
    wf.add_argument("--plan", help="ai/index.json 计划 id")
    wf.add_argument("--bundle", help="DAG JSON 路径（覆盖 --plan）")
    wf.add_argument("--no-start", action="store_true")
    wf.add_argument("--wait", type=int, default=0, help="等待 awaiting_review；0=不等待")
    wf.set_defaults(func=cmd_workflow)

    te = sub.add_parser("test", help="跑 gateway / slave / app 单测")
    te.set_defaults(func=cmd_test)

    ap = sub.add_parser("app", help="打印 Flutter App 联调说明")
    ap.set_defaults(func=cmd_app)

    return p


def main() -> None:
    parser = build_parser()
    args = parser.parse_args()
    args.func(args)


if __name__ == "__main__":
    main()
