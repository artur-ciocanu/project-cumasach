#!/usr/bin/env sh
set -eu

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"

# Example only. Replace values for a real registry.
SKILL_DIR="$ROOT_DIR/examples/python-development"
TARBALL="$ROOT_DIR/dist/python-development-1.2.0.tgz"
REPO="registry.example.com/agentskills/python-development"
TAG="1.2.0"

mkdir -p "$ROOT_DIR/dist"
go run "$ROOT_DIR/implementation/go/cmd/cumasach" package "$SKILL_DIR" --output "$TARBALL" --files-sha256

oras push "$REPO:$TAG" \
  --config "$SKILL_DIR/.skill/manifest.json:application/vnd.agentskills.config.v1+json" \
  "$TARBALL:application/vnd.agentskills.skill.content.v1.tar+gzip"
