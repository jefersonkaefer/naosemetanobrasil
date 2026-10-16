.PHONY: test test-integration vet fmt build web-build web-test e2e

test:
	rtk go test -race ./...

test-integration:
	rtk go test -race -tags integration ./...

vet:
	rtk go vet ./...

fmt:
	rtk proxy gofmt -w cmd internal migrations

web-build:
	rtk proxy npm --prefix web run build

web-test:
	rtk proxy npm --prefix web test

e2e:
	rtk proxy npm --prefix web run test:e2e:stack

build: web-build
	rtk go build -trimpath -o bin/api ./cmd/api
	rtk go build -trimpath -o bin/migrate ./cmd/migrate
	rtk go build -trimpath -o bin/admin ./cmd/admin
