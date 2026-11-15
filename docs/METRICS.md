# Metrics endpoint

PodToast exposes Prometheus text format 0.0.4 at `GET /metrics` on its existing
HTTP/2 exec connection. FarshMac and future scrapers can call `Client.Metrics(ctx)`
without opening a network port. This endpoint is available starting in 0.2.0;
protocol v1 is unchanged.

For an external scraper, a running `serve` session may opt into a dedicated HTTP
listener:

```text
/podtoast serve --target-id CONTAINER_RUNTIME_ID --metrics-listen 127.0.0.1:9108
```

Add `--shared-processes` if the pod shares its process namespace. The HTTP listener
exists only for that exec session and closes when it ends. The idle process does
not expose metrics. Loopback allows a scraper in the same pod to connect. An
explicit `0.0.0.0:9108` address allows pod-network access subject to network policy;
it has no built-in authentication or TLS and must not be exposed publicly.
The listener only serves read-only `/metrics`, never file or probe APIs. No
Service, annotations or network policy are automatically created. The default
is disabled. FarshMac currently uses the exec API; external listener configuration
is a CLI option, not an automatic deployment setting.

## Series

All names use the `podtoast_` prefix. Samples use scrape-time timestamps.

| Family | Type | Scope and units |
| --- | --- | --- |
| `processes_available`, `resources_available`, `storage_available`, `network_available` | gauge | 1 when the observation section is available, otherwise 0 |
| `cpu_usage_seconds_total`, `cpu_user_seconds_total`, `cpu_system_seconds_total`, `cpu_throttled_seconds_total` | counter | Verified target cgroup v2, seconds |
| `cpu_periods_total`, `cpu_throttled_periods_total` | counter | Verified target cgroup periods |
| `cpu_limit_cores` | gauge | Quota divided by period, in cores |
| `memory_usage_bytes`, `memory_limit_bytes`, `memory_peak_bytes`, `memory_swap_bytes` | gauge | Verified target cgroup v2, bytes |
| `memory_oom_total`, `memory_oom_kill_total` | counter | Verified target cgroup memory events |
| `tasks`, `tasks_limit` | gauge | Cgroup task count and limit |
| `filesystem_capacity_bytes`, `filesystem_available_bytes`, `filesystem_used_bytes`, `filesystem_inodes`, `filesystem_free_inodes` | gauge | Root filesystem; can be shared with other containers |
| `pod_network_receive_bytes_total`, `pod_network_transmit_bytes_total` | counter | All pod interfaces, including loopback |
| `visible_processes`, `visible_process_rss_bytes`, `visible_process_cpu_seconds` | gauge | Visible PID namespace; may include other containers. RSS can double-count shared pages. CPU sum decreases when processes exit. |

Missing values and unlimited limits are omitted, never fabricated as zero.
An available section does not guarantee every optional metric exists. Counters
can reset on container restart or interface recreation; scrapers must account
for resets. CPU utilization should be calculated from `rate()` of the cumulative
cgroup CPU counter. 1 CPU second per wall-clock second means one fully used core.
Scrapers should attach cluster, namespace, pod UID and target container identity
as target labels. No per-PID, filename or connection labels are emitted.

Only one metrics snapshot runs at a time per session. Concurrent scrapes receive
503 instead of queueing. Requests use a five-second context deadline; collectors
retain their bounded table/text limits and keep no sample history. A lost target
returns 410. `/v1/snapshot` remains available for detailed non-numeric data.
