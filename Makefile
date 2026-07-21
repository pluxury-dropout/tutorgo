include .env
export

.PHONY: build run test test-integration migrate-test migrate-up migrate-down migrate-status

# TEST_DB_URL — отдельная от DB_URL переменная НАМЕРЕННО: .env смотрит в тот же
# Supabase, что и прод, и тесты с БД не должны иметь туда ни малейшего доступа.
# Задаётся в .env; не задана — integration-тесты просто пропускаются.
TEST_DB_URL ?= postgres://dev:dev@localhost:5432/tutorgo_dev?sslmode=disable

build:
	go build -o ./tmp/main.exe .

run: build
	./tmp/main.exe

test:
	go test ./...

# Тесты, которым нужна живая БД. Накатывает схему на тестовую базу и гоняет их.
test-integration: migrate-test
	TEST_DB_URL="$(TEST_DB_URL)" go test -tags=integration -count=1 ./...

migrate-test:
	goose -dir migrations postgres "$(TEST_DB_URL)" up

migrate-up:
	goose -dir migrations postgres "$(DB_URL)" up

migrate-down:
	goose -dir migrations postgres "$(DB_URL)" down

migrate-status:
	goose -dir migrations postgres "$(DB_URL)" status
