package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/NaNA1337/super-proxy-manager/backend/embedded"
	"github.com/NaNA1337/super-proxy-manager/backend/server"
)

func main() {
	port := flag.Int("port", 8443, "Web Manager listen port")
	host := flag.String("host", "0.0.0.0", "Web Manager listen host")
	flag.Parse()

	envPort := os.Getenv("PORT")
	if envPort != "" {
		_, _ = fmt.Sscanf(envPort, "%d", port)
	}

	distFS, err := embedded.GetDistFS()
	if err != nil {
		log.Printf("[WebManager] Warning: failed to load embedded frontend assets: %v", err)
	}

	addr := fmt.Sprintf("%s:%d", *host, *port)
	srv := server.NewServer(addr, distFS)

	log.Printf("======================================================")
	log.Printf("  Super-Proxy Web Manager / Control Panel             ")
	log.Printf("  Listening on: http://%s                             ", addr)
	log.Printf("  Default Admin: admin / (see $WEB_ADMIN_PASSWORD)   ")
	log.Printf("  Default Readonly: readonly / ($WEB_READONLY_PASSWORD)")
	log.Printf("======================================================")

	if err := srv.Start(); err != nil {
		log.Fatalf("Server exited with error: %v", err)
	}
}
