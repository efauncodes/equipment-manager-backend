#!/usr/bin/env sh

set -eu

data_dir="${STAGING_DATA_DIR:?set STAGING_DATA_DIR to the staging data directory}"
backup_dir="${STAGING_BACKUP_DIR:-${data_dir%/}/../backups}"
database="${data_dir%/}/equipment.db"
timestamp="$(date -u +%Y%m%d-%H%M%S)"
backup="${backup_dir%/}/equipment-${timestamp}.db"

if [ ! -f "$database" ]; then
	echo "staging database not found: $database" >&2
	exit 1
fi

mkdir -p "$backup_dir"
sqlite3 "$database" ".backup '$backup'"
chmod 600 "$backup"
echo "staging backup created: $backup"
