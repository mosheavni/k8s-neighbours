# k8s-neighbours

👫 See a pod's (or node's) Kubernetes neighbours.

[![CI](https://github.com/mosheavni/k8s-neighbours/actions/workflows/ci.yml/badge.svg)](https://github.com/mosheavni/k8s-neighbours/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/mosheavni/k8s-neighbours)](https://github.com/mosheavni/k8s-neighbours/releases/latest)
[![Go Reference](https://pkg.go.dev/badge/github.com/mosheavni/k8s-neighbours.svg)](https://pkg.go.dev/github.com/mosheavni/k8s-neighbours)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

A small CLI that lists all pods scheduled on the same Kubernetes node as a
given pod — or all pods on a given node — across all namespaces.

## Install

### GitHub Releases

Download the archive for your OS/arch from the
[latest release](https://github.com/mosheavni/k8s-neighbours/releases/latest),
extract it, and put `kubectl-neighbours` on your `PATH`.

```sh
# example: macOS arm64
curl -sL https://github.com/mosheavni/k8s-neighbours/releases/latest/download/k8s-neighbours_darwin_arm64.tar.gz | tar xz kubectl-neighbours
mv kubectl-neighbours /usr/local/bin/
```

### go install

```sh
go install github.com/mosheavni/k8s-neighbours@latest
```

### Krew (kubectl plugin)

The binary is named `kubectl-neighbours`, so once on your `PATH` it also works
as a kubectl plugin: `kubectl neighbours my-pod`.

This repository doubles as a self-hosted
[custom Krew index](https://krew.sigs.k8s.io/docs/developer-guide/custom-indexes/)
(manifest in `plugins/`, updated automatically on every release):

```sh
kubectl krew index add mosheavni https://github.com/mosheavni/k8s-neighbours.git
kubectl krew install mosheavni/neighbours
kubectl neighbours my-pod
```

## Usage

```sh
# pods on the same node as a pod (namespace defaults from kubeconfig context)
kubectl-neighbours my-pod-abc123 [-n my-namespace]

# pods on a specific node
kubectl-neighbours --node ip-10-0-1-23.ec2.internal

# version info
kubectl-neighbours --version
```

Example output:

```
Node: ip-10-0-1-23.ec2.internal
NAMESPACE     NAME                       READY   STATUS    AGE
default       web-6d4b75cb6d-abcde       1/1     Running   5m
kube-system   kube-proxy-x7x9k           1/1     Running   2d2h
monitoring    node-exporter-b4qtj        1/1     Running   2d2h
```

### Flags

| Flag                | Description                                                          |
| ------------------- | -------------------------------------------------------------------- |
| `[pod]`             | Positional: name of the pod whose node neighbours to list            |
| `--node`            | Name of the node to list pods from                                   |
| `-n`, `--namespace` | Namespace of the pod (defaults to the current kubeconfig namespace)  |
| `-v`, `--version`   | Print version information and exit                                   |

Exactly one of a pod name argument or `--node` is required.

### Shell completion

Completion suggests live pod names for the positional argument, and node /
namespace names for `--node` and `-n` — straight from your cluster.

**As a standalone binary**, generate a script for your shell (bash, zsh, fish,
powershell) and source it from your shell config:

```sh
source <(kubectl-neighbours completion zsh)
```

Or, from a checkout, install them into the standard user-level directories
(bash, zsh, fish) in one go — safe to re-run:

```sh
make install-completions
```

**As a kubectl plugin** (`kubectl neighbours <TAB>`), kubectl ≥ 1.26 looks for
an executable named `kubectl_complete-neighbours` on your `PATH`. Release
archives ship one, or create it yourself — Krew
[cannot install it for you yet](https://github.com/kubernetes-sigs/krew/issues/812):

```sh
cat <<'EOF' >/usr/local/bin/kubectl_complete-neighbours
#!/usr/bin/env sh
exec kubectl-neighbours __complete "$@"
EOF
chmod +x /usr/local/bin/kubectl_complete-neighbours
```

Alternatively, the [plugin-completion](https://github.com/marckhouzam/kubectl-plugin_completion)
Krew plugin generates these shims for all your cobra-based plugins.

### Cluster access

The tool uses in-cluster configuration when running inside a pod, and falls
back to your kubeconfig (`$KUBECONFIG` or `~/.kube/config`) otherwise. It
needs permission to `get` pods (and nodes for `--node`) and `list` pods
cluster-wide.

## Development

```sh
make build      # build ./kubectl-neighbours
make test       # go test -race with coverage
make lint       # golangci-lint
make snapshot   # local multi-platform release build (requires goreleaser)
```

## Releasing

Releases are tag-driven. Either run the **Tag Release** workflow from the
Actions tab (pick patch/minor/major), or push a semver tag manually:

```sh
git tag v0.2.0
git push origin v0.2.0
```

CI does the rest: GoReleaser builds linux/darwin/windows × amd64/arm64,
publishes a GitHub release, and the Krew manifest in `plugins/` is
regenerated from the release checksums and committed to master.

Note: mark the `CI` workflow as a required status check in branch protection
so Dependabot auto-merge waits for it.

## License

[MIT](LICENSE)
