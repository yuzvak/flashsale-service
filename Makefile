# Variables
BINARY_NAME=flashsale
DOCKER_COMPOSE=docker-compose

.PHONY: all build clean run test docker-build docker-run docker-stop load-test load-test-light load-test-heavy load-test-stress realistic-test realistic-test-light realistic-test-heavy realistic-test-stress

all: build

build:
	go build -o $(BINARY_NAME) ./cmd/server

clean:
	rm -f $(BINARY_NAME)
	go clean

run: build
	./$(BINARY_NAME)

run-monitoring:
	cd monitoring
	docker-compose up -d

test:
	go test -v ./...

docker-build:
	docker build -t flashsale-service .

docker-run:
	$(DOCKER_COMPOSE) up -d

docker-stop:
	$(DOCKER_COMPOSE) down

docker-logs:
	$(DOCKER_COMPOSE) logs -f

generate-mocks:
	mockgen -source=internal/application/ports/sale_repository.go -destination=internal/mocks/sale_repository_mock.go -package=mocks
	mockgen -source=internal/application/ports/checkout_repository.go -destination=internal/mocks/checkout_repository_mock.go -package=mocks
	mockgen -source=internal/application/ports/cache.go -destination=internal/mocks/cache_mock.go -package=mocks

lint:
	golangci-lint run ./...

fmt:
	go fmt ./...

vet:
	go vet ./...

# Load testing commands
load-test:
	go run ./scripts/load-testing/test_load.go ./scripts/load-testing/run_test_load.go

load-test-light:
	go run ./scripts/load-testing/test_load.go ./scripts/load-testing/run_test_load.go light

load-test-heavy:
	go run ./scripts/load-testing/test_load.go ./scripts/load-testing/run_test_load.go heavy

load-test-stress:
	go run ./scripts/load-testing/test_load.go ./scripts/load-testing/run_test_load.go stress

realistic-test:
	go run ./scripts/load-testing/test_load.go ./scripts/load-testing/test_realistic_load.go ./scripts/load-testing/run_test_realistic_load.go

realistic-test-light:
	go run ./scripts/load-testing/test_load.go ./scripts/load-testing/test_realistic_load.go ./scripts/load-testing/run_test_realistic_load.go light

realistic-test-heavy:
	go run ./scripts/load-testing/test_load.go ./scripts/load-testing/test_realistic_load.go ./scripts/load-testing/run_test_realistic_load.go heavy

realistic-test-stress:
	go run ./scripts/load-testing/test_load.go ./scripts/load-testing/test_realistic_load.go ./scripts/load-testing/run_test_realistic_load.go stress

install-hooks:
	cp -f .git/hooks/pre-commit.sample .git/hooks/pre-commit
	chmod +x .git/hooks/pre-commit
	echo "#!/bin/sh\n\n# Pre-commit hook to ensure code quality\n# Runs formatting, linting, and tests before allowing commit\n\necho \"Running pre-commit checks...\"\n\n# Get only staged Go files\nSTAGED_GO_FILES=\$$(git diff --cached --name-only --diff-filter=ACM | grep \"\\.go\$$\")\n\n# Skip if no Go files are staged\nif [ -z \"\$$STAGED_GO_FILES\" ]; then\n    echo \"No Go files staged, skipping pre-commit checks.\"\n    exit 0\nfi\n\n# Run formatting check\necho \"Checking formatting...\"\ngo fmt ./... >/dev/null\nif [ \$$? -ne 0 ]; then\n    echo \"Error: Code formatting failed. Run 'make fmt' to fix.\"\n    exit 1\nfi\n\n# Run vet check\necho \"Running go vet...\"\ngo vet ./... >/dev/null\nif [ \$$? -ne 0 ]; then\n    echo \"Error: go vet found issues. Run 'make vet' to see details.\"\n    exit 1\nfi\n\n# Run linter if available\nif command -v golangci-lint >/dev/null 2>&1; then\n    echo \"Running linter...\"\n    golangci-lint run --fast ./... >/dev/null\n    if [ \$$? -ne 0 ]; then\n        echo \"Error: Linting failed. Run 'make lint' to see details.\"\n        exit 1\n    fi\nfi\n\n# Run tests\necho \"Running tests...\"\ngo test -short ./... >/dev/null\nif [ \$$? -ne 0 ]; then\n    echo \"Error: Tests failed. Run 'make test' to see details.\"\n    exit 1\nfi\n\necho \"All pre-commit checks passed!\"\nexit 0" > .git/hooks/pre-commit

check-all: fmt vet lint test