//go:build tui
// +build tui

// Package tui 提供实时终端仪表板（编译标签：tui）。
//
// 通过 bubbletea 框架构建，显示代理服务器的实时统计信息：
//   - 请求总数 / token 用量 / 累计费用
//   - 最近请求日志（滚动列表）
//   - 提供商和模型状态
//
// 使用方式：ais monitor
//
// 编译标签：//go:build tui
package tui

import (
	"sync"
	"sync/atomic"
	"time"
)

// RequestSummary 是单次请求的摘要，由 handler 在请求完成后调用 Record 写入。
type RequestSummary struct {
	Time             time.Time
	Model            string
	Status           int
	PromptTokens     int
	CompletionTokens int
	CostUSD          float64
	DurationMS       int64
	Stream           bool
}

// StatsCollector 线程安全地收集代理请求的实时统计信息。
type StatsCollector struct {
	mu             sync.Mutex
	startTime      time.Time
	totalReqs      int64
	totalPrompt    int64
	totalCompletion int64
	totalCost      float64 // separate field to avoid atomic on float
	costMu         sync.Mutex
	recent         []RequestSummary // ring buffer
	recentIdx      int
	recentMax      int
}

// NewStatsCollector 创建容量为 recentCap 的统计收集器。
func NewStatsCollector(recentCap int) *StatsCollector {
	return &StatsCollector{
		startTime: time.Now(),
		recentMax: recentCap,
		recent:    make([]RequestSummary, recentCap),
	}
}

// Record 记录一次请求（供 proxy handler 调用）。线程安全。
func (s *StatsCollector) Record(r RequestSummary) {
	atomic.AddInt64(&s.totalReqs, 1)
	atomic.AddInt64(&s.totalPrompt, int64(r.PromptTokens))
	atomic.AddInt64(&s.totalCompletion, int64(r.CompletionTokens))

	s.costMu.Lock()
	s.totalCost += r.CostUSD
	s.costMu.Unlock()

	s.mu.Lock()
	s.recent[s.recentIdx%s.recentMax] = r
	s.recentIdx++
	s.mu.Unlock()
}

// Snapshot 返回当前统计信息的只读快照。
func (s *StatsCollector) Snapshot() StatsSnapshot {
	s.mu.Lock()
	n := min(s.recentIdx, s.recentMax)
	recent := make([]RequestSummary, n)
	// Copy in reverse (newest first)
	for i := 0; i < n; i++ {
		idx := (s.recentIdx - 1 - i) % s.recentMax
		recent[i] = s.recent[idx]
	}
	s.mu.Unlock()

	s.costMu.Lock()
	cost := s.totalCost
	s.costMu.Unlock()

	return StatsSnapshot{
		Uptime:           time.Since(s.startTime),
		TotalReqs:        atomic.LoadInt64(&s.totalReqs),
		TotalPrompt:      atomic.LoadInt64(&s.totalPrompt),
		TotalCompletion:  atomic.LoadInt64(&s.totalCompletion),
		TotalCost:        cost,
		Recent:           recent,
	}
}

// StatsSnapshot 是不可变的统计快照。
type StatsSnapshot struct {
	Uptime           time.Duration
	TotalReqs        int64
	TotalPrompt      int64
	TotalCompletion  int64
	TotalCost        float64
	Recent           []RequestSummary
}
