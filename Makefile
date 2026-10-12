FUZZTIME ?= 10s
MODULES := . metrics localization

.PHONY: test test-race test-bench test-fuzz test-leaks fmt vet lint cover tidy

test:
	@for mod in $(MODULES); do echo "==> $$mod"; (cd $$mod && go test ./...); done

test-race:
	@for mod in $(MODULES); do echo "==> $$mod"; (cd $$mod && go test -race ./...); done

test-bench:
	@for mod in $(MODULES); do echo "==> $$mod"; (cd $$mod && go test -bench=. ./...); done

test-fuzz:
	go test ./httputil -run '^$$' -fuzz=FuzzDecodeJSON -fuzztime=$(FUZZTIME)
	go test ./httputil -run '^$$' -fuzz=FuzzGetClientIPWithNets -fuzztime=$(FUZZTIME)
	go test ./httputil -run '^$$' -fuzz=FuzzSanitizeContentDispositionFilename -fuzztime=$(FUZZTIME)
	go test ./httputil -run '^$$' -fuzz=FuzzSanitizeSSEField -fuzztime=$(FUZZTIME)
	go test ./httputil/middleware -run '^$$' -fuzz=FuzzRequestID -fuzztime=$(FUZZTIME)

test-leaks:
	go test ./httputil ./httputil/middleware -run NoGoroutineLeaks

fmt:
	gofmt -w .
	goimports -w .

vet:
	@for mod in $(MODULES); do echo "==> $$mod"; (cd $$mod && go vet ./...); done

lint:
	@for mod in $(MODULES); do echo "==> $$mod"; (cd $$mod && golangci-lint run --fix ./...); done

cover:
	@for mod in $(MODULES); do echo "==> $$mod"; (cd $$mod && go test -coverprofile=coverage.out ./... && go tool cover -func=coverage.out | tail -1); done

tidy:
	@for mod in $(MODULES); do echo "==> $$mod"; (cd $$mod && go mod tidy); done
