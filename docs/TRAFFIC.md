# Bounded pod traffic experiments

PodToast 0.4.0 adds `podtoast traffic`, an HTTP/2 control service over Kubernetes
exec stdin/stdout. It does not expose an HTTP control port in the pod. The new
`traffic` hello capability is separate from filesystem access. Traffic runs in
the ephemeral root without chroot or a target PID requirement.

TLS decryption is not implemented. No CA is generated, trust store changed, or
TLS certificate validation bypassed. Encrypted connections remain opaque TCP.

## Scope and effects

Each experiment selects one **IPv4 TCP destination port**, a peer CIDR and an
inbound/outbound/both direction. Inbound uses PREROUTING and the source CIDR;
outbound uses OUTPUT and the destination CIDR. IPv6 and UDP are unsupported.
Only new connections are redirected. The proxy changes the client source
address seen by the upstream application. This is an active proxy, not passive
packet capture; TLS still passes through without termination.

- Initial connection delay: 0–30,000 ms, once before upstream connect.
- Bandwidth: 0 (unchanged) or up to 1,048,576 KiB/s per connection and direction.
- Packet loss: 0–100%, real packet drops in mangle before NAT, in the selected
  direction. TCP retransmission and connection setup are affected.
- Reset: optional abort after 1–600,000 ms of an established proxy connection.
- Duration: mandatory 5–600 seconds. No indefinite experiment mode.

A monitor shows accepted/rejected connections, sent/received bytes, resets and
recent connections. Only the first HTTP/1 request and response header in each
connection is inspected. Bodies, authorization/cookie headers and query strings
are not retained. HTTP/2, gRPC and subsequent HTTP keep-alive messages are not
parsed. There is no PCAP export in this version.

## Ownership and cleanup

A pod-wide abstract Unix socket prevents multiple PodToast traffic experiments,
including agents in separate PID namespaces. Proxy sockets carry a mark to
exclude them from the redirect/drop rules and avoid loops. Up to 128 concurrent
connections and 128 recent records are retained; stream buffers are bounded.

An independent watchdog owns **all** rule installation and removal. It journals
exact inverse operations in a private temporary directory, retains the pod-wide
lock, and removes only its own rules/chains after stop, control-stream EOF,
controller crash or expiry. Partial installation rolls back through the same
owner, so a late installation cannot race cleanup. It never flushes a shared
chain or restores an old snapshot over another controller's changes. Cleanup
errors are surfaced and the journal is retained for diagnosis.

The watchdog must remain running; simultaneous termination of it and the proxy
cannot guarantee cleanup in a still-running network namespace. Kernel/iptables
failures may also prevent cleanup. Existing service-mesh/CNI rules can change
which traffic reaches the hooks; support is not claimed for every mesh or eBPF
routing mode. Test selected flows before relying on an experiment.

## FarshMac

Select a pod/container, then **F11 / Alt+Enter → Traffic monitoring / faults…**.
The Start form specifies the image, scope, faults and deadline. Only Start may
attach a network agent, and only when its explicit attachment checkbox is set.
An existing running PodToast with NET_ADMIN is reused. Older images without the
traffic API return an actionable compatibility error.

The network agent uses the Network profile (NET_ADMIN/NET_RAW), not privileged
mode. RBAC/admission must allow this profile. FarshMac rejects hostNetwork and
hostPID pods. The monitor uses the current FarshMac theme. Esc stops and cleans
up; closing the application closes the exec stream and triggers watchdog cleanup.

## API

- GET `/v1/hello`: `traffic: true`, `networkScope: "pod"`.
- POST `/v1/traffic/start`: validated TrafficConfig; returns TrafficStatus.
- GET `/v1/traffic`: bounded statistics and current lifecycle state.
- POST `/v1/traffic/stop`: remove rules, stop proxy connections and return status.

## Tests

Unit tests cover scope validation, inverse rules and HTTP privacy. An opt-in
Docker test in its own network namespace with NET_ADMIN verifies redirection,
initial delay, bandwidth limiting, 100% loss, reset, deadline, process-kill
cleanup, exclusion of overlapping experiments and preservation of unrelated
chains. Both amd64 and arm64 CI runners execute it. FarshMac also exercises the
API through an ephemeral container in a disposable kind cluster.
