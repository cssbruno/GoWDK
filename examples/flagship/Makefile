CONFIG := gowdk.config.go
APP_DIR := .gowdk/app
HOOK_FILE := $(APP_DIR)/gowdkapp/flagship_hooks.go

.PHONY: check routes build serve clean

check:
	go run ../../cmd/gowdk check --config $(CONFIG)

routes:
	go run ../../cmd/gowdk routes --config $(CONFIG)

$(HOOK_FILE): apphooks/flagship_hooks.go.txt
	mkdir -p $(APP_DIR)/gowdkapp
	cp apphooks/flagship_hooks.go.txt $(HOOK_FILE)

build: $(HOOK_FILE)
	go run ../../cmd/gowdk build --config $(CONFIG) --target flagship

serve: build
	GOWDK_CSRF_SECRET=development-flagship-csrf-secret-32b GOWDK_FLAGSHIP_SECRET=development-flagship-session-secret-32b GOWDK_FLAGSHIP_PASSWORD=demo-password GOWDK_ADDR=127.0.0.1:8092 bin/flagship

clean:
	rm -rf .gowdk bin dist
