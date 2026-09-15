package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type accountAuth struct {
	secret               []byte
	baseURL, internalKey string
	localPIDs            map[uint64]bool
	client               *http.Client
	inflight             chan struct{}
}

func loadAccountAuth(sameHost bool) (*accountAuth, error) {
	secret := []byte(os.Getenv("NEXTENDO_SECRET"))
	if len(secret) == 0 {
		data, err := os.ReadFile(env("NEXTENDO_SECRET_FILE", "nextendo_secret.key"))
		if err != nil {
			return nil, fmt.Errorf("set NEXTENDO_SECRET or NEXTENDO_SECRET_FILE")
		}
		secret, err = hex.DecodeString(strings.TrimSpace(string(data)))
		if err != nil {
			return nil, fmt.Errorf("NEXTENDO_SECRET_FILE must contain a hex key")
		}
	}
	if len(secret) < 16 {
		return nil, fmt.Errorf("NEXTENDO_SECRET must contain at least 16 bytes")
	}
	baseURL := strings.TrimRight(env("NEXTENDO_ACCOUNT_URL", "http://127.0.0.1:8080"), "/")
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.User != nil {
		return nil, fmt.Errorf("invalid NEXTENDO_ACCOUNT_URL")
	}
	key := os.Getenv("NEXTENDO_INTERNAL_KEY")
	if key == "" {
		return nil, fmt.Errorf("NEXTENDO_INTERNAL_KEY is required")
	}
	localPIDs := make(map[uint64]bool)
	if values := os.Getenv("SM3DW_SAME_HOST_PIDS"); values != "" {
		if !sameHost {
			return nil, fmt.Errorf("SM3DW_SAME_HOST_PIDS requires TRANSPORT_PROFILE=same-host")
		}
		for _, value := range strings.Split(values, ",") {
			pid, err := strconv.ParseUint(strings.TrimSpace(value), 10, 64)
			if err != nil || pid == 0 {
				return nil, fmt.Errorf("invalid SM3DW_SAME_HOST_PIDS entry")
			}
			localPIDs[pid] = true
		}
	}
	return &accountAuth{
		secret: secret, baseURL: baseURL, internalKey: key, localPIDs: localPIDs,
		client:   &http.Client{Timeout: 3 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }},
		inflight: make(chan struct{}, 16),
	}, nil
}

func (a *accountAuth) resolve(username string, _ []byte) (uint64, []byte, bool) {
	pid, ok := a.tokenPID(username, time.Now())
	if !ok && !strings.HasPrefix(username, "nx2.") {
		// Older clients send a bare PID. Allow only named accounts in a loopback test.
		pid, _ = strconv.ParseUint(username, 10, 64)
		ok = a.localPIDs[pid]
	}
	if !ok || !a.onlineAllowed(pid) {
		return 0, nil, false
	}
	key := sha256.Sum256([]byte("nextendo-src:" + username))
	return pid, key[:], true
}

// The account service signs nx2 tokens with HMAC-SHA256 and the "nex:" prefix.
func (a *accountAuth) tokenPID(token string, now time.Time) (uint64, bool) {
	if len(a.secret) == 0 || !strings.HasPrefix(token, "nx2.") {
		return 0, false
	}
	parts := strings.Split(strings.TrimPrefix(token, "nx2."), ".")
	if len(parts) != 2 {
		return 0, false
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return 0, false
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return 0, false
	}
	mac := hmac.New(sha256.New, a.secret)
	_, _ = mac.Write(append([]byte("nex:"), raw...))
	if !hmac.Equal(signature, mac.Sum(nil)) {
		return 0, false
	}
	fields := strings.Split(string(raw), ".")
	if len(fields) != 3 || fields[1] == "" {
		return 0, false
	}
	pid, err := strconv.ParseUint(fields[0], 10, 64)
	if err != nil || pid == 0 {
		return 0, false
	}
	expiry, err := strconv.ParseInt(fields[2], 10, 64)
	return pid, err == nil && expiry > now.Unix()
}

func (a *accountAuth) onlineAllowed(pid uint64) bool {
	select {
	case a.inflight <- struct{}{}:
		defer func() { <-a.inflight }()
	default:
		return false
	}
	body, _ := json.Marshal(map[string]any{"pid": pid, "kind": "ryujinx"})
	req, err := http.NewRequest("POST", a.baseURL+"/internal/online-check", bytes.NewReader(body))
	if err != nil {
		return false
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Key", a.internalKey)
	response, err := a.client.Do(req)
	if err != nil {
		return false
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return false
	}
	var result struct {
		Allow bool `json:"allow"`
	}
	return json.NewDecoder(io.LimitReader(response.Body, 4096)).Decode(&result) == nil && result.Allow
}
