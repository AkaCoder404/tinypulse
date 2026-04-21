package monitor

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"tinypulse/internal/db"
	"tinypulse/internal/model"
)

type EndpointStats struct {
	LastStatusCode *int
	LastResponseMs *int
	LastIsUp       *bool
	LastCheckedAt  *time.Time

	RecentChecks []model.MinimalCheck
	Uptime24h    *float64
	Uptime30d    *float64
}

type StateManager struct {
	mu    sync.RWMutex
	state map[int64]*EndpointStats
	db    *db.DB
}

func NewStateManager(database *db.DB) *StateManager {
	return &StateManager{
		state: make(map[int64]*EndpointStats),
		db:    database,
	}
}

func (sm *StateManager) Hydrate(ctx context.Context) error {
	slog.Info("hydrating state manager from database...")

	endpoints, err := sm.db.ListActiveEndpoints(ctx)
	if err != nil {
		return err
	}

	sm.mu.Lock()
	defer sm.mu.Unlock()

	// TODO(Performance): This currently executes 3 queries per endpoint (O(N)). 
	// For <1000 endpoints, SQLite handles this sequentially in ~100ms, making it acceptable for startup.
	// If TinyPulse scales to thousands of endpoints, this loop should be refactored into two bulk queries:
	// 1. Conditional aggregation for 24h/30d uptimes (SELECT ... AVG(CASE WHEN ...)).
	// 2. Window functions for the recent 50 checks (ROW_NUMBER() OVER (PARTITION BY endpoint_id)).
	for _, ep := range endpoints {
		stats := &EndpointStats{
			RecentChecks: make([]model.MinimalCheck, 0),
		}

		recent, err := sm.db.GetChecksByEndpoint(ctx, ep.ID, 50)
		if err == nil && len(recent) > 0 {
			// Populate recent checks in chronological order
			for i := len(recent) - 1; i >= 0; i-- {
				stats.RecentChecks = append(stats.RecentChecks, model.MinimalCheck{
					IsUp:           recent[i].IsUp,
					CheckedAt:      recent[i].CheckedAt,
					StatusCode:     recent[i].StatusCode,
					ResponseTimeMs: recent[i].ResponseTimeMs,
				})
			}

			// Latest status
			latest := recent[0]
			stats.LastStatusCode = latest.StatusCode
			
			respMs := latest.ResponseTimeMs
			stats.LastResponseMs = &respMs
			
			isUp := latest.IsUp
			stats.LastIsUp = &isUp
			
			checkedAt := latest.CheckedAt
			stats.LastCheckedAt = &checkedAt
		}

		up24, err := sm.db.CalculateUptime(ctx, ep.ID, 24)
		if err == nil {
			stats.Uptime24h = up24
		}
		
		up30, err := sm.db.CalculateUptime(ctx, ep.ID, 24*30)
		if err == nil {
			stats.Uptime30d = up30
		}

		sm.state[ep.ID] = stats
	}

	slog.Info("state manager hydrated successfully", "endpoints", len(endpoints))
	return nil
}

func (sm *StateManager) UpdateOnCheck(check *model.Check) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	stats, exists := sm.state[check.EndpointID]
	if !exists {
		stats = &EndpointStats{
			RecentChecks: make([]model.MinimalCheck, 0),
		}
		sm.state[check.EndpointID] = stats
	}

	stats.LastStatusCode = check.StatusCode
	
	respMs := check.ResponseTimeMs
	stats.LastResponseMs = &respMs
	
	isUp := check.IsUp
	stats.LastIsUp = &isUp
	
	checkedAt := check.CheckedAt
	stats.LastCheckedAt = &checkedAt

	stats.RecentChecks = append(stats.RecentChecks, model.MinimalCheck{
		IsUp:           check.IsUp,
		CheckedAt:      check.CheckedAt,
		StatusCode:     check.StatusCode,
		ResponseTimeMs: check.ResponseTimeMs,
	})
	if len(stats.RecentChecks) > 50 {
		stats.RecentChecks = stats.RecentChecks[1:]
	}
}

func (sm *StateManager) BackgroundAggregator(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			sm.refreshAverages(ctx)
		}
	}
}

func (sm *StateManager) refreshAverages(ctx context.Context) {
	sm.mu.RLock()
	ids := make([]int64, 0, len(sm.state))
	for id := range sm.state {
		ids = append(ids, id)
	}
	sm.mu.RUnlock()

	for _, id := range ids {
		up24, err24 := sm.db.CalculateUptime(ctx, id, 24)
		up30, err30 := sm.db.CalculateUptime(ctx, id, 24*30)

		sm.mu.Lock()
		if stats, exists := sm.state[id]; exists {
			if err24 == nil && up24 != nil {
				stats.Uptime24h = up24
			}
			if err30 == nil && up30 != nil {
				stats.Uptime30d = up30
			}
		}
		sm.mu.Unlock()
	}
}

func (s *EndpointStats) Clone() *EndpointStats {
	if s == nil {
		return nil
	}

	copyStats := *s
	
	if s.LastStatusCode != nil {
		code := *s.LastStatusCode
		copyStats.LastStatusCode = &code
	}
	if s.LastResponseMs != nil {
		ms := *s.LastResponseMs
		copyStats.LastResponseMs = &ms
	}
	if s.LastIsUp != nil {
		up := *s.LastIsUp
		copyStats.LastIsUp = &up
	}
	if s.LastCheckedAt != nil {
		ca := *s.LastCheckedAt
		copyStats.LastCheckedAt = &ca
	}
	if s.Uptime24h != nil {
		u := *s.Uptime24h
		copyStats.Uptime24h = &u
	}
	if s.Uptime30d != nil {
		u := *s.Uptime30d
		copyStats.Uptime30d = &u
	}

	if s.RecentChecks != nil {
		copyStats.RecentChecks = make([]model.MinimalCheck, len(s.RecentChecks))
		copy(copyStats.RecentChecks, s.RecentChecks)
	}

	return &copyStats
}

func (sm *StateManager) GetStats(id int64) *EndpointStats {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if stats, exists := sm.state[id]; exists {
		return stats.Clone()
	}

	return nil
}

func (sm *StateManager) GetDashboardStats() map[int64]*EndpointStats {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	result := make(map[int64]*EndpointStats, len(sm.state))
	for id, stats := range sm.state {
		result[id] = stats.Clone()
	}
	return result
}

func (sm *StateManager) Delete(id int64) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	delete(sm.state, id)
}
