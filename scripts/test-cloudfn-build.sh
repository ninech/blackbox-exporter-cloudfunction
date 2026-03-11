#!/usr/bin/env bash
# Simulates the GCP Gen2 buildpack wrapper module build to catch ambiguous import
# errors that only surface during Cloud Function deployment.
set -euo pipefail

REPO_ROOT=$(git rev-parse --show-toplevel)

# Avoid macOS treating /tmp as a system root (Go ignores go.mod files there).
WORK_DIR=$(mktemp -d "$(go env GOPATH)/cf-build-test-XXXXXX")
trap "rm -rf $WORK_DIR" EXIT

cd "$WORK_DIR"
go mod init functions.local/app

# Now merge with the main repo, as Google's buildpack does.
go work init . "$REPO_ROOT"

cat > main.go <<'EOF'
package main

import (
	_ "github.com/ninech/blackbox-exporter-cloudfunction"
	_ "github.com/GoogleCloudPlatform/functions-framework-go/funcframework"
)

func main() {}
EOF

go build ./...
echo "Build OK - no ambiguous imports detected"
