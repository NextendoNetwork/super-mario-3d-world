package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func signedToken(secret []byte, payload string) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte("nex:" + payload))
	return "nx2." + base64.RawURLEncoding.EncodeToString([]byte(payload)) + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func TestTokenValidation(t *testing.T) {
	secret := []byte("test-key-not-a-real-secret")
	auth := &accountAuth{secret: secret}
	now := time.Unix(1800000000, 0)
	for _, tc := range []struct {
		name, token string
		valid       bool
	}{
		{"valid", signedToken(secret, "1001.Player.1800000060"), true},
		{"expired", signedToken(secret, "1001.Player.1799999999"), false},
		{"wrong-key", signedToken([]byte("different"), "1001.Player.1800000060"), false},
		{"zero-pid", signedToken(secret, "0.Player.1800000060"), false},
		{"empty-name", signedToken(secret, "1001..1800000060"), false},
		{"bare-pid", "1001", false},
		{"bad-base64", "nx2.!.!", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pid, ok := auth.tokenPID(tc.token, now)
			if ok != tc.valid || (ok && pid != 1001) {
				t.Fatalf("PID=%d valid=%v", pid, ok)
			}
		})
	}
}

func TestAccountGateFailsClosed(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		status     int
		allowed    bool
	}{
		{"allowed", `{"allow":true}`, 200, true},
		{"denied", `{"allow":false}`, 200, false},
		{"server-error", `{"allow":true}`, 500, false},
		{"bad-json", "not json", 200, false},
		{"missing-field", "{}", 200, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/internal/online-check" || r.Method != "POST" || r.Header.Get("X-Internal-Key") != "test-internal-key" {
					t.Error("wrong account request")
				}
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.body)
			}))
			defer server.Close()
			auth := &accountAuth{baseURL: server.URL, internalKey: "test-internal-key", client: server.Client(), inflight: make(chan struct{}, 1)}
			if got := auth.onlineAllowed(1001); got != tc.allowed {
				t.Fatalf("allowed=%v", got)
			}
		})
	}
}

func TestResolveRequiresIdentityAndAccountApproval(t *testing.T) {
	secret := []byte("test-key-not-a-real-secret")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `{"allow":true}`) }))
	auth := &accountAuth{secret: secret, baseURL: server.URL, client: server.Client(), inflight: make(chan struct{}, 1), localPIDs: map[uint64]bool{1002: true}}
	token := signedToken(secret, fmt.Sprintf("1001.Player.%d", time.Now().Add(time.Hour).Unix()))
	pid, key, ok := auth.resolve(token, nil)
	if !ok || pid != 1001 || len(key) != 32 {
		t.Fatal("signed account rejected")
	}
	if _, _, ok := auth.resolve("1001", nil); ok {
		t.Fatal("unlisted bare PID accepted")
	}
	if pid, _, ok := auth.resolve("1002", nil); !ok || pid != 1002 {
		t.Fatal("local allowlisted account rejected")
	}
	if _, _, ok := auth.resolve("anything", nil); ok {
		t.Fatal("anonymous login accepted")
	}
	server.Close()
	if _, _, ok := auth.resolve(token, nil); ok {
		t.Fatal("unreachable account service accepted")
	}
}

func TestLegacyPIDsCannotBeEnabledOutsideLocalMode(t *testing.T) {
	t.Setenv("NEXTENDO_SECRET", "test-key-not-a-real-secret")
	t.Setenv("NEXTENDO_INTERNAL_KEY", "test-internal-key")
	t.Setenv("SM3DW_SAME_HOST_PIDS", "1001,1002")
	if _, err := loadAccountAuth(false); err == nil || !strings.Contains(err.Error(), "same-host") {
		t.Fatal("legacy PID configuration accepted outside local mode")
	}
	if _, err := loadAccountAuth(true); err != nil {
		t.Fatal(err)
	}
}
