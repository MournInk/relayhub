package main

import (
	"flag"
	"log"
	"net/http"
	"os"

	runtimeapp "github.com/relayhub/relayhub/server/internal/runtime"
	"github.com/relayhub/relayhub/server/internal/transport"
)

func main() {
	defaultConfigPath := os.Getenv("RELAYHUB_CONFIG")
	if defaultConfigPath == "" {
		defaultConfigPath = "./config/relayhub.json"
	}

	configPath := flag.String("config", defaultConfigPath, "path to RelayHub config file")
	listen := flag.String("listen", "", "listen address override")
	adminToken := flag.String("admin-token", "", "admin token override")
	databasePath := flag.String("database", "", "database path override")
	instanceName := flag.String("instance-name", "", "instance name override")
	flag.Parse()

	if *listen != "" {
		_ = os.Setenv("RELAYHUB_LISTEN", *listen)
	}
	if *adminToken != "" {
		_ = os.Setenv("RELAYHUB_ADMIN_TOKEN", *adminToken)
	}
	if *databasePath != "" {
		_ = os.Setenv("RELAYHUB_DATABASE_PATH", *databasePath)
	}
	if *instanceName != "" {
		_ = os.Setenv("RELAYHUB_INSTANCE_NAME", *instanceName)
	}

	app, err := runtimeapp.New(*configPath)
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
