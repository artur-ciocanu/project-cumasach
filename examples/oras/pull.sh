#!/usr/bin/env sh
set -eu

# Example only. Replace values for a real registry and digest.
REPO="registry.example.com/agentskills/python-development"
DIGEST="sha256:1111111111111111111111111111111111111111111111111111111111111111"

oras pull "$REPO@$DIGEST"

