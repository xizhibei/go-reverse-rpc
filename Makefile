PKG_LIST=$(shell go list ./... | grep -v /test | grep -v /mock)

.PHONY: generate
generate:
	go generate ./...

.PHONY: lint
lint:
	@golint $(PKG_LIST)

.PHONY: clean
clean:
	@rm -f build/*

.PHONY: deps
deps:
	@go mod tidy

.PHONY: test
test:
	@go test $(PKG_LIST)

.PHONY: cover
cover:
	@go test -cover $(PKG_LIST) -coverprofile=coverage.out

.PHONY: cover-report
cover-report: cover
	@go tool cover -html=coverage.out

.PHONY: mocks
mocks:
	go generate $(PKG_LIST)

.PHONY: protoc
protoc:
	find mqttpb -type f  -name *.proto  \
	| xargs -I {} \
	protoc \
	-I mqttpb \
	--go_out=. \
	{}
	find mqttpb/test -type f  -name *.proto  \
	| xargs -I {} \
	protoc \
	-I mqttpb/test \
	--go_out=. \
	{}
