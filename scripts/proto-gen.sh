#!/usr/bin/env bash
set -euo pipefail

PROTO_DIR="proto"
GEN_DIR="gen"
mkdir -p "${GEN_DIR}"

echo "▶ Generating protobuf code..."

find "${PROTO_DIR}" -name "*.proto" | while read -r proto_file; do
  echo "  Processing ${proto_file}..."
  protoc \
    --go_out="${GEN_DIR}" \
    --go_opt=paths=source_relative \
    --go-grpc_out="${GEN_DIR}" \
    --go-grpc_opt=paths=source_relative \
    -I "${PROTO_DIR}" \
    "${proto_file}"
done

echo "✅ Protobuf generation complete → ${GEN_DIR}/"
