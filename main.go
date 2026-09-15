package main

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"

	nex "github.com/NextendoNetwork/nextendo-nex"
	"github.com/NextendoNetwork/super-mario-3d-world/internal/transport"
	"github.com/lxzan/gws"
)

const (
	accessKey  = "67f366f2"
	nexVersion = 40604
	securePID  = 2
)

type config struct {
	transport                         transport.Config
	certFile, keyFile, securePassword string
	authPort, securePort              int
	auth                              *accountAuth
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	auth, secure := endpoints(cfg)
	servers := []*http.Server{
		websocketServer(cfg.transport.NEXBindIP, cfg.authPort, auth),
		websocketServer(cfg.transport.NEXBindIP, cfg.securePort, secure),
	}
	errs := make(chan error, len(servers))
	for _, server := range servers {
		go func(s *http.Server) {
			log.Printf("3D World listening on %s", s.Addr)
			errs <- s.ListenAndServeTLS(cfg.certFile, cfg.keyFile)
		}(server)
	}
	select {
	case err = <-errs:
	case <-ctx.Done():
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, server := range servers {
		_ = server.Shutdown(shutdown)
	}
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func endpoints(cfg config) (*nex.Endpoint, *nex.Endpoint) {
	settings := nex.NewSwitchSettings(accessKey, nexVersion)
	settings.PrudpMinorVersion = 6
	url := nex.NewStationURL("prudp")
	url.Set("address", cfg.transport.NEXAdvertisedIP)
	url.SetInt("port", cfg.securePort)
	for key, value := range map[string]int{"CID": 1, "PID": securePID, "sid": 1, "stream": 10, "type": 2} {
		url.SetInt(key, value)
	}

	auth := nex.NewEndpoint(settings)
	auth.Register(nex.ProtocolTicketGranting, (&nex.AuthConfig{
		Settings: settings, SecurePID: securePID, SecurePassword: cfg.securePassword,
		SecureStationURL: url, ServerName: "Nextendo", SessionKeyLength: 32,
		ResolveUser: cfg.auth.resolve,
	}).Handler())

	secure := nex.NewEndpoint(settings)
	secure.SetSecureAccount(cfg.securePassword, securePID)
	mm := nex.NewMatchmaking()
	mm.JoinRespExistingCount = true
	mm.ParticipationNotificationDelay = 100 * time.Millisecond
	mm.LocalLoopbackStations = cfg.transport.SameHost()
	secure.Register(nex.ProtocolSecureConnection, nex.SecureConnectionHandler())
	secure.Register(nex.ProtocolMatchmakeExtension, mm.ExtensionHandler())
	secure.Register(nex.ProtocolMatchMaking, mm.MatchMakingHandler())
	secure.Register(nex.ProtocolMatchMakingExt, mm.MatchMakingExtHandler())
	secure.Register(nex.ProtocolNATTraversal, natHandler(cfg.transport.SameHost()))
	secure.Register(nex.ProtocolRanking, nex.RankingHandler())
	secure.Register(nex.ProtocolUtility, nex.UtilityHandler())
	secure.OnDisconnect = func(c *nex.Connection) { mm.RemovePlayer(c.PID) }
	secure.StartReaper()
	return auth, secure
}

// Keep the transport bound to the configured interface, including localhost.
func websocketServer(ip string, port int, endpoint *nex.Endpoint) *http.Server {
	transport := nex.NewServer(endpoint)
	upgrader := gws.NewUpgrader(transport, &gws.ServerOption{
		ParallelEnabled: true, Recovery: gws.Recovery,
		ReadBufferSize: 64 * 1024, WriteBufferSize: 64 * 1024,
	})
	return &http.Server{
		Addr:              net.JoinHostPort(ip, strconv.Itoa(port)),
		ReadHeaderTimeout: 10 * time.Second,
		TLSNextProto:      make(map[string]func(*http.Server, *tls.Conn, http.Handler)),
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
				http.NotFound(w, r)
				return
			}
			socket, err := upgrader.Upgrade(w, r)
			if err != nil {
				return
			}
			go socket.ReadLoop()
		}),
	}
}

func loadConfig() (config, error) {
	network, err := transport.LoadFromEnv()
	if err != nil {
		return config{}, err
	}
	cfg := config{
		transport: network,
		certFile:  env("CERT_FILE", "cert.pem"), keyFile: env("KEY_FILE", "key.pem"),
		securePassword: os.Getenv("NEXTENDO_SECURE_PASSWORD"),
	}
	if cfg.authPort, err = port("AUTH_PORT", 443); err != nil {
		return cfg, err
	}
	if cfg.securePort, err = port("SECURE_PORT", 60003); err != nil {
		return cfg, err
	}
	if cfg.authPort == cfg.securePort {
		return cfg, fmt.Errorf("AUTH_PORT and SECURE_PORT must differ")
	}
	if len(cfg.securePassword) < 16 {
		return cfg, fmt.Errorf("set NEXTENDO_SECURE_PASSWORD to at least 16 characters")
	}
	if _, err = tls.LoadX509KeyPair(cfg.certFile, cfg.keyFile); err != nil {
		return cfg, fmt.Errorf("TLS certificate: %w", err)
	}
	cfg.auth, err = loadAccountAuth(cfg.transport.SameHost())
	return cfg, err
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func port(key string, fallback int) (int, error) {
	value, err := strconv.Atoi(env(key, strconv.Itoa(fallback)))
	if err != nil || value < 1 || value > 65535 {
		return 0, fmt.Errorf("%s must be between 1 and 65535", key)
	}
	return value, nil
}
