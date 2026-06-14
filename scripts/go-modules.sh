#!/usr/bin/env sh
set -eu

cat <<'EOF'
.
runtime/adapters/chi
runtime/adapters/echo
runtime/adapters/fiber
runtime/adapters/gin
runtime/contracts/natsbroker
runtime/contracts/redisstream
runtime/contracts/websocketfanout
EOF
