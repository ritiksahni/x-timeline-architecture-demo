package api

import (
	"sort"
	"sync"
	"time"

	"github.com/ritik/twitter-fan-out/internal/timeline"
)

// MetricsStore stores operation metrics for analysis
type MetricsStore struct {
	mu           sync.RWMutex
	writeMetrics []*timeline.OperationMetrics
	readMetrics  []*timeline.OperationMetrics
	maxSize      int
}

// NewMetricsStore creates a new MetricsStore
func NewMetricsStore() *MetricsStore {
	return &MetricsStore{
		writeMetrics: make([]*timeline.OperationMetrics, 0),
		readMetrics:  make([]*timeline.OperationMetrics, 0),
		maxSize:      10000, // Keep last 10k metrics
	}
}

// AddWriteMetric adds a write operation metric
func (ms *MetricsStore) AddWriteMetric(m *timeline.OperationMetrics) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	ms.writeMetrics = append(ms.writeMetrics, m)
	if len(ms.writeMetrics) > ms.maxSize {
		ms.writeMetrics = ms.writeMetrics[len(ms.writeMetrics)-ms.maxSize:]
	}
}

// AddReadMetric adds a read operation metric
func (ms *MetricsStore) AddReadMetric(m *timeline.OperationMetrics) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	ms.readMetrics = append(ms.readMetrics, m)
	if len(ms.readMetrics) > ms.maxSize {
		ms.readMetrics = ms.readMetrics[len(ms.readMetrics)-ms.maxSize:]
	}
}

// MetricsSummary holds aggregated metrics
type MetricsSummary struct {
	TotalWrites     int                        `json:"total_writes"`
	TotalReads      int                        `json:"total_reads"`
	ByStrategy      map[string]*StrategySummary `json:"by_strategy"`
	RecentWriteP50  string                     `json:"recent_write_p50"`
	RecentWriteP95  string                     `json:"recent_write_p95"`
	RecentWriteP99  string                     `json:"recent_write_p99"`
	RecentReadP50   string                     `json:"recent_read_p50"`
	RecentReadP95   string                     `json:"recent_read_p95"`
	RecentReadP99   string                     `json:"recent_read_p99"`
}

// StrategySummary holds metrics for a specific strategy
type StrategySummary struct {
	WriteCount      int     `json:"write_count"`
	ReadCount       int     `json:"read_count"`
	WriteLatencyAvg string  `json:"write_latency_avg"`
	WriteLatencyP50 string  `json:"write_latency_p50"`
	WriteLatencyP95 string  `json:"write_latency_p95"`
	WriteLatencyP99 string  `json:"write_latency_p99"`
	ReadLatencyAvg  string  `json:"read_latency_avg"`
	ReadLatencyP50  string  `json:"read_latency_p50"`
	ReadLatencyP95  string  `json:"read_latency_p95"`
	ReadLatencyP99  string  `json:"read_latency_p99"`
	AvgFanOutCount  float64 `json:"avg_fan_out_count"`
	CacheHitRate    float64 `json:"cache_hit_rate"`
}

