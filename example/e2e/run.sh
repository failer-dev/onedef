#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

echo "=== 1. Starting backend ==="
go run ./backend &
BACKEND_PID=$!
sleep 2

cleanup() {
  echo "=== Stopping backend ==="
  kill $BACKEND_PID 2>/dev/null || true
}
trap cleanup EXIT

echo "=== 2. Downloading Dart SDK ==="
mkdir -p dart_client/packages
curl -s -o /tmp/user_api.zip "http://localhost:8081/onedef/sdk/dart?name=user_api"
rm -rf dart_client/packages/user_api
unzip -o /tmp/user_api.zip -d dart_client/packages/

echo "=== 3. Running Dart client ==="
cd dart_client
dart pub get
dart run bin/main.dart

echo "=== E2E complete ==="
