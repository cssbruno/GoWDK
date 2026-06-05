package appgen

const serverMainSource = `package main

import (
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"gowdk-generated-app/gowdkapp"
)

func main() {
	handler, err := gowdkapp.Handler()
	if err != nil {
		log.Fatal(err)
	}

	addr := env("GOWDK_ADDR", "127.0.0.1:8080")
	server := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
	log.Printf("serving embedded GOWDK static app at http://%s", addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func env(name, fallback string) string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	return value
}
`

const appPackageSourceTemplate = `package gowdkapp

import (
	"embed"
	"io/fs"
	"net/http"
{{RUNTIME_IMPORTS}}
)

const maxActionBodyBytes int64 = 1 << 20

//go:embed static
var embeddedFiles embed.FS

func Handler() (http.Handler, error) {
	return ServeMux()
}

func ServeMux() (*http.ServeMux, error) {
	root, err := fs.Sub(embeddedFiles, "static")
	if err != nil {
		return nil, err
	}
	mux := http.NewServeMux()
	mux.Handle("/", gowdkruntime.Handler{
		Root:       root,
		Identity:   gowdkruntime.InstanceIdentity(),
		Assets:     gowdkruntime.LoadAssetManifest(root),
		Action:     action,
		SSRExact:   ssrExact,
		SSRDynamic: ssrDynamic,
	})
	return mux, nil
}

{{ACTION_HANDLER}}

{{SSR_HANDLER}}
`
