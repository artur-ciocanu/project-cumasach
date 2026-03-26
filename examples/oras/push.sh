#!/usr/bin/env sh
set -eu

# Example only. Replace values for a real registry.
SKILL_DIR="examples/python-development"
TARBALL="python-development-1.2.0.tgz"
REPO="registry.example.com/agentskills/python-development"
TAG="1.2.0"

tar -czf "$TARBALL" -C examples python-development

oras push "$REPO:$TAG" \
  --config "$SKILL_DIR/.skill/manifest.json:application/vnd.agentskills.config.v1+json" \
  "$TARBALL:application/vnd.agentskills.skill.content.v1.tar+gzip"
