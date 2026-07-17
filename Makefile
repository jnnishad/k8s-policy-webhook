.PHONY: build test vet lint run docker

build:
	go build -o bin/webhook ./cmd/webhook

test:
	go test ./... -race -cover

vet:
	go vet ./...

run: build
	./bin/webhook --addr=:8443 --tls-cert=dev-certs/tls.crt --tls-key=dev-certs/tls.key

docker:
	docker build -t k8s-policy-webhook:dev .
