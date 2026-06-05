GOWDK ?= go run ../../cmd/gowdk
FRONTEND_ADDR ?= 127.0.0.1:8090
BACKEND_ADDR ?= 127.0.0.1:8091
FRONTEND_ORIGIN ?= http://$(FRONTEND_ADDR)
FRONTEND_BIN ?= bin/login-frontend
BACKEND_BIN ?= bin/login-backend

.PHONY: build check serve frontend backend dev clean

build:
	$(GOWDK) build
	go build -o $(BACKEND_BIN) ./src/features/auth/backend

check:
	$(GOWDK) check --ssr src/features/auth/*.gwdk
	go test ./src/features/auth/backend

serve: build
	GOWDK_BACKEND_ADDR=$(BACKEND_ADDR) GOWDK_FRONTEND_ORIGIN=$(FRONTEND_ORIGIN) $(BACKEND_BIN) & \
	backend_pid=$$!; \
	GOWDK_ADDR=$(FRONTEND_ADDR) $(FRONTEND_BIN) & \
	frontend_pid=$$!; \
	trap 'kill $$backend_pid $$frontend_pid 2>/dev/null || true' INT TERM EXIT; \
	echo "frontend: http://$(FRONTEND_ADDR)"; \
	echo "backend:  http://$(BACKEND_ADDR)"; \
	wait

frontend:
	GOWDK_ADDR=$(FRONTEND_ADDR) $(FRONTEND_BIN)

backend:
	GOWDK_BACKEND_ADDR=$(BACKEND_ADDR) GOWDK_FRONTEND_ORIGIN=$(FRONTEND_ORIGIN) $(BACKEND_BIN)

dev:
	$(GOWDK) dev --target frontend

clean:
	rm -rf dist .gowdk bin
