ROOT := ../..
CONFIG := examples/auth-guard/gowdk.config.go
APP_DIR := examples/auth-guard/.gowdk/app
HOOK_FILE := $(APP_DIR)/gowdkapp/auth_guard_hooks.go
GOWDK_AUTH_SESSION_SECRET ?= development-auth-session-secret-32bytes
GOWDK_CSRF_SECRET ?= development-auth-csrf-secret-32bytes

export GOWDK_AUTH_SESSION_SECRET
export GOWDK_CSRF_SECRET

.PHONY: check routes build serve clean

check:
	cd $(ROOT) && go run ./cmd/gowdk check --config $(CONFIG)
	cd $(ROOT)/examples/auth-guard && go test ./src/authguard

routes:
	cd $(ROOT) && go run ./cmd/gowdk routes --config $(CONFIG)

$(HOOK_FILE): apphooks/auth_guard_hooks.go.txt
	mkdir -p $(ROOT)/$(APP_DIR)/gowdkapp
	cp apphooks/auth_guard_hooks.go.txt $(ROOT)/$(HOOK_FILE)

build: $(HOOK_FILE)
	cd $(ROOT) && go run ./cmd/gowdk build --config $(CONFIG) --target auth-guard

serve: build
	GOWDK_ADDR=127.0.0.1:8094 bin/auth-guard

clean:
	rm -rf .gowdk bin dist gowdk_cache
