#!/bin/sh
set -e

# O .env de desenvolvimento traz STORAGE_PATH=./storage. No container o
# cwd é / e o processo não é root, então mkdir ./storage falha.
# Caminho relativo → volume persistente; absoluto (ex. /data/storage) permanece.
storage_path="${STORAGE_PATH:-/data/storage}"
case "$storage_path" in
  /*) ;;
  *) storage_path=/data/storage ;;
esac
export STORAGE_PATH="$storage_path"
export TZ="${TZ:-America/Sao_Paulo}"

# .env de desenvolvimento vaza APP_ENV=development via env_file.
case "${APP_ENV:-}" in
  development|dev|"") export APP_ENV=production ;;
esac

mkdir -p "$STORAGE_PATH"
chown -R atlas:atlas "$STORAGE_PATH"

exec su-exec atlas:atlas /usr/local/bin/api "$@"
