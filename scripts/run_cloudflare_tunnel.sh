#!/usr/bin/env bash
set -eu

TUNNEL_NAME="api-demo"
CONFIG_PATH="$HOME/.cloudflared/config.yml"

if [[ ! -f "$CONFIG_PATH" ]]; then
  echo "Missing Cloudflare config at $CONFIG_PATH"
  exit 1
fi

exec cloudflared tunnel run --config "$CONFIG_PATH" "$TUNNEL_NAME"
