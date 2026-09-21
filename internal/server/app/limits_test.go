package app

import (
	"net/http"
	"net/http/httptest"
	"net/netip"
	"sync/atomic"
	"testing"
	"time"
)

func TestRequestLimiterRefillAndBoundedIdentities(t *testing.T) {
	now := time.Now().UnixMilli()
	limiter := newRequestLimiter(func() int64 { return now }, 12*time.Second, 5)
	client := netip.MustParseAddr("192.0.2.1")
	for range 5 {
		if !limiter.allow(client) {
			t.Fatal("initial burst rejected")
		}
	}
	if limiter.allow(client) {
		t.Fatal("exhausted client admitted")
	}
	if !limiter.allow(client.Next()) {
		t.Fatal("independent client rejected")
	}
	now += 12_000
	if !limiter.allow(client) || limiter.allow(client) {
		t.Fatal("expected exactly one refilled token")
	}
	for len(limiter.clients) < maxRateLimitClients {
		client = client.Next()
		limiter.allow(client)
	}
	if limiter.allow(client.Next()) {
		t.Fatal("identity table exceeded its bound")
	}
	now += rateLimitIdleTTL.Milliseconds()
	if !limiter.allow(client.Next()) || len(limiter.clients) != 1 {
		t.Fatal("idle identities were not reclaimed")
	}
}

func TestClientAddressTrustBoundary(t *testing.T) {
	s := &Server{}
	s.config.TrustedProxyCIDRs = []netip.Prefix{netip.MustParsePrefix("127.0.0.1/32"), netip.MustParsePrefix("::1/128")}
	for _, test := range []struct{ name, peer, forwarded, want string }{
		{"direct spoof", "192.0.2.1:1234", "198.51.100.1", "192.0.2.1"},
		{"proxy", "127.0.0.1:1234", "192.0.2.1", "192.0.2.1"},
		{"prepended spoof", "127.0.0.1:1234", "198.51.100.1, 192.0.2.1", "192.0.2.1"},
		{"trusted chain", "[::1]:1234", "192.0.2.1, 127.0.0.1", "192.0.2.1"},
		{"malformed", "127.0.0.1:1234", "not-an-ip", "127.0.0.1"},
		{"missing", "127.0.0.1:1234", "", "127.0.0.1"},
		{"ipv6", "[2001:db8::1]:1234", "192.0.2.1", "2001:db8::1"},
		{"mapped", "[::ffff:192.0.2.1]:1234", "198.51.100.1", "192.0.2.1"},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/api/site-access", nil)
			request.RemoteAddr = test.peer
			request.Header.Set("X-Forwarded-For", test.forwarded)
			if got := s.clientAddress(request); got != netip.MustParseAddr(test.want) {
				t.Fatalf("address = %s, want %s", got, test.want)
			}
		})
	}
}

func TestHTTPAdmissionLimits(t *testing.T) {
	var now atomic.Int64
	now.Store(time.Now().UnixMilli())
	h := start(t, Options{Now: now.Load})
	// Origin failures must not consume a client's login allowance.
	for range 6 {
		h.do(http.MethodPost, "/api/site-access", withOrigin("https://invalid.test"), withBearer(testAccessPassword)).expectStatus(http.StatusForbidden)
	}
	for range 5 {
		h.do(http.MethodPost, "/api/site-access", withOrigin(allowedOrigin), withBearer("wrong")).expectStatus(http.StatusUnauthorized)
	}
	h.login().expectStatus(http.StatusTooManyRequests).expectHeader("Retry-After", "60")
	h.do(http.MethodGet, "/api/site-access").expectStatus(http.StatusOK)
	now.Add(12_000)
	cookie := h.cookie()
	// Unauthorized room creation must not consume the authenticated allowance.
	for range 11 {
		h.createRoom(roomRequest{}).expectStatus(http.StatusUnauthorized)
	}
	for range 10 {
		h.createRoom(roomRequest{cookie: cookie, codeEntryPolicy: "private"}).expectStatus(http.StatusCreated)
	}
	h.createRoom(roomRequest{cookie: cookie}).expectStatus(http.StatusTooManyRequests).expectHeader("Retry-After", "60")
	now.Add(6_000)
	h.createRoom(roomRequest{cookie: cookie, codeEntryPolicy: "private"}).expectStatus(http.StatusCreated)
}
