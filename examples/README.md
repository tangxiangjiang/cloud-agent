# Examples

| File | Use |
|------|-----|
| [sample-dag.json](./sample-dag.json) | Two-node DAG pointing at `doc/roadmaps/cloud-agent/phases` |

Load into Gateway (after pair):

```bash
# Replace TOKEN and adjust slaveId/repoId to match slave config.yaml
curl -s -X POST http://127.0.0.1:8080/v1/workflows \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d @<(python -c "
import json
b=json.load(open('examples/sample-dag.json'))
print(json.dumps({
  'bundleId': b['id'],
  'bundleRef': 'examples/sample-dag.json',
  'slaveId': b.get('preferredSlaveId','slave_devpc'),
  'repoId': b['repoId'],
  'progressDoc': b['progressDoc'],
  'nodes': b['nodes'],
}))
")
```

Or use PowerShell: see [doc/deploy.md](../doc/deploy.md).

Also see `ai/bundles/` and `slave/fixtures/dag-two-node.json` for smoke fixtures.