// GetSummary returns aggregated metrics
func (ms *MetricsStore) GetSummary() *MetricsSummary {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	summary := &MetricsSummary{
		TotalWrites: len(ms.writeMetrics),
		TotalReads:  len(ms.readMetrics),
		ByStrategy:  make(map[string]*StrategySummary),
	}

	// Group by strategy
	writeByStrategy := make(map[string][]*timeline.OperationMetrics)
	readByStrategy := make(map[string][]*timeline.OperationMetrics)

	for _, m := range ms.writeMetrics {
		writeByStrategy[m.Strategy] = append(writeByStrategy[m.Strategy], m)
	}
	for _, m := range ms.readMetrics {
		readByStrategy[m.Strategy] = append(readByStrategy[m.Strategy], m)
	}

	// Calculate per-strategy metrics
	strategies := []string{"fanout_write", "fanout_read", "hybrid"}
	for _, strategy := range strategies {
		writes := writeByStrategy[strategy]
		reads := readByStrategy[strategy]

		ss := &StrategySummary{
			WriteCount: len(writes),
			ReadCount:  len(reads),
		}

		if len(writes) > 0 {
			writeDurations := make([]time.Duration, len(writes))
			var totalFanOut int
			for i, m := range writes {
				writeDurations[i] = m.Duration()
				totalFanOut += m.FanOutCount
			}
			ss.WriteLatencyAvg = avgDuration(writeDurations).String()
			ss.WriteLatencyP50 = percentileDuration(writeDurations, 50).String()
			ss.WriteLatencyP95 = percentileDuration(writeDurations, 95).String()
			ss.WriteLatencyP99 = percentileDuration(writeDurations, 99).String()
			ss.AvgFanOutCount = float64(totalFanOut) / float64(len(writes))
		}

		if len(reads) > 0 {
			readDurations := make([]time.Duration, len(reads))
			var cacheHits int
			for i, m := range reads {
				readDurations[i] = m.Duration()
				if m.CacheHit {
					cacheHits++
				}
			}
			ss.ReadLatencyAvg = avgDuration(readDurations).String()
			ss.ReadLatencyP50 = percentileDuration(readDurations, 50).String()
			ss.ReadLatencyP95 = percentileDuration(readDurations, 95).String()
			ss.ReadLatencyP99 = percentileDuration(readDurations, 99).String()
			ss.CacheHitRate = float64(cacheHits) / float64(len(reads))
		}

		summary.ByStrategy[strategy] = ss
	}

	// Calculate recent overall metrics (last 100)
	recentWrites := ms.writeMetrics
	if len(recentWrites) > 100 {
		recentWrites = recentWrites[len(recentWrites)-100:]
	}
	recentReads := ms.readMetrics
	if len(recentReads) > 100 {
		recentReads = recentReads[len(recentReads)-100:]
	}

	if len(recentWrites) > 0 {
		durations := make([]time.Duration, len(recentWrites))
		for i, m := range recentWrites {
			durations[i] = m.Duration()
		}
		summary.RecentWriteP50 = percentileDuration(durations, 50).String()
		summary.RecentWriteP95 = percentileDuration(durations, 95).String()
		summary.RecentWriteP99 = percentileDuration(durations, 99).String()
	}

	if len(recentReads) > 0 {
		durations := make([]time.Duration, len(recentReads))
		for i, m := range recentReads {
			durations[i] = m.Duration()
		}
		summary.RecentReadP50 = percentileDuration(durations, 50).String()
		summary.RecentReadP95 = percentileDuration(durations, 95).String()
		summary.RecentReadP99 = percentileDuration(durations, 99).String()
	}

	return summary
}

// RecentMetric represents a single metric point for the UI
type RecentMetric struct {
	Timestamp   string `json:"timestamp"`
	Strategy    string `json:"strategy"`
	Operation   string `json:"operation"`
	DurationMs  int64  `json:"duration_ms"`
	FanOutCount int    `json:"fan_out_count"`
	CacheHit    bool   `json:"cache_hit"`
	Success     bool   `json:"success"`
}

// GetRecent returns recent metrics for real-time display
func (ms *MetricsStore) GetRecent(limit int) []RecentMetric {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	// Combine and sort by time
	all := make([]*timeline.OperationMetrics, 0, len(ms.writeMetrics)+len(ms.readMetrics))
	all = append(all, ms.writeMetrics...)
	all = append(all, ms.readMetrics...)

	sort.Slice(all, func(i, j int) bool {
		return all[i].StartTime.After(all[j].StartTime)
	})

	if len(all) > limit {
		all = all[:limit]
	}

	result := make([]RecentMetric, len(all))
	for i, m := range all {
		result[i] = RecentMetric{
			Timestamp:   m.StartTime.Format(time.RFC3339Nano),
			Strategy:    m.Strategy,
			Operation:   m.Operation,
			DurationMs:  m.Duration().Milliseconds(),
			FanOutCount: m.FanOutCount,
			CacheHit:    m.CacheHit,
			Success:     m.Success,
		}
	}

	return result
}

// Clear clears all stored metrics
func (ms *MetricsStore) Clear() {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	ms.writeMetrics = make([]*timeline.OperationMetrics, 0)
	ms.readMetrics = make([]*timeline.OperationMetrics, 0)
}

// Helper functions
func avgDuration(durations []time.Duration) time.Duration {
	if len(durations) == 0 {
		return 0
	}
	var total time.Duration
	for _, d := range durations {
		total += d
	}
	return total / time.Duration(len(durations))
}

func percentileDuration(durations []time.Duration, p int) time.Duration {
	if len(durations) == 0 {
		return 0
	}

	sorted := make([]time.Duration, len(durations))
	copy(sorted, durations)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i] < sorted[j]
	})

	idx := (p * len(sorted)) / 100
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}
