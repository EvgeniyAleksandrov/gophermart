.PHONY: build run test run-accrual generate test-race coverage

RUN_ADDRESS ?= :8080
DATABASE_URI ?= host=localhost port=5432 dbname=gophermart user=postgres password=postgres sslmode=disable
ACCRUAL_SYSTEM_ADDRESS ?= http://localhost:8081
ACCRUAL_RUN_ADDRESS ?= :8081
ACCRUAL_DATABASE_URI ?= $(DATABASE_URI)

# Quote one complete POSIX shell argument, including embedded single quotes.
shell_quote = '$(subst ','"'"',$(1))'

build:
	go build -buildvcs=false -o cmd/gophermart/gophermart ./cmd/gophermart

run: build
	@exec ./cmd/gophermart/gophermart -a $(call shell_quote,$(RUN_ADDRESS)) -d $(call shell_quote,$(DATABASE_URI)) -r $(call shell_quote,$(ACCRUAL_SYSTEM_ADDRESS))

test:
	go test ./...

generate:
	go generate ./...

test-race:
	go test -race -count=1 ./...

coverage:
	go test -count=1 -coverpkg=./... -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out
	go tool cover -html=coverage.out -o coverage.html

run-accrual:
	@set -eu; \
	platform="$$(go env GOHOSTOS)_$$(go env GOHOSTARCH)"; \
	case "$$platform" in darwin_arm64|darwin_amd64|linux_amd64|windows_amd64) ;; \
	*) echo "Unsupported accrual platform: $$platform" >&2; exit 1 ;; esac; \
	binary="./cmd/accrual/accrual_$$platform"; \
	if [ ! -f "$$binary" ]; then echo "Missing accrual binary: $$binary" >&2; exit 1; fi; \
	if [ ! -x "$$binary" ]; then chmod +x "$$binary"; fi; \
	unset RUN_ADDRESS DATABASE_URI ACCRUAL_SYSTEM_ADDRESS; \
	exec "$$binary" -a $(call shell_quote,$(ACCRUAL_RUN_ADDRESS)) -d $(call shell_quote,$(ACCRUAL_DATABASE_URI))
