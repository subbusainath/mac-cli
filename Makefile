.PHONY: build run install clean migrate-up migrate-down migrate-create tidy

BINARY     := mac
DB_URL     ?= postgres://postgres:postgres@localhost:5432/mac_cli?sslmode=disable
MIGRATIONS := internal/db/migrations

build:
	go build -o $(BINARY) ./cmd/mac

run: build
	./$(BINARY)

install: build
	install -m 0755 $(BINARY) /usr/local/bin/$(BINARY)

clean:
	rm -f $(BINARY)

tidy:
	go mod tidy

migrate-up:
	migrate -path $(MIGRATIONS) -database "$(DB_URL)" up

migrate-down:
	migrate -path $(MIGRATIONS) -database "$(DB_URL)" down

migrate-create:
	@read -p "Migration name: " name; \
	migrate create -ext sql -dir $(MIGRATIONS) -seq $$name
