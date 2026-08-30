.PHONY: fmt test build run up down migrate seed seed-docker logs

fmt:
	gofmt -w $$(find . -name '*.go' -not -path './vendor/*')

test:
	GOFLAGS=-buildvcs=false go test ./...

build:
	GOFLAGS=-buildvcs=false go build ./cmd/api ./cmd/worker ./cmd/seed

run:
	set -a; . ./.env; set +a; GOFLAGS=-buildvcs=false go run ./cmd/api

up:
	docker compose up -d postgres redis migrate api worker

down:
	docker compose down

migrate:
	docker compose run --rm migrate

seed:
	set -a; . ./.env; set +a; GOFLAGS=-buildvcs=false go run ./cmd/seed

seed-docker:
	docker compose --profile tools run --rm seed

logs:
	docker compose logs -f api worker
