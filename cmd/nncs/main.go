package main

import (
	"context"
	"log"
	"os"
	"os/signal"

	"github.com/NextendoNetwork/super-mario-3d-world/internal/transport"
)

func main() {
	cfg, err := transport.LoadFromEnv()
	if err != nil {
		log.Fatal(err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	server, err := startNNCS(cfg, cfg.NATFile)
	if err != nil {
		log.Fatal(err)
	}
	defer server.Close()
	<-ctx.Done()
}
