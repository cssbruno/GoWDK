GOWDK ?= go run ../../cmd/gowdk
APP_ADDR ?= 127.0.0.1:8090
BACKEND_ADDR ?= 127.0.0.1:8091
APP_BIN ?= bin/login
FRONTEND_BIN ?= bin/login-frontend
BACKEND_BIN ?= bin/login-backend

.PHONY: build split-build check serve serve-split frontend backend dev clean

build:
	$(GOWDK) build --target app

split-build:
	$(GOWDK) build --target split

check:
	$(GOWDK) check --ssr src/features/auth/*.gwdk
	go test ./src/features/auth

serve: build
	GOWDK_ADDR=$(APP_ADDR) $(APP_BIN)

serve-split: split-build
	GOWDK_ADDR=$(BACKEND_ADDR) $(BACKEND_BIN) & \
	backend_pid=$$!; \
	GOWDK_ADDR=$(APP_ADDR) GOWDK_BACKEND_ORIGIN=http://$(BACKEND_ADDR) $(FRONTEND_BIN) & \
	frontend_pid=$$!; \
	trap 'kill $$backend_pid $$frontend_pid 2>/dev/null || true' INT TERM EXIT; \
	echo "frontend: http://$(APP_ADDR)"; \
	echo "backend:  http://$(BACKEND_ADDR)"; \
	wait

frontend:
	GOWDK_ADDR=$(APP_ADDR) GOWDK_BACKEND_ORIGIN=http://$(BACKEND_ADDR) $(FRONTEND_BIN)

backend:
	GOWDK_ADDR=$(BACKEND_ADDR) $(BACKEND_BIN)

dev:
	$(GOWDK) dev --target app

clean:
	rm -rf dist .gowdk bin
