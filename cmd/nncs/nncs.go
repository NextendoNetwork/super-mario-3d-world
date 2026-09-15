package main

import (
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/NextendoNetwork/super-mario-3d-world/internal/transport"
)

const (
	nncsPrimaryPort   = 10025
	nncsSecondaryPort = 10125
)

type nncsServer struct{ sockets []*net.UDPConn }

func (s *nncsServer) Close() {
	for _, socket := range s.sockets {
		_ = socket.Close()
	}
}

// startNNCS creates the four NAT-check sockets expected by PIA. Bind addresses
// are local interfaces; advertised addresses are the identities visible after
// routing or one-to-one NAT.
func startNNCS(cfg transport.Config, natFile string) (*nncsServer, error) {
	server := &nncsServer{}
	fail := func(err error) (*nncsServer, error) { server.Close(); return nil, err }
	bindIPs := []string{cfg.NNCS1BindIP, cfg.NNCS2BindIP}
	publicIPs := []string{cfg.NNCS1PublicIP, cfg.NNCS2PublicIP}
	recorder := newNATRecorder(natFile)
	alternate := make(map[string]*net.UDPConn, 2)
	for _, ip := range bindIPs {
		conn, err := listenNNCS(ip, 0)
		if err != nil {
			return fail(fmt.Errorf("NNCS alternate %s: %w", ip, err))
		}
		server.sockets = append(server.sockets, conn)
		alternate[ip] = conn
	}
	for index, ip := range bindIPs {
		other := bindIPs[1-index]
		for _, port := range []int{nncsPrimaryPort, nncsSecondaryPort} {
			conn, err := listenNNCS(ip, port)
			if err != nil {
				return fail(fmt.Errorf("NNCS %s:%d: %w", ip, port, err))
			}
			server.sockets = append(server.sockets, conn)
			go serveNNCS(conn, publicIPs[index], publicIPs[1-index], alternate[ip], alternate[other], recorder)
		}
	}
	for _, port := range []int{33334, 33335} {
		conn, err := listenNNCS(bindIPs[0], port)
		if err != nil {
			return fail(fmt.Errorf("NNCS sink %s:%d: %w", bindIPs[0], port, err))
		}
		server.sockets = append(server.sockets, conn)
		go sinkNNCS(conn)
	}
	fmt.Printf("[NNCS] bind=%s/%s advertised=%s/%s\n", bindIPs[0], bindIPs[1], publicIPs[0], publicIPs[1])
	return server, nil
}

func listenNNCS(ip string, port int) (*net.UDPConn, error) {
	return net.ListenUDP("udp4", &net.UDPAddr{IP: net.ParseIP(ip).To4(), Port: port})
}

func serveNNCS(conn *net.UDPConn, advertisedIP, otherAdvertisedIP string, sameAlternate, otherAlternate *net.UDPConn, recorder *natRecorder) {
	buffer := make([]byte, 1024)
	for {
		n, remote, err := conn.ReadFromUDP(buffer)
		if err != nil {
			return
		}
		if n < 16 {
			continue
		}
		kind := binary.BigEndian.Uint32(buffer[:4])
		reply := conn
		responseIP := advertisedIP
		switch kind {
		case 1, 4, 5, 101, 103:
		case 2:
			reply = otherAlternate
			responseIP = otherAdvertisedIP
		case 3, 102:
			reply = sameAlternate
		default:
			continue
		}
		if recorder != nil {
			recorder.record(remote)
		}
		if _, err := reply.WriteToUDP(makeNNCSResponse(kind, remote, responseIP), remote); err != nil {
			fmt.Printf("[NNCS] type=%d response to %s failed: %v\n", kind, remote, err)
		}
	}
}

func makeNNCSResponse(kind uint32, remote *net.UDPAddr, advertisedIP string) []byte {
	response := make([]byte, 16)
	binary.BigEndian.PutUint32(response[0:4], kind)
	binary.BigEndian.PutUint32(response[4:8], uint32(remote.Port))
	copy(response[8:12], remote.IP.To4())
	copy(response[12:16], net.ParseIP(advertisedIP).To4())
	return response
}

func sinkNNCS(conn *net.UDPConn) {
	buffer := make([]byte, 1024)
	for {
		if _, _, err := conn.ReadFromUDP(buffer); err != nil {
			return
		}
	}
}

type natRecorder struct {
	mu        sync.Mutex
	path      string
	endpoints map[string]int
}

func newNATRecorder(path string) *natRecorder {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	return &natRecorder{path: path, endpoints: make(map[string]int)}
}

func (r *natRecorder) record(remote *net.UDPAddr) {
	if remote.IP.String() == "" || remote.Port < 1 {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.endpoints[remote.IP.String()] = remote.Port
	keys := make([]string, 0, len(r.endpoints))
	for key := range r.endpoints {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var output strings.Builder
	for _, key := range keys {
		fmt.Fprintf(&output, "%s %d\n", key, r.endpoints[key])
	}
	if err := os.MkdirAll(filepath.Dir(r.path), 0o755); err != nil {
		return
	}
	_ = os.WriteFile(r.path, []byte(output.String()), 0o600)
}
