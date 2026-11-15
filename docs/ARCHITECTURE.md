# PodToast architecture and protocol v1

## Purpose and lifecycle

PodToast provides file operations and bounded diagnostics inside a running Linux
container, including distroless and scratch images. Deployment is always explicit.
FarshMac creates an ephemeral container targeting the application container's PID
namespace. The image entrypoint runs `podtoast idle` for up to 24 hours.
Each integration session executes `/podtoast serve --target-id ID`, adding
`--shared-processes` when the pod shares its PID namespace. No network listener,
SSH service, shell, runtime socket or host mount is required.

The server identifies the target using the runtime ID in cgroups. A private PID
namespace permits a checked PID 1 fallback; ambiguous shared namespaces fail.
Before starting request goroutines it enters the target root with chroot. The
process start time is pinned and checked every second. Restarting the target
ends the session; the client must reconnect explicitly.

## Transport

HTTP/2 runs over a Kubernetes exec stdin/stdout stream, with TTY disabled.
Kubernetes authentication, RBAC and transport security authorize the stream;
there is no additional agent password. stdout is exclusively protocol data;
stderr carries diagnostics. The protocol version is `1` and endpoints use `/v1`.
The server permits 32 concurrent streams, 64 KiB frames, a 2 MiB connection
upload buffer and 256 KiB per-stream upload buffer. Idle timeout is 30 minutes.
HTTP/2 cancellation stops a request without closing unrelated requests.

## Endpoints

| Method | Path | Contract |
| --- | --- | --- |
| GET | `/v1/hello` | Version, target PID, file capability, pod network scope |
| GET | `/v1/list?path=...` | NDJSON entry records, batched in groups of 256; final `text: done` record distinguishes success from truncation |
| GET | `/v1/read?path=...` | Binary regular-file stream; `Podtoast-Error` trailer propagates partial-read errors |
| PUT | `/v1/write?path=...` | Binary exclusive creation; existing paths fail; failed uploads are removed |
| POST | `/v1/op` | JSON operation: stat/lstat, mkdir, remove, rename, copy, chmod, chtimes, symlink, readlink and canonical path; see server.go for exact operation names |
| GET | `/v1/snapshot` | Timestamped process, resource, storage and network sections |
| GET | `/v1/monitor` | NDJSON snapshots once per second until cancellation |
| POST | `/v1/probe` | DNS, TCP, HTTP GET or bounded HTTP load probe |

The Go structures in `internal/podtoast/api.go` define field names and types.
JSON byte fields preserve non-UTF-8 filenames using base64; Go file-mode bits
are used, not a plain Unix permission integer. Times are RFC3339 JSON timestamps.
Missing, forbidden and existing paths use HTTP 404, 403 and 409. Invalid requests
use 400; invalidated targets use 410. Diagnostic sections include an unavailable
reason when the underlying kernel data cannot be read or attributed safely.

## Resource and compatibility constraints

File bodies and directory records stream without buffering entire files. Clients
may retain entries for sorting. New file ownership follows the target process's
filesystem UID/GID. Mode and modification time are supported; full ACL/xattr,
arbitrary ownership and hard-link topology preservation are not implemented.
Read-only mounts remain read-only; special-file payloads are rejected.

Metrics use verified cgroup v2 membership. Cgroup v1 is not implemented. Network
metrics describe the pod, and mount usage is not exclusive writable-layer usage.
Process and socket tables, text reads and load-test work have explicit limits;
see README for current values. HTTPS probes use the target root's certificate
store and never disable validation. Missing CA roots can therefore cause errors.

## Integration boundary

FarshMac owns Kubernetes discovery, explicit deployment, cancellation UI,
collision policy, progress, staging and panel caches. PodToast owns target-local
operations and observations. Other ephemeral images use normal Kubernetes attach
and must never be assumed to implement this protocol. Losing PodToast does not
silently switch file operations to another backend.
