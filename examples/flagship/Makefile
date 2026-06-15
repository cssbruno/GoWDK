ROOT := ../..
CONFIG := examples/flagship/gowdk.config.go
APP_DIR := examples/flagship/.gowdk/app
HOOK_FILE := $(APP_DIR)/gowdkapp/flagship_hooks.go

.PHONY: check routes build serve clean

check:
	cd $(ROOT) && go run ./cmd/gowdk check --config $(CONFIG)

routes:
	cd $(ROOT) && go run ./cmd/gowdk routes --config $(CONFIG)

$(HOOK_FILE): apphooks/flagship_hooks.go.txt
	mkdir -p $(ROOT)/$(APP_DIR)/gowdkapp
	cp apphooks/flagship_hooks.go.txt $(ROOT)/$(HOOK_FILE)

build: $(HOOK_FILE)
	cd $(ROOT) && go run ./cmd/gowdk build --config $(CONFIG) --target flagship

serve: build
	GOWDK_CSRF_SECRET=development-flagship-csrf-secret-32b GOWDK_ADDR=127.0.0.1:8092 bin/flagship

clean:
	rm -rf .gowdk bin dist
