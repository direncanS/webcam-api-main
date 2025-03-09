.PHONY: build buildAWS run zip

build:
	env GOARCH=amd64 GOOS=linux go build -o bin/bootstrap main.go


run: build
	./bin/bootstrap

buildAWS:
	env GOARCH=arm64 GOOS=linux go build -o bin/bootstrap main.go

zip: buildAWS
	zip -j bootstrap.zip ./bin/bootstrap
