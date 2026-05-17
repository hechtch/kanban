CONTAINER_RUNTIME ?= docker
IMAGE ?= kanban:latest

.PHONY: install run run-bg build test lint clean \
        run-frontend run-backend build-frontend build-backend \
        test-frontend test-backend lint-frontend lint-backend \
        container-build container-run container-push

install:
	cd frontend && npm install
	cd backend  && go mod tidy

run:
	$(MAKE) -j2 run-backend run-frontend

run-bg:
	@mkdir -p /tmp
	@cd backend && nohup go run . -addr :8000 > /tmp/kanban-backend.log 2>&1 & \
		echo "[backend] PID $$! — logs: /tmp/kanban-backend.log"
	cd frontend && npx ng serve

run-frontend:
	cd frontend && npx ng serve

run-backend:
	cd backend && go run . -addr :8000

build: build-frontend build-backend

build-frontend:
	cd frontend && npx ng build

build-backend:
	cd backend && mkdir -p bin && go build -o bin/kanban .

test: test-backend test-frontend

test-backend:
	cd backend && go test -coverprofile=coverage.out ./... && go tool cover -func=coverage.out

test-frontend:
	cd frontend && npx ng test --watch=false --code-coverage

lint: lint-frontend lint-backend

lint-backend:
	cd backend && golangci-lint run ./...

lint-frontend:
	cd frontend && npx ng lint

clean:
	rm -rf backend/bin backend/coverage.out
	rm -rf frontend/dist frontend/coverage frontend/.angular

container-build:
	$(CONTAINER_RUNTIME) build -t $(IMAGE) .

container-run:
	$(CONTAINER_RUNTIME) run --rm -p 8000:8000 \
		-v $(HOME)/.kanban/data:/data:Z \
		-e KANBAN_DB_PATH=/data/kanban.db \
		$(IMAGE)

container-push:
	$(CONTAINER_RUNTIME) push $(IMAGE)
