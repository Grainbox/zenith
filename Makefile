ifneq (,$(wildcard ./.env))
	include .env
	export
endif

.PHONY: gen lint tidy migrate-up migrate-down

GEN = buf generate
LINT = buf lint
GO_TIDY = go mod tidy

# Migration config
MIGRATE = migrate -path deployments/db/migrations -database "$(DATABASE_URL)"

gen:
	$(GEN)

lint:
	$(LINT)

tidy:
	$(GO_TIDY)

migrate-up:
	$(MIGRATE) up

migrate-down:
	$(MIGRATE) down

all: lint gen tidy
