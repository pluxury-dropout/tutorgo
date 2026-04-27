#!/bin/sh
set -e
./goose -dir migrations postgres "$DB_URL" up
exec ./main
