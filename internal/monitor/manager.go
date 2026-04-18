package monitor

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"tinypulse/internal/db"
	"tinypulse/internal/model"
	"tinypulse/internal/notifier"
)

type endpointState struct {
	isUp             bool
	consecutiveFails int
}

// Manager orchestrates one goroutine per monitored endpoint.
type Manager struct {
	db         *db.DB
	client     *http.Client
	dispatcher *notifier.Dispatcher
	mu         sync.RWMutex
	workers    map[int64]context.CancelFunc
	state      map[int64]endpointState
	stop       context.CancelFunc
	baseCtx    context.Context
	results    chan *model.Check
}

func New(database *db.DB, dispatcher *notifier.Dispatcher) *Manager {
	return &Manager{
		db:         database,
		dispatcher: dispatcher,
		client: &http.Client{
			Timeout: 10 * time.Second,
			Transport: &http.Transport{
				DisableKeepAlives:     true,
				TLSHandshakeTimeout:   5 * time.Second,
				ResponseHeaderTimeout: 5 * time.Second,
			},
		},
		workers: make(map[int64]context.CancelFunc),
		state:   make(map[int64]endpointState),
		results: make(chan *model.Check, 1024),
	}
}

// Start loads all active endpoints and spawns workers.
func (m *Manager) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	m.stop = cancel
	m.baseCtx = ctx

	endpoints, err := m.db.ListActiveEndpoints(ctx)
	if err != nil {
		return fmt.Errorf("list active endpoints: %w", err)
	}
	for _, ep := range endpoints {
		m.spawn(ctx, ep)
	}

	go m.dbWriterLoop(ctx)
	go m.cleanupLoop(ctx)
	return nil
}

// Stop cancels all workers and the cleanup loop.
func (m *Manager) Stop() {
	if m.stop != nil {
		m.stop()
	}
}

// Add starts monitoring for a new or resumed endpoint.
func (m *Manager) Add(_ context.Context, id int64) error {
	ep, err := m.db.GetEndpoint(m.baseCtx, id)
	if err != nil {
		return err
	}
	if ep.Paused {
		return nil
	}
	m.spawn(m.baseCtx, *ep)
	return nil
}

// Edit restarts a worker with updated configuration.
func (m *Manager) Edit(_ context.Context, id int64) error {
	m.kill(id)
	return m.Add(m.baseCtx, id)
}

// Delete stops monitoring and removes the endpoint.
func (m *Manager) Delete(id int64) {
	m.kill(id)
	m.mu.Lock()
	delete(m.state, id)
	m.mu.Unlock()
}

// Pause stops monitoring for an endpoint.
func (m *Manager) Pause(id int64) {
	m.kill(id)
}

func (m *Manager) spawn(parent context.Context, ep model.Endpoint) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.workers[ep.ID]; ok {
		return // already running
	}

	ctx, cancel := context.WithCancel(parent)
	m.workers[ep.ID] = cancel

	go m.runWorker(ctx, ep)
}

func (m *Manager) kill(id int64) {
	m.mu.Lock()
	cancel, ok := m.workers[id]
	delete(m.workers, id)
	m.mu.Unlock()

	if ok {
		cancel()
	}
}

func (m *Manager) runWorker(ctx context.Context, ep model.Endpoint) {
	// Stagger startup (0-2s) to prevent a thundering herd on app restart
	jitter := time.Duration(rand.Intn(2000)) * time.Millisecond
	select {
	case <-time.After(jitter):
	case <-ctx.Done():
		return
	}

	ticker := time.NewTicker(time.Duration(ep.IntervalSeconds) * time.Second)
	defer ticker.Stop()

	// immediate first check (safely staggered)
	m.check(ctx, ep)

	for {
		select {
		case <-ticker.C:
			m.check(ctx, ep)
		case <-ctx.Done():
			return
		}
	}
}

func (m *Manager) check(ctx context.Context, ep model.Endpoint) {
	statusCode, responseTimeMs, isUp := m.performCheck(ctx, ep)

	check := &model.Check{
		EndpointID:     ep.ID,
		StatusCode:     statusCode,
		ResponseTimeMs: responseTimeMs,
		IsUp:           isUp,
	}

	select {
	case m.results <- check:
	default:
		slog.Warn("db writer channel full, dropping check metric", "endpoint_id", ep.ID)
	}

	m.mu.Lock()
	currentState, hadLast := m.state[ep.ID]

	newState := endpointState{
		isUp:             isUp,
		consecutiveFails: currentState.consecutiveFails,
	}

	if !isUp {
		newState.consecutiveFails++
	} else {
		newState.consecutiveFails = 0
	}

	m.state[ep.ID] = newState
	m.mu.Unlock()

	if !hadLast {
		// First check ever since app start, don't alert on state change
		// except if it's immediately down (could have been down before restart).
		// For now, let's treat the first check as the baseline.
		return
	}

	if isUp && !currentState.isUp {
		slog.Info("endpoint recovered", "endpoint_id", ep.ID, "name", ep.Name, "url", ep.URL)
		m.dispatcher.SendAlert(ep.ID, "✅ Service Recovered", fmt.Sprintf("%s is back online.", ep.Name))
	} else if !isUp && newState.consecutiveFails == ep.FailThreshold {
		slog.Info("endpoint down threshold reached", "endpoint_id", ep.ID, "name", ep.Name, "url", ep.URL, "status_code", statusCode)
		m.dispatcher.SendAlert(ep.ID, "🚨 Service Down", fmt.Sprintf("%s is offline.\nURL: %s", ep.Name, ep.URL))
	}
}

func (m *Manager) performCheck(ctx context.Context, ep model.Endpoint) (*int, int, bool) {
	start := time.Now()
	statusCode, err := m.doRequest(ctx, ep.URL, http.MethodHead)
	if err != nil {
		// If HEAD fails, try GET as a fallback in case the server doesn't support HEAD.
		if statusCode != nil && *statusCode == http.StatusMethodNotAllowed {
			start = time.Now()
			statusCode, err = m.doRequest(ctx, ep.URL, http.MethodGet)
		}
	}
	responseTimeMs := int(time.Since(start).Milliseconds())

	if err != nil {
		return nil, responseTimeMs, false
	}
	isUp := *statusCode >= 200 && *statusCode < 400
	return statusCode, responseTimeMs, isUp
}

func (m *Manager) doRequest(ctx context.Context, url, method string) (*int, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return nil, err
	}

	// Add a User-Agent so we don't get blocked by WAFs like Cloudflare
	req.Header.Set("User-Agent", "tinypulse/1.0 (Health Monitor)")

	resp, err := m.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Safely read up to 1MB to clear the network buffer before closing
	io.Copy(io.Discard, io.LimitReader(resp.Body, 1024*1024))

	sc := resp.StatusCode
	return &sc, nil
}

func (m *Manager) cleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := m.db.CleanupOldChecks(ctx, 90); err != nil {
				slog.Error("cleanup old checks failed", "error", err)
			}
		case <-ctx.Done():
			return
		}
	}
}

func (m *Manager) dbWriterLoop(ctx context.Context) {
	for {
		select {
		case check := <-m.results:
			if err := m.db.RecordCheck(ctx, check); err != nil {
				slog.Error("failed to record check", "endpoint_id", check.EndpointID, "error", err)
			}
		case <-ctx.Done():
			return
		}
	}
}
