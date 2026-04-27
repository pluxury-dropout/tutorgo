#!/bin/sh
set -e


if [ -z "$DB_URL" ]; then
  echo "ERROR: DB_URL is not set"
  exit 1
fi

# Parse host and port from DB_URL (postgres://user:pass@host:port/db)
DB_HOST=$(echo "$DB_URL" | sed -E 's|.*@([^:/]+).*|\1|')
DB_PORT=$(echo "$DB_URL" | sed -E 's|.*:([0-9]+)/.*|\1|')
DB_PORT=${DB_PORT:-5432}

echo "Waiting for PostgreSQL at $DB_HOST:$DB_PORT..."
until nc -z "$DB_HOST" "$DB_PORT"; do
  sleep 1
done
echo "PostgreSQL is ready."

./goose -dir migrations postgres "$DB_URL" up
exec ./main
