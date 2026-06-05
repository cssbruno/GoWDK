package main

import (
	"log"

	auth "github.com/cssbruno/gowdk/examples/login/src/features/auth"
)

func main() {
	if err := auth.RunBackendFromEnv(); err != nil {
		log.Fatal(err)
	}
}
