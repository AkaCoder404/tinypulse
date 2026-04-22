package notifier

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"tinypulse/internal/db"
	"tinypulse/internal/model"
)

// Provider defines the interface that all notification services must implement.
type Provider interface {
	Type() string
	Send(ctx context.Context, endpointID int64, endpointName, title, message string) error
}

// Factory defines a function that creates a new Provider from a JSON config blob.
type Factory func(configJSON string) (Provider, error)

var (
	registry = make(map[string]Factory)
	mu       sync.RWMutex
)

// Register registers a new notification provider factory by its Type string (e.g., "TELEGRAM").
func Register(providerType string, factory Factory) {
	mu.Lock()
	defer mu.Unlock()
	registry[strings.ToLower(providerType)] = factory
}

// Build instantiates a Provider from a Notifier model using the registered factories.
func Build(n *model.Notifier) (Provider, error) {
	mu.RLock()
	factory, ok := registry[strings.ToLower(n.Type)]
	mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("unknown notifier type: %s", n.Type)
	}

	return factory(n.ConfigJSON)
}

// Dispatcher reads linked notifiers for an endpoint and sends alerts.
type Dispatcher struct {
	db *db.DB
	wg sync.WaitGroup
}

func NewDispatcher(database *db.DB) *Dispatcher {
	return &Dispatcher{db: database}
}

// Stop blocks until all in-flight alerts have finished sending.
func (d *Dispatcher) Stop() {
	d.wg.Wait()
}

// SendAlert fires an alert to all notifiers linked to an endpoint, tracked by the Dispatcher.
func (d *Dispatcher) SendAlert(endpointID int64, endpointName, title, message string) {
	d.wg.Add(1)

	go func() {
		defer d.wg.Done()

		// Use a bounded context for the entire dispatch process (e.g., 30s)
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		notifiers, err := d.db.GetNotifiersForEndpoint(ctx, endpointID)
		if err != nil {
			slog.Error("failed to get notifiers for alert", "endpoint_id", endpointID, "error", err)
			return
		}

		if len(notifiers) == 0 {
			// No notifiers linked to this endpoint
			return
		}

		var sendWg sync.WaitGroup
		for _, n := range notifiers {
			n := n // capture loop variable for goroutine
			sendWg.Add(1)
			go func() {
				defer sendWg.Done()
				provider, err := Build(&n)
				if err != nil {
					slog.Error("failed to build notifier", "notifier_id", n.ID, "type", n.Type, "error", err)
					return
				}

				if err := provider.Send(ctx, endpointID, endpointName, title, message); err != nil {
					slog.Error("failed to send alert", "notifier_id", n.ID, "type", n.Type, "error", err)
				} else {
					slog.Info("alert sent successfully", "notifier_id", n.ID, "type", n.Type)
				}
			}()
		}
		sendWg.Wait()
	}()
}
