package resolved

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/anomalyco/android-resolved/internal/cache"
	"github.com/anomalyco/android-resolved/internal/config"
	"github.com/anomalyco/android-resolved/internal/dnsmsg"
	"github.com/anomalyco/android-resolved/internal/resolver"
	"github.com/anomalyco/android-resolved/internal/router"
	"github.com/anomalyco/android-resolved/internal/server"
	"github.com/miekg/dns"

	statstracker "github.com/anomalyco/android-resolved/internal/stats"
)

var (
	mu         sync.RWMutex
	tunHdl     *server.TunHandler
	rtr        *router.TrieRouter
	cch        *cache.Cache
	stats      *statstracker.StatsTracker
	started    bool
	ownerRules map[string][]config.Rule
	ownerOrder []string
)

type Stats struct {
	CacheSize    int    `json:"cache_size"`
	Rules        int    `json:"rules"`
	Running      bool   `json:"running"`
	TotalQueries uint64 `json:"total_queries"`
	CacheHits    uint64 `json:"cache_hits"`
	CacheMisses  uint64 `json:"cache_misses"`
}

func initEngine(cfg *config.Config) error {
	log.Printf("resolved: initializing engine with %d rules", len(cfg.Rules))
	rtr = router.NewTrieRouter(cfg.Rules)
	cch = cache.New(cfg.Cache.Size, cfg.Cache.MinTTL*time.Second, cfg.Cache.MaxTTL*time.Second)
	stats = statstracker.NewStatsTracker()
	ownerRules = make(map[string][]config.Rule)
	ownerOrder = nil
	return nil
}

func StartWithTunFd(fd int) error {
	mu.Lock()
	defer mu.Unlock()

	if started {
		return fmt.Errorf("already running")
	}

	log.Printf("resolved: StartWithTunFd(fd=%d)", fd)

	cfg := config.Default()
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}
	if err := initEngine(cfg); err != nil {
		return err
	}

	var err error
	tunHdl, err = server.NewTunHandler(fd, rtr, cch, stats, 5*time.Second)
	if err != nil {
		return fmt.Errorf("tun handler: %w", err)
	}
	tunHdl.Start()

	started = true
	log.Printf("resolved: engine started successfully")
	return nil
}

func AddRule(owner string, ruleJSON string) error {
	mu.Lock()
	defer mu.Unlock()

	if !started {
		return fmt.Errorf("not running")
	}

	var rule config.Rule
	if err := json.Unmarshal([]byte(ruleJSON), &rule); err != nil {
		return fmt.Errorf("invalid rule: %w", err)
	}
	if rule.Domain == "" {
		return fmt.Errorf("rule domain is required")
	}
	switch rule.Protocol {
	case config.ProtocolUDP, config.ProtocolTCP, config.ProtocolMDNS:
	default:
		return fmt.Errorf("unknown protocol %q", rule.Protocol)
	}
	if rule.Protocol != config.ProtocolMDNS && rule.Upstream == "" {
		return fmt.Errorf("upstream is required for protocol %s", rule.Protocol)
	}

	if _, exists := ownerRules[owner]; !exists {
		ownerOrder = append(ownerOrder, owner)
	}
	ownerRules[owner] = append(ownerRules[owner], rule)

	rebuildRouter()
	return nil
}

func FlushRules(owner string) error {
	mu.Lock()
	defer mu.Unlock()

	if !started {
		return fmt.Errorf("not running")
	}

	delete(ownerRules, owner)
	filtered := make([]string, 0, len(ownerOrder))
	for _, o := range ownerOrder {
		if o != owner {
			filtered = append(filtered, o)
		}
	}
	ownerOrder = filtered

	rebuildRouter()
	return nil
}

func rebuildRouter() {
	var allRules []config.Rule
	seen := make(map[string]bool)

	for _, r := range config.Default().Rules {
		seen[r.Domain] = true
		allRules = append(allRules, r)
	}

	for _, owner := range ownerOrder {
		for _, r := range ownerRules[owner] {
			if !seen[r.Domain] {
				seen[r.Domain] = true
				allRules = append(allRules, r)
			}
		}
	}

	rtr.Update(allRules)
}

