.PHONY: build run test clean migrate-up migrate-down migrate-create help

build:
	@go build -o bin/dancemirror cmd/main.go

run: build
	@./bin/dancemirror

test:
	@go test -v ./...

clean:
	@rm -rf bin/

migrate-up:
	@go run cmd/migrate/main.go up

migrate-down:
	@go run cmd/migrate/main.go down

migrate-status:
	@go run cmd/migrate/main.go version

help:
	@echo "DanceMirror Makefile Commands:"
	@echo "  make build        - Build application"
	@echo "  make run          - Run application"
	@echo "  make test         - Run tests"
	@echo "  make migrate-up   - Apply migrations"
	@echo "  make migrate-down - Rollback migrations"
