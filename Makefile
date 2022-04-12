build:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o ./bin/main ./cmd/app/main.go

run:
	godotenv go run ./cmd/app/main.go