package stats

import (
	"sync"
	"sync/atomic"
)

type StatsTracker struct {
	totalQueries  atomic.Uint64
	cacheHits     atomic.Uint64
	cacheMisses   atomic.Uint64
	upstreamStats sync.Map
}

type upstreamStats struct {
	queries atomic.Uint64
}

func NewStatsTracker() *StatsTracker {
	return &StatsTracker{}
}

func (s *StatsTracker) RecordQuery() {
	s.totalQueries.Add(1)
}

func (s *StatsTracker) RecordCacheHit() {
	s.cacheHits.Add(1)
}

func (s *StatsTracker) RecordCacheMiss() {
	s.cacheMisses.Add(1)
}

func (s *StatsTracker) RecordUpstream(addr string) {
	val, _ := s.upstreamStats.LoadOrStore(addr, &upstreamStats{})
	val.(*upstreamStats).queries.Add(1)
}

func (s *StatsTracker) TotalQueries() uint64 {
	return s.totalQueries.Load()
}

func (s *StatsTracker) CacheHits() uint64 {
	return s.cacheHits.Load()
}

func (s *StatsTracker) CacheMisses() uint64 {
	return s.cacheMisses.Load()
}

func (s *StatsTracker) Reset() {
	s.totalQueries.Store(0)
	s.cacheHits.Store(0)
	s.cacheMisses.Store(0)
	s.upstreamStats.Range(func(key, value any) bool {
		s.upstreamStats.Delete(key)
		return true
	})
}
