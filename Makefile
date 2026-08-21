APP_DIR := cmd/x6configurator
FRONTEND_DIR := frontend
BIN_DIR := bin
WAILS := wails3

.DEFAULT_GOAL := help

.PHONY: help install-deps frontend-install frontend-build frontend-test go-test vet test build dev check clean

help: ## Show available targets
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z0-9_-]+:.*##/ {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

install-deps: ## Install native Ubuntu/Debian dependencies required by Wails
	sudo apt-get update
	sudo apt-get install -y --no-install-recommends libgtk-4-dev libwebkitgtk-6.0-dev

frontend-install: ## Install frontend dependencies deterministically
	cd $(FRONTEND_DIR) && npm ci

frontend-build: ## Build frontend assets
	cd $(FRONTEND_DIR) && npm run build

frontend-test: ## Run frontend tests
	cd $(FRONTEND_DIR) && npm test

go-test: ## Run Go tests
	go test ./...

vet: ## Run Go vet
	go vet ./...

test: go-test frontend-test ## Run Go and frontend tests

build: ## Build the Wails desktop application
	cd $(APP_DIR) && $(WAILS) build

dev: ## Start the Wails development application
	cd $(APP_DIR) && $(WAILS) dev

check: vet test build ## Run vet, tests, and the Wails build

clean: ## Remove ignored build output
	rm -rf $(BIN_DIR)
