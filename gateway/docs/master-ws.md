# Master outbound WebSocket (M11)

Path: `GET /v1/master/ws`

Auth: same Bearer as Slave (`?token=` or first `{"type":"auth","token":"..."}`).

## Master → Gateway

| type | purpose |
|------|---------|
| `master.register` | `{masterId,name,slaves:[…]}` full mirror |
| `master.heartbeat` | keep-alive |
| `master.slaves.report` | process / pid / lastError updates |
| `master.config.ok` / `master.config.error` | CRUD reply (`requestId`) |
| `master.control.ok` / `master.control.error` | start/stop/restart reply |

## Gateway → Master

| type | purpose |
|------|---------|
| `master.config.upsert` | `{requestId, slave}` |
| `master.config.delete` | `{requestId, slaveId}` |
| `master.control` | `{requestId, slaveId, action}` |
| `master.config.list` | ask Master to re-report |

HTTP App API waits up to 30s for `*.ok` / `*.error` → otherwise `504` `master_control_timeout`.
See `doc/slave-master.md` and `doc/api-outline.md`.
