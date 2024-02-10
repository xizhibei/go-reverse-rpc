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
	@go test $(shell go list ./... | grep -v /vendor/)

.PHONY: cover
cover:
	@go test -cover $(shell go list ./... | grep -v /vendor/) -coverprofile=coverage.out

.PHONY: cover-report
cover-report: cover
	@go tool cover -html=coverage.out

.PHONY: mocks
mocks:
	go generate $(shell go list ./... | grep -v /vendor/)

.PHONY: protoc
protoc:
	find pb_encoding -type f  -name *.proto  \
	| xargs -I {} \
	protoc \
	-I pb_encoding \
	--go_out=. \
	{}
	find pb_encoding/test -type f  -name *.proto  \
	| xargs -I {} \
	protoc \
	-I pb_encoding/test \
	--go_out=. \
	{}
