package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strconv"

	"github.com/NaNA1337/super-proxy-manager/backend/auth"
	"github.com/NaNA1337/super-proxy-manager/backend/db"
	"github.com/NaNA1337/super-proxy-manager/backend/embedded"
	hostpolicy "github.com/NaNA1337/super-proxy-manager/backend/host"
	"github.com/NaNA1337/super-proxy-manager/backend/server"
)

var version = "dev"
var commit = "unknown"

const defaultDataDir = "/var/lib/super-proxy-manager/data"

func main() {
	showVersion := flag.Bool("version", false, "print build version and exit")
	port := flag.Int("port", 8443, "Web Manager listen port")
	listenHost := flag.String("host", "0.0.0.0", "Web Manager listen host")
	dataDir := flag.String("data-dir", defaultDataDir, "Data directory for SQLite database")
	allowPrivateHosts := flag.Bool("allow-private-hosts", false, "allow loopback and private-network Agent URLs")
	resetAdminPassword := flag.Bool("reset-admin-password", false, "reset admin to a new one-time password and exit")
	flag.Parse()
	if *showVersion {
		fmt.Printf("super-proxy-manager %s (%s)\n", version, commit)
		return
	}

	if envPort := os.Getenv("PORT"); envPort != "" {
		value, err := strconv.Atoi(envPort)
		if err != nil {
			log.Fatalf("Invalid PORT: %v", err)
		}
		*port = value
	}
	if envHost := os.Getenv("HOST"); envHost != "" {
		*listenHost = envHost
	}
	if envData := os.Getenv("DATA_DIR"); envData != "" {
		*dataDir = envData
	}

	if *port < 1 || *port > 65535 {
		log.Fatal("port must be between 1 and 65535")
	}
	if *resetAdminPassword {
		dbPath := filepath.Join(*dataDir, "super-proxy-manager.db")
		if _, err := os.Stat(dbPath); err != nil {
			log.Fatalf("Cannot reset admin password: database %q is not accessible: %v", dbPath, err)
		}
	}
	hostpolicy.SetPrivateHostsAllowed(*allowPrivateHosts)
	if hostpolicy.IsPrivateHostsAllowed() {
		log.Println("Private-network Agent URLs are enabled")
	}
	addr := net.JoinHostPort(*listenHost, strconv.Itoa(*port))

	// 1. Initialize SQLite Database
	database, err := db.InitDB(*dataDir)
	if err != nil {
		log.Fatalf("Failed to initialize database in %q: %v", *dataDir, err)
	}
	defer database.Close()
	if *resetAdminPassword {
		temporaryPassword, err := auth.ResetAdminPassword(database)
		if err != nil {
			log.Fatalf("Failed to reset admin password: %v", err)
		}
		fmt.Printf("Admin username: admin\nTemporary password: %s\n", temporaryPassword)
		return
	}

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
