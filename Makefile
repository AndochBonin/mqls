test:
	go test ./...

build:
	go build ./...

lint:
	go run cmd/mqls/main.go lint $(FILE)

compile:
	go run cmd/mqls/main.go compile $(FILE)
