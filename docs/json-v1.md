# Socket JSON v1

`list` and `inspect` accept an opt-in, versioned JSON response:

```sh
socklens list --format json-v1
socklens list --format=json-v1 --listening --protocol tcp
socklens inspect --format json-v1 8080
```

Put options before the port for `inspect`. The same socket filters apply to text,
legacy JSON, and JSON v1. Other commands do not accept `--format`.

## Response

The command writes one JSON object followed by a newline. For example:

```json
{
  "schema_version": "1.0",
  "command": "inspect",
  "generated_at": "2026-11-17T00:00:00Z",
  "count": 1,
  "sockets": [
    {
      "protocol": "tcp",
      "local": {"address": "127.0.0.1", "port": 8080},
      "remote": {"address": "0.0.0.0", "port": 0},
      "state": "LISTEN",
      "pids": [42],
      "processes": [{"pid": 42, "name": "node"}]
    }
  ]
}
```

| Field | Type | Meaning |
| --- | --- | --- |
| `schema_version` | string | `"1.0"` for this contract. |
| `command` | string | `"list"` or `"inspect"`. |
| `generated_at` | string | UTC RFC 3339 timestamp when the response is prepared after collection. Fractional seconds may be present. |
| `count` | integer | Number of sockets after filtering; equals the length of `sockets`. |
| `sockets` | array | Observed sockets, always an array, including when empty. |

Each socket contains:

| Field | Type | Meaning |
| --- | --- | --- |
| `protocol` | string | `"tcp"` or `"udp"`. |
| `local` | object | Local endpoint: `address` (string) and `port` (integer, 0–65535). |
| `remote` | object | Remote endpoint with the same fields. |
| `state` | string | Observed backend state, such as `LISTEN`, `ESTABLISHED`, `UNCONN`, or Windows UDP `BOUND`. |
| `inode` | string, optional | Backend socket inode when available; omitted otherwise. Treat as an opaque identifier. |
| `pids` | array of integers | Observed process IDs; `[]` when unavailable. |
| `processes` | array of objects | Best-effort names as `{ "pid": 42, "name": "node" }`; `[]` when unavailable. A name can be empty. |

IPv6 addresses are unbracketed in `address` because the port is separate. A scope
suffix, such as `fe80::1%12`, is preserved. Wildcard addresses and zero remote
ports are observations, not evidence of a remote connection. If the backend
provides no remote endpoint, it is represented by `{"address":"","port":0}`.
An unconnected socket can also have a backend-reported `0.0.0.0:0` or `[::]:0`.

Missing ownership data does not mean a socket has no owner. Process names and
states are platform observations, not verified service identities. Collection
is a best-effort snapshot, not an atomic history of socket/process activity.

## Empty results

No matching sockets is a successful result:

```json
{
  "schema_version": "1.0",
  "command": "list",
  "generated_at": "2026-11-17T00:00:00Z",
  "count": 0,
  "sockets": []
}
```

`sockets`, `pids`, and `processes` are never `null` in JSON v1.

## Compatibility and errors

- `--json` retains the existing array and capitalized socket fields. Existing
  scripts do not need to migrate. Empty legacy JSON remains `[]`.
- Without `--format`, existing text/`--json` behavior is preserved.
- `--format table` selects the existing text behavior, or legacy JSON when
  combined with `--json`. `--format json-v1` cannot be combined with `--json=true`.
- JSON v1 suppresses terminal colors and automatic update notices/background
  checks, including when output goes to a terminal.
- Exit `0` means collection, filtering, and writing succeeded, including empty
  results. Exit `1` means invalid arguments, discovery, serialization, or output
  failure. Diagnostics go to stderr as plain text; there is no JSON error envelope.
- A collection/endpoint error emits no response. A failing output stream can
  contain a partial write; check the exit code before consuming output.
- Consumers should ignore unknown added fields and avoid depending on whitespace,
  object-key ordering, array ordering across platforms, or an exhaustive set of
  state strings. Removing fields or changing their meaning/types requires a new
  explicitly selected schema contract; do not silently repurpose `json-v1`.

## Example consumer

```sh
socklens list --format json-v1 > sockets.json
```

After confirming the command succeeded, read `sockets.json` with any JSON parser.
For example, the Python standard library:

```python
import json

with open("sockets.json", encoding="utf-8") as stream:
    report = json.load(stream)
if report["schema_version"] != "1.0":
    raise ValueError("unsupported socket schema")
for socket in report["sockets"]:
    print(socket["protocol"], socket["local"]["address"], socket["local"]["port"])
```
