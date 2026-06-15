ROOT := ../..
CONFIG := examples/endpoints/gowdk.config.go

.PHONY: check routes build serve clean

check:
	cd $(ROOT) && go run ./cmd/gowdk check --config $(CONFIG)

routes:
	cd $(ROOT) && go run ./cmd/gowdk routes --config $(CONFIG)

build:
	cd $(ROOT) && go run ./cmd/gowdk build --config $(CONFIG) --target endpoints

serve: build
	GOWDK_CSRF_SECRET=development-endpoints-csrf-secret-32b GOWDK_ADDR=127.0.0.1:8093 bin/endpoints

clean:
	rm -rf .gowdk bin dist