func Stop() {
	mu.Lock()
	defer mu.Unlock()

	if !started {
		return
	}

	if tunHdl != nil {
		tunHdl.Stop()
		tunHdl = nil
	}
	started = false
}

func GetStats() string {
	mu.Lock()
	defer mu.Unlock()

	s := Stats{Running: started}
	if rtr != nil {
		s.Rules = len(rtr.Rules())
	}
	if stats != nil {
		s.TotalQueries = stats.TotalQueries()
		s.CacheHits = stats.CacheHits()
		s.CacheMisses = stats.CacheMisses()
	}
	data, _ := json.Marshal(s)
	return string(data)
}

func IsRunning() bool {
	mu.Lock()
	defer mu.Unlock()
	return started
}

func ResolveHostname(name string, qtype int) string {
	mu.RLock()
	r := rtr
	c := cch
	s := stats
	mu.RUnlock()

	if r == nil || c == nil || s == nil {
		return `{"error":"not running"}`
	}

	msg := dnsmsg.Question(name, uint16(qtype))

	reply := resolveMsg(msg, r, c, s)
	if reply == nil {
		return `{"error":"resolution failed"}`
	}

	type answer struct {
		Name string `json:"name"`
		Type int    `json:"type"`
		TTL  uint32 `json:"ttl"`
		Data string `json:"data"`
	}

	var answers []answer
	for _, rr := range reply.Answer {
		answers = append(answers, answer{
			Name: rr.Header().Name,
			Type: int(rr.Header().Rrtype),
			TTL:  rr.Header().Ttl,
			Data: rr.String(),
		})
	}

	data, _ := json.Marshal(answers)
	return string(data)
}

func GetStatusJSON() string {
	mu.RLock()
	running := started
	stat := stats
	rtrRules := rtr
	mu.RUnlock()

	if stat == nil {
		return `{"error":"not running"}`
	}

	rulesCount := 0
	if rtrRules != nil {
		rulesCount = len(rtrRules.Rules())
	}

	status := struct {
		Running      bool   `json:"running"`
		TotalQueries uint64 `json:"total_queries"`
		CacheHits    uint64 `json:"cache_hits"`
		CacheMisses  uint64 `json:"cache_misses"`
		Rules        int    `json:"rules"`
	}{
		Running:      running,
		TotalQueries: stat.TotalQueries(),
		CacheHits:    stat.CacheHits(),
		CacheMisses:  stat.CacheMisses(),
		Rules:        rulesCount,
	}

	data, _ := json.MarshalIndent(status, "", "  ")
	return string(data)
}

func FlushCaches() error {
	mu.RLock()
	c := cch
	mu.RUnlock()

	if c == nil {
		return fmt.Errorf("not running")
	}
	c.Clear()
	return nil
}

func ResetStatistics() error {
	mu.RLock()
	s := stats
	mu.RUnlock()

	if s == nil {
		return fmt.Errorf("not running")
	}
	s.Reset()
	return nil
}

func resolveMsg(msg *dns.Msg, rtr router.Router, cch *cache.Cache, stats *statstracker.StatsTracker) *dns.Msg {
	if msg == nil || len(msg.Question) == 0 {
		return nil
	}

	qname := dnsmsg.ExtractQName(msg)
	qtype := dnsmsg.ExtractQType(msg)

	stats.RecordQuery()

	if cached := cch.Get(qname, qtype); cached != nil {
		stats.RecordCacheHit()
		cached.Id = msg.Id
		return cached
	}
	stats.RecordCacheMiss()

	match := rtr.Route(qname)
	if match == nil {
		return dnsmsg.Refused(msg)
	}

	reslv, err := resolver.NewFromMatch(match, 5*time.Second)
	if err != nil {
		return dnsmsg.ServFail(msg)
	}

	reply, err := reslv.Exchange(msg)
	if err != nil {
		return dnsmsg.ServFail(msg)
	}

	stats.RecordUpstream(match.Upstream)
	cch.Set(qname, qtype, reply)

	return reply
}
