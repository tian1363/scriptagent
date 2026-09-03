#!/usr/bin/env sh
set -eu

data_dir=${DATA_DIR:-./data}
upload_dir=${UPLOAD_DIR:-./uploads}
backup_dir=${BACKUP_DIR:-./backups}
timestamp=$(date -u +%Y%m%dT%H%M%SZ)
target="$backup_dir/scriptagent-$timestamp"

mkdir -p "$target"
sqlite3 "$data_dir/scriptagent.db" ".backup '$target/scriptagent.db'"
if [ -d "$upload_dir" ]; then
  tar -czf "$target/uploads.tar.gz" -C "$upload_dir" .
fi
printf '%s\n' "Backup written to $target"
