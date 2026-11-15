# Contributing

Use English for code comments, errors, documentation, specifications and issues.
Changes should include focused tests, cancellation behavior and bounded memory
usage. Run `go test -race ./internal/podtoast` and cross-build the agent for Linux
amd64 and arm64. Do not commit credentials, kubeconfigs or private cluster data.
Protocol changes must document compatibility with FarshMac. Contributions are
licensed under the repository's MIT license.
