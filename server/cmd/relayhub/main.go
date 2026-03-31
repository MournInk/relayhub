package main

import (
	"log"
	"net/http"
	"os"

	runtimeapp "github.com/relayhub/relayhub/server/internal/runtime"
	"github.com/relayhub/relayhub/server/internal/transport"
)

func main() {
	configPath := os.Getenv("RELAYHUB_CONFIG")
	if configPath == "" {
		configPath = "./config/relayhub.json"
	}

	app, err := runtimeapp.New(configPath)
	if err != nil {
		log.Fatalf("failed to initialize app: %v", err)
	}
	defer app.Close()

	cfg := app.Snapshot()
	log.Printf("RelayHub listening on %s", cfg.Listen)
	if err := http.ListenAndServe(cfg.Listen, transport.NewRouter(app)); err != nil {
		log.Fatalf("server exited with error: %v", err)
	}
}
