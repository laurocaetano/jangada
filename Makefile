test:
	go test ./... -race

tidy:
	go mod tidy

format:
	go fmt ./...

.PHONY: test tidy format
