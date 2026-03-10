package util

import (
	"fmt"
	"math/rand"
	"sync"
	"time"
)

var (
	rngMu sync.Mutex
	rng   = rand.New(rand.NewSource(time.Now().UnixNano()))
)

var userAgents = []string{
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/123.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 14_5) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Safari/605.1.15",
	"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36",
	"Mozilla/5.0 (iPhone; CPU iPhone OS 17_4 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.4 Mobile/15E148 Safari/604.1",
}

var ipPrefixes = []string{
	"101.71",
	"106.120",
	"111.13",
	"117.136",
	"120.244",
	"223.104",
}

func RandomUserAgent() string {
	rngMu.Lock()
	defer rngMu.Unlock()
	return userAgents[rng.Intn(len(userAgents))]
}

func RandomIPv4() string {
	rngMu.Lock()
	defer rngMu.Unlock()
	prefix := ipPrefixes[rng.Intn(len(ipPrefixes))]
	return fmt.Sprintf("%s.%d.%d", prefix, rng.Intn(255), rng.Intn(255))
}

func RandomInt(n int) int {
	rngMu.Lock()
	defer rngMu.Unlock()
	if n <= 1 {
		return 0
	}
	return rng.Intn(n)
}
