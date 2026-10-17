.PHONY: run test migrate tidy build

DSN ?= sqlite://./data/linkgear.db
HTTP_ADDR ?= :8080
PUBLIC_URL ?= http://localhost:8080

run:
	mkdir -p data
	LINKGEAR_DSN='$(DSN)' LINKGEAR_HTTP_ADDR='$(HTTP_ADDR)' LINKGEAR_PUBLIC_URL='$(PUBLIC_URL)' go run ./cmd/linkgear

test:
	go test ./...

tidy:
	go mod tidy

build:
	mkdir -p bin
	go build -o bin/linkgear ./cmd/linkgear

# Migrations are applied automatically on startup (embedded).
# This target just verifies they embed/compile.
migrate: build
	@echo "migrations are embedded and applied on process start"
