test:
	go test ./... -race

tidy:
	go mod tidy

format:
	go fmt ./...

coverage:
	go test ./... -v -coverprofile cover.out .
	go tool cover -html cover.out -o cover.html

.PHONY: test tidy format
