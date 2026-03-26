#!/usr/bin/env sh
set -eu

# Example only. Replace values for a real registry.
SKILL_DIR="examples/skill-python-development"
TARBALL="python-development-1.2.0.tgz"
REPO="registry.example.com/agentskills/python-development"
TAG="1.2.0"

tar -czf "$TARBALL" -C examples skill-python-development

oras push "$REPO:$TAG" \
  --config "$SKILL_DIR/.skill/manifest.json:application/vnd.cumasach.config.v1+json" \
  "$TARBALL:application/vnd.cumasach.skill.content.v1.tar+gzip"

