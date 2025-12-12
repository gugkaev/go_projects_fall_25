.PHONY: test integration-test build up down logs

test:
	@echo "Running unit tests..."
	cd orders-service && go test -v ./...
	cd payments-service && go test -v ./...

integration-test:
	@echo "Running integration tests..."
	@echo "Make sure services are running: docker compose up -d"
	go test -v -tags=integration ./integration_test.go

build:
	docker compose build

up:
	docker compose up -d

down:
	docker compose down

logs:
	docker compose logs -f

clean:
	docker compose down -v
	docker system prune -f

