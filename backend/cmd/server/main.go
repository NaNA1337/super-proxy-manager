package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/NaNA1337/super-proxy-manager/backend/auth"
	"github.com/NaNA1337/super-proxy-manager/backend/db"
	"github.com/NaNA1337/super-proxy-manager/backend/embedded"
	"github.com/NaNA1337/super-proxy-manager/backend/server"
)

func main() {
	port := flag.Int("port", 8443, "Web Manager listen port")
	host := flag.String("host", "0.0.0.0", "Web Manager listen host")
	dataDir := flag.String("data-dir", "data", "Data directory for SQLite database")
	flag.Parse()

	if envPort := os.Getenv("PORT"); envPort != "" {
		_, _ = fmt.Sscanf(envPort, "%d", port)
	}
	if envHost := os.Getenv("HOST"); envHost != "" {
		*host = envHost
	}
	if envData := os.Getenv("DATA_DIR"); envData != "" {
		*dataDir = envData
	}

	addr := fmt.Sprintf("%s:%d", *host, *port)

	// 1. Initialize SQLite Database
	database, err := db.InitDB(*dataDir)
	if err != nil {
		log.Fatalf("Failed to initialize database in %q: %v", *dataDir, err)
	}
	defer database.Close()

	// 2. Perform first-time setup check / bootstrap
	_, _, err = auth.CheckOrInitBootstrap(database, addr)
	if err != nil {
		log.Fatalf("Failed during bootstrap verification: %v", err)
	}

	// 3. Load embedded frontend assets
	distFS, err := embedded.GetDistFS()
	if err != nil {
		log.Printf("[WebManager] Warning: failed to load embedded frontend assets: %v", err)
	}

	// 4. Create and start HTTP server
	srv := server.NewServer(addr, database, distFS)

	log.Printf("======================================================")
	log.Printf("  Super-Proxy Multi-Host Control Panel                ")
	log.Printf("  Listening on: http://%s                             ", addr)
	log.Printf("  Database: %s/super-proxy-manager.db                 ", *dataDir)
	log.Printf("======================================================")

	if err := srv.Start(); err != nil {
		log.Fatalf("Server exited with error: %v", err)
	}
}
