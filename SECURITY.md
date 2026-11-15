# Security model

PodToast intentionally grants an operator access to a target container's files,
process observations and pod network. Run it only in workloads you are authorized
to inspect. Authentication and authorization are provided by Kubernetes; the
agent must not be exposed through a public TCP listener.

The FarshMac launcher requests root with SYS_PTRACE, SYS_CHROOT, DAC_OVERRIDE,
FOWNER, CHOWN and FSETID, drops other capabilities, disables privilege escalation,
uses RuntimeDefault seccomp and a read-only agent root. Pod Security Restricted
normally rejects this configuration. There is no automatic privileged fallback.
Chroot provides target path semantics; it is not a hostile-workload sandbox.

A public image does not bypass admission, RBAC, runtime PID-namespace support,
read-only mounts or network restrictions. Air-gapped clusters must mirror it.
Ephemeral-container records remain until the pod is deleted.

Report vulnerabilities through GitHub private vulnerability reporting for this
repository. Do not include credentials, private kubeconfigs or workload data in
public issues. Pin release digests when reproducibility is required.
