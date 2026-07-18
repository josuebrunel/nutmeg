.PHONY: dev build run db docker-up docker-down migrate migrate-down templ-gen clean test

dev:
	templ generate --watch > /dev/null 2>&1 &
	air -c .air.toml

build:
	templ generate
	go build -buildvcs=false -ldflags="-s -w" -o bin/server ./cmd/server

run: build
	./bin/server

db:
	docker compose up -d db

docker-up:
	docker compose up --build

docker-down:
	docker compose down

migrate:
	go run github.com/pressly/goose/v3/cmd/goose -dir migrations postgres "$(DB_DSN)" up

migrate-down:
	go run github.com/pressly/goose/v3/cmd/goose -dir migrations postgres "$(DB_DSN)" down

templ-gen:
	templ generate

test:
	go test -failfast ./... -v -p=1 -count=1

clean:
	rm -rf tmp/ bin/ views/**/*_templ.go
