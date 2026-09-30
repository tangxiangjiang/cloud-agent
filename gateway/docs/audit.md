# Audit & rate limits (M07-P01)

## Audit

Gateway records JSONL events (stderr by default, or `GATEWAY_AUDIT_LOG` / `-audit-log`).

Example line:

```json
{"at":"2026-09-28T12:00:00Z","action":"pair","method":"POST","path":"/v1/auth/pair","ip":"127.0.0.1","status":200}
```

Actions include: `pair`, `task.create`, `task.cancel`, `workflow.create`, `workflow.start`, `workflow.revise`, `workflow.review`, and M11 fleet:

| action | when |
|--------|------|
| `slave.config.upsert` / `slave.config.delete` | App CRUD via `/v1/masters/.../slaves` |
| `slave.control.start` / `stop` / `restart` | App start/stop/restart |

Meta may include `masterId`, `slaveId`, `code` — **never** API keys or tokens.

`workflow.review` meta may include `autoApprove` / `autoStartNext` (`true`/`false`) so automatic approve (M10-P03) is distinguishable from App approve.

**Never** logs full `Authorization` / bearer tokens / `CURSOR_API_KEY`. Meta fields containing `token`/`authorization`/`api` are redacted (`***` + last 4).

Query recent ring buffer (Bearer required):

`GET /v1/audit` → `{ "events": [ … ] }`

## Rate limits

| Endpoint | Limit |
|----------|-------|
| `POST /v1/auth/pair` | 10 / minute / client IP |
| `POST .../revise` | 30 / minute / client IP |
| mutating `/v1/masters*` (POST/PUT/PATCH/DELETE) | 30 / minute / client IP |

Over limit → `429` `{ "error": "rate limit exceeded" }` with `Retry-After: 60`.

## Master control errors (M11)

Stable `code` in JSON body (see `doc/slave-master.md` §14): `master_offline` (409), `master_unreachable` (503), `master_control_timeout` (504), `slave_disabled`, `slave_limit`, `repo_conflict`, `cwd_denied`, …
