#!/bin/sh
set -eu

psql -v ON_ERROR_STOP=1 <<'SQL'
CREATE TABLE IF NOT EXISTS schema_migrations (
    version VARCHAR(255) PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
SQL

for migration in /migrations/*.up.sql; do
    version=$(basename "$migration" .up.sql)
    applied=$(psql -Atqc "SELECT 1 FROM schema_migrations WHERE version = '$version'")
    if [ "$applied" = "1" ]; then
        echo "Migration $version already applied"
        continue
    fi
    echo "Applying migration $version"
    psql -v ON_ERROR_STOP=1 -1 -f "$migration" -c "INSERT INTO schema_migrations(version) VALUES ('$version')"
done
