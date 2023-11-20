.PHONY: generate
generate:
	go generate ./...

go-pkg-list:
	export PKG_LIST="$(shell go list ./... | grep -v /vendor/)"

.PHONY: lint
lint:
	@go lint $(shell go list ./... | grep -v /vendor/)

.PHONY: clean
clean:
	@rm -f build/*

.PHONY: deps
deps:
	@go mod tidy

.PHONY: test
test:
	@go test -cover -v $(shell go list ./... | grep -v /vendor/)

.PHONY: coverage-report
coverage-report:
	@go test -cover -v $(shell go list ./... | grep -v /vendor/) -coverprofile=build/coverage.out
	@go tool cover -html=build/coverage.out

.PHONY: mocks
mocks:
	go generate $(shell go list ./... | grep -v /vendor/)

.PHONY: protoc
protoc:
	find reverse_rpc_pb/proto -type f  -name *.proto  \
	| xargs -I {} \
	protoc \
	-I reverse_rpc_pb/proto \
	--go_out=. \
	{}
	find reverse_rpc_pb/test/proto -type f  -name *.proto  \
	| xargs -I {} \
	protoc \
	-I reverse_rpc_pb/test/proto \
	--go_out=. \
	{}
