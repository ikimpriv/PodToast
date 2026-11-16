# PodToast

PodToast is a Go file/diagnostic agent with an ephemeral-container toolbox for inspecting and operating on running
Kubernetes containers, including distroless and scratch applications. FarshMac
connects to it through an ephemeral container and a persistent Kubernetes exec
stream. No shell, `ls`, `cat`, `tar`, SSH server, service or listening agent port
is needed in the application container.

PodToast is an independent MIT-licensed project, originally developed for
[FarshMac](https://github.com/ikimpriv/FarshMac). The agent is in `cmd/podtoast`;
the protocol and client are in `internal/podtoast`.

Container image: **`ghcr.io/ikimpriv/podtoast:0.6.0`**, for Linux amd64 and arm64.
Release tags also update `latest`; main-branch builds use `edge`.
See [architecture and protocol](docs/ARCHITECTURE.md) and [security](SECURITY.md).

## Use from FarshMac

1. Use `ghcr.io/ikimpriv/podtoast:0.6.0`, or a digest-pinned copy in your own registry.
   The cluster must allow GHCR pulls and the required debug-container permissions.
2. Select a pod, or enter/select its container, then press **Alt+Enter / F11 → Attach ephemeral container… → PodToast**.
   Enter the image, image pull policy and timeout. Select targets with checkboxes;
   a sole running container is automatic. Shared PID targets reuse one agent.
3. Enter the container. Existing viewer/editor, copy, move, local copy/rename,
   mkdir, delete, chmod, timestamps and symbolic links use PodToast. Transfers
   retain FarshMac's collision, staging, recovery and metadata policies. Same-
   container copies stay inside the container; rename uses a filesystem rename.
4. The Kubernetes menu also provides **PodToast live monitor**, **PodToast
   snapshot**, and **PodToast network / HTTP load test…**.

Esc cancels file operations and network tests. Esc / Ctrl+C / Cmd+. stops the
live monitor. Cancelling one HTTP/2 request does not close unrelated streams.
The original shell backend remains available when PodToast has not been started.
Once connected, errors do not silently switch backends. Reconnect explicitly
when the target restarts or the connection expires (30 minutes idle).

PodToast does not provide a shell to a distroless application. Existing shell
commands still require an actual shell in the selected application container.

## Other ephemeral images and launch options

Alt+Enter or F11 opens the panel/object context menu. On a pod or its container, **Attach
 ephemeral container…** offers PodToast, the last 16 successful launch presets,
and **New image…**. Nothing is created until **Start / attach** is submitted.
History retains the image and options across restarts; target pod/container names
are not reused across pods. PodToast is an explicit integration role: an arbitrary
image is never treated as the agent just because its name contains “podtoast”.

For other images the dialog supports:

- A target container (blank means no target PID namespace; pod networking remains).
- A command as a JSON argv array, e.g. `["/bin/sh"]`; blank uses the image entrypoint.
- `Default` access (image/pod defaults), `Restricted` (non-root, drop capabilities,
  no privilege escalation, RuntimeDefault seccomp), `Network` (NET_RAW/NET_ADMIN),
  `Processes` (SYS_PTRACE), or explicit `Privileged` access. Network/Processes also
  drop other capabilities, disable privilege escalation and use RuntimeDefault.
- An optional run-as UID. Restricted requires a non-root image or nonzero UID.
- Image pull policy: IfNotPresent, Always or Never; 5–600 second startup timeout.
- Interactive TTY and read-only debug-container root switches.

FarshMac attaches to the chosen image's existing entrypoint through Kubernetes
attach, rather than launching a second shell. Short-lived commands show their
logs and exit code. These images do not enable PodToast file/diagnostic integration.
Admission/RBAC failures are reported; profiles are never broadened automatically.
Cancelling creation/wait/attach cannot remove a container already accepted by
Kubernetes. PodToast keeps its required filesystem-access profile and offers only
compatible image, target, pull-policy and startup-timeout settings.

Adding a debugger changes a single-container pod's layout; open panels are
remapped to the original container and folder. Window maximize/restore uses **Alt+F9**.

## Build

From the PodToast repository root:

```sh
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -o /tmp/podtoast-amd64 ./cmd/podtoast
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -o /tmp/podtoast-arm64 ./cmd/podtoast

docker build -f Dockerfile -t podtoast:dev .
kind load docker-image --name YOUR_LOCAL_CLUSTER podtoast:dev
```

For a registry, substitute a repository you control:

```sh
docker buildx build --platform linux/amd64,linux/arm64 \
  -f Dockerfile -t REGISTRY/OWNER/podtoast:VERSION --push .
```

Use the resulting digest in FarshMac for reproducibility. Registry credentials
are managed by the pod/cluster image pull configuration, not passed to the agent.
The final image contains the static Go agent and an Alpine diagnostic toolbox. FarshMac does not require
Docker or kubectl to attach; those tools are only used for building/testing.

## Metrics for scrapers

Prometheus-format `GET /metrics` is available over the exec connection, with an
optional dedicated HTTP listener. See [metrics specification](docs/METRICS.md)
for series, scopes, lifecycle and listener configuration.

## Available observations

| Area | Implemented |
| --- | --- |
| Files | Persistent multiplexed API, directory records streamed in batches of 256, binary streams, regular-file copy within the target, atomic filesystem rename, links, mode and modification time |
| Processes | Visible PID namespace: PID, parent, command name, state, threads, cumulative CPU time, RSS and process start ticks; capped at 16,384 records |
| CPU | Verified target cgroup v2 usage, quota, throttling and pressure; live CPU percentage uses deltas, where 100% is one core |
| Memory | Current/peak/limit, memory breakdown, swap, pressure and OOM counters when exposed by the target cgroup |
| Storage | Root and up to 64 non-pseudo directory mounts: capacity, available bytes, used bytes and inodes; cgroup I/O counters and pressure |
| Network | Pod-scoped TCP/UDP IPv4/IPv6 sockets, states and endpoints (up to 4096 per table), Unix socket table, interface counters, routes, TCP/IP counters and resolver configuration |
| Probes | DNS lookup, TCP connect, HTTP(S) GET and bounded HTTP load test from the pod network |
| Load test | Explicit target, request count, concurrency and global rate; status counts, transport failures, p50/p95/p99 elapsed request latency |

Live monitor shows a compact overview once per second; the JSON snapshot contains
more detail. File browsing remains independent from monitoring and probes.
Counters/tables that the kernel does not expose carry an `unavailable` reason.
Some diagnostic text files are capped at 64 KiB. The monitor has no growing
history buffer; snapshots and socket/process counts are bounded. Directory
listings are streamed at the agent, but FarshMac still holds the current panel's
entries in memory to sort and display them.

Load tests are HTTP GET only, at most 10,000 requests, 32 workers, 1,000 requests
per second and 60 seconds. Individual requests time out after 10 seconds and
consume at most 8 MiB of response body. Redirects are not followed; TLS
verification is enabled and uses the target root's trust store. HTTP status
counts are separate from transport/read failures. Percentiles include completed
failed requests; deadline-limited runs report how many requests completed.

## Access model and limitations

- Creation is an explicit user action. It requires access to pods and the
  `pods/ephemeralcontainers` subresource, plus `pods/exec`. Admission policies may
  refuse the requested security context. PodToast never installs a DaemonSet,
  mounts the host filesystem, uses a runtime socket, or changes cluster RBAC.
- The agent runs as root with a writable ephemeral root for shell temporary files, no privilege escalation,
  `RuntimeDefault` seccomp, all capabilities dropped except SYS_PTRACE,
  SYS_CHROOT, DAC_OVERRIDE, FOWNER, CHOWN and FSETID. These permit intentional
  target-root access and file operations; Pod Security Restricted will normally
  reject this. There is no automatic privileged retry.
- The runtime must support `targetContainerName`. The agent identifies the target
  via its runtime ID in `/proc/*/cgroup`. For a non-shared PID namespace it can
  use PID 1 after verifying that this is not its own root. Ambiguous shared-PID
  targets are refused. Host PID/network pods are not supported.
- Before serving, the dedicated exec process enters `/proc/PID/root` using
  chroot, so absolute symlinks have the application's semantics. Process start
  time is pinned and checked; a target exit/restart ends the session. Chroot is
  for path semantics, not an independent security sandbox. Kubernetes RBAC and
  the debug container security context remain the access boundary.
- New files, directories and symlinks use the target process filesystem UID/GID,
  so running the diagnostic process as root does not leave ordinary application
  uploads owned by root. This is not preservation of arbitrary source ownership.
- Read-only mounts stay read-only. The generic FarshMac remote transfer layer
  currently preserves mode and modification time, not all Linux ACLs/xattrs,
  owner/group or sparse/hard-link topology. Special-file payloads are refused.
- Cgroup metrics are reported only if target membership is verified; cgroup v1
  and inaccessible or ambiguous cgroup layouts show unavailable. Process RSS
  sums must not be mistaken for cgroup memory. Network counters are shared by
  the pod. Filesystem capacity/usage can be shared with other pods and is **not**
  an exclusive container writable-layer or directory size.
- A running matching ephemeral container is reused. Its idle process expires
  after 24 hours. Disconnecting closes exec sessions, not the idle container.
  Kubernetes keeps ephemeral-container records until the pod is deleted, and
  does not restart a terminated ephemeral container. Cancelling startup cannot
  undo an already accepted creation request.

## Verification and publishing

```sh
go test -race ./internal/podtoast
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ./cmd/podtoast
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build ./cmd/podtoast
```

CI runs native Linux race tests and binary builds on amd64 and arm64.
After tests pass, pushes to main publish `edge`; version tags such as `v0.1.0`
publish `0.1.0` and `latest`. Pull requests test without publishing. Images include
OCI source/license labels, provenance and an SBOM. Publishing uses the scoped
GitHub Actions token; no long-lived registry credentials are required.

FarshMac also maintains a real Kubernetes integration test in `podtoast/test.sh`:
it covers a distroless target, file operations, metadata, observations, probes,
cancellation and container reuse. It is run from the FarshMac checkout.

## Further integrations worth adding

These are opportunities, **not implemented features**:

- Process detail screens: file descriptors with socket-to-PID correlation,
  deleted-but-open files, cwd/executable, limits, I/O, signals and namespace IDs.
  `/proc` access is permission-dependent; environment values should be opt-in
  because they commonly contain credentials.
- Directory watch/inotify updates and agent-side recursive size/search jobs with
  progress, bounded queues and cancellation. These can remove per-directory
  network round trips for very large trees.
- Application-specific endpoints: Prometheus/OpenMetrics, Go pprof, JVM metrics
  and health endpoints. These require the application to expose them.
- Network screens with time-series rates, connection churn, DNS timing, TLS
  certificate inspection, configurable HTTP methods/headers/bodies and latency
  histograms. UDP probes and protocol-aware checks need explicit semantics.
- Packet capture and syscall profiling as separate capability profiles. Capture
  generally needs NET_RAW; ptrace profiling can disturb an application. eBPF
  often needs node/kernel access and cannot be assumed to work from this profile.
- Exact writable-layer usage, GPU telemetry, host-wide conntrack and historical
  metrics generally require runtime, node, device or external monitoring access.
  An ephemeral container alone cannot promise these.

Design references: [Kubernetes ephemeral containers](https://kubernetes.io/docs/concepts/workloads/pods/ephemeral-containers/),
[process namespace sharing](https://kubernetes.io/docs/tasks/configure-pod-container/share-process-namespace/),
[Linux cgroup v2](https://www.kernel.org/doc/html/latest/admin-guide/cgroup-v2.html).

## Ephemeral shell and toolbox

Version 0.3.1 adds shell utilities. FarshMac runs commands in the ephemeral
container, at the target folder shown in the panel. See [execution context, target selection and component licenses](docs/TOOLBOX.md).

## Traffic monitoring and connection faults

Version 0.6.0 adds port-wide controls alongside an independent graphical FarshMac traffic window with live charts and selected-connection delay, bandwidth limits, packet loss and resets. Interactive monitoring runs until exit using a renewable lease. An independent watchdog owns rule installation and cleanup. TLS stays encrypted. See [traffic control](docs/TRAFFIC.md) for scope, privileges and limitations.
