# Changelog

## 0.4.0

- Add bounded pod traffic experiments over the authenticated exec transport: IPv4 TCP monitoring, delay, bandwidth, packet loss and resets.
- Add independent watchdog ownership, crash/deadline cleanup and pod-wide experiment locking.
- Keep TLS opaque and bound connection records and buffers.
- Add real network fault and recovery tests on amd64 and arm64.

## 0.3.1

- Include toolbox notices in the Docker build context; 0.3.0 image publication failed before upload.

- Add an Alpine-based ephemeral toolbox with bash, filesystem/archive utilities, networking and process diagnostics. Targets still need no shell or tools.
- Document target-cwd execution, explicit ephemeral context and third-party image licenses. The Go agent protocol is unchanged.

## 0.2.0

- Prometheus metrics over exec and an optional read-only HTTP listener.
- Bounded concurrent scrapes, documented metric scopes and missing-value semantics.
- Compatible filesystem error types for creation and collision handling in FarshMac.

## 0.1.0

First standalone release, extracted from FarshMac.

- Shell-free Go agent for Linux Kubernetes ephemeral containers, including distroless targets.
- Streaming file operations over multiplexed HTTP/2 on Kubernetes exec.
- Process, cgroup v2, storage and pod-network observations.
- Bounded DNS, TCP, HTTP and HTTP load probes.
- Public amd64/arm64 container builds through GitHub Actions.
- MIT license and English architecture/protocol documentation.
