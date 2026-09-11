test:
	go test ./...

validate:
	go run cmd/mqls/main.go validate $(FILE)

compile:
	go run cmd/mqls/main.go compile $(FILE)
