include .env
export

.PHONY: build run test migrate-up migrate-down migrate-status

build:
	go build -o ./tmp/main.exe .

run: build
	./tmp/main.exe

test:
	go test ./...

migrate-up:
	goose -dir migrations postgres "$(DB_URL)" up

migrate-down:
	goose -dir migrations postgres "$(DB_URL)" down

migrate-status:
	goose -dir migrations postgres "$(DB_URL)" status
