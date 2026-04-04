#!/bin/sh
set -eu
msg="${1:-}"
if [ -z "$msg" ]; then
  exit 0
fi
ts=$(date -u '+%Y-%m-%d %H:%M:%S')
printf '[%s] %s\n' "$ts" "$msg" >> '/workspace/.smith/loop/activity.log'
