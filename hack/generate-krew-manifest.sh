#!/usr/bin/env bash
# Generates plugins/neighbours.yaml (self-hosted krew index manifest) for a
# release tag, using the sha256 sums GoReleaser published in checksums.txt.
#
# Usage: hack/generate-krew-manifest.sh <tag> <path-to-checksums.txt>
set -euo pipefail

TAG="${1:?usage: $0 <tag> <path-to-checksums.txt>}"
CHECKSUMS="${2:?usage: $0 <tag> <path-to-checksums.txt>}"
REPO_URL="https://github.com/mosheavni/k8s-neighbours"

sha() {
  local file="$1"
  awk -v f="$file" '$2 == f { print $1; found=1 } END { exit !found }' "$CHECKSUMS" ||
    { echo "sha256 for ${file} not found in ${CHECKSUMS}" >&2; exit 1; }
}

platform() {
  local os="$1" arch="$2" ext="$3" bin="$4"
  local file="k8s-neighbours_${os}_${arch}.${ext}"
  cat <<EOF
  - selector:
      matchLabels:
        os: ${os}
        arch: ${arch}
    uri: ${REPO_URL}/releases/download/${TAG}/${file}
    sha256: $(sha "${file}")
    bin: ${bin}
EOF
}

mkdir -p plugins
{
  cat <<EOF
apiVersion: krew.googlecontainertools.github.com/v1alpha2
kind: Plugin
metadata:
  name: neighbours
spec:
  version: ${TAG}
  homepage: ${REPO_URL}
  shortDescription: List pods on the same node as a given pod
  description: |
    Lists all pods scheduled on the same Kubernetes node as a given pod,
    or all pods running on a given node, across all namespaces.
  platforms:
EOF
  platform linux amd64 tar.gz kubectl-neighbours
  platform linux arm64 tar.gz kubectl-neighbours
  platform darwin amd64 tar.gz kubectl-neighbours
  platform darwin arm64 tar.gz kubectl-neighbours
  platform windows amd64 zip kubectl-neighbours.exe
  platform windows arm64 zip kubectl-neighbours.exe
} > plugins/neighbours.yaml

echo "wrote plugins/neighbours.yaml for ${TAG}"
