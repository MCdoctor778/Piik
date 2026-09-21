package app

import (
	"net"
	"net/http"
	"net/netip"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

const (
	maxRateLimitClients = 4096
	rateLimitIdleTTL    = 10 * time.Minute
)

type requestLimitEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// Bounded, process-local admission for expensive or authority-creating HTTP
// requests. Full maps reject new identities instead of evicting active limits.
type requestLimiter struct {
	mu        sync.Mutex
	now       func() int64
	interval  time.Duration
	burst     int
	clients   map[netip.Addr]*requestLimitEntry
	lastSweep time.Time
}

func newRequestLimiter(now func() int64, interval time.Duration, burst int) *requestLimiter {
	return &requestLimiter{now: now, interval: interval, burst: burst, clients: make(map[netip.Addr]*requestLimitEntry)}
}

func (limiter *requestLimiter) allow(address netip.Addr) bool {
	// ponytail: one lock and at most 4,096 entries; shard only if contention is measured.
	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	now := time.UnixMilli(limiter.now())
	if limiter.lastSweep.IsZero() || now.Sub(limiter.lastSweep) >= time.Minute {
		for key, entry := range limiter.clients {
			if now.Sub(entry.lastSeen) >= rateLimitIdleTTL {
				delete(limiter.clients, key)
			}
		}
		limiter.lastSweep = now
	}
	entry := limiter.clients[address]
	if entry == nil {
		if len(limiter.clients) >= maxRateLimitClients {
			return false
		}
		entry = &requestLimitEntry{limiter: rate.NewLimiter(rate.Every(limiter.interval), limiter.burst)}
		limiter.clients[address] = entry
	}
	entry.lastSeen = now
	return entry.limiter.AllowN(now, 1)
}

func (s *Server) allowLimitedRequest(writer http.ResponseWriter, request *http.Request, limiter *requestLimiter) bool {
	if limiter.allow(s.clientAddress(request)) {
		return true
	}
	// Also covers temporary identity-table saturation. No client addresses are logged.
	writer.Header().Set("Retry-After", "60")
	sendJSON(writer, http.StatusTooManyRequests, errorBody{"Too many requests; try again later"})
	return false
}

func (s *Server) trustedProxy(address netip.Addr) bool {
	for _, network := range s.config.TrustedProxyCIDRs {
		if network.Contains(address) {
			return true
		}
	}
	return false
}

func (s *Server) clientAddress(request *http.Request) netip.Addr {
	host, _, err := net.SplitHostPort(request.RemoteAddr)
	if err != nil {
		host = request.RemoteAddr
	}
	peer, err := netip.ParseAddr(host)
	if err != nil {
		return netip.Addr{} // Malformed peers share a limited bucket, never bypass it.
	}
	peer = peer.Unmap()
	if !s.trustedProxy(peer) {
		return peer
	}
	forwarded := strings.Join(request.Header.Values("X-Forwarded-For"), ",")
	if forwarded == "" || len(forwarded) > 4096 {
		return peer
	}
	chain := strings.Split(forwarded, ",")
	current := peer
	// Walk toward the client; never trust entries preceding an untrusted hop.
	for index := len(chain) - 1; index >= 0 && s.trustedProxy(current); index-- {
		address, err := netip.ParseAddr(strings.TrimSpace(chain[index]))
		if err != nil {
			return peer
		}
		current = address.Unmap()
	}
	return current
}
