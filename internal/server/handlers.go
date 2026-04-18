package server

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"tinypulse/internal/model"
	"tinypulse/internal/notifier"
)

func respondJSON(w http.ResponseWriter, status int, payload any) {
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(payload)
}

func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{"error": message})
}

func parseID(r *http.Request) (int64, error) {
	return strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
}

func validateEndpoint(ep *model.Endpoint) error {
	if ep.Name == "" {
		return jsonError("name is required")
	}
	if ep.URL == "" {
		return jsonError("url is required")
	}
	u, err := url.Parse(ep.URL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return jsonError("url must be a valid http or https URL")
	}
	if ep.IntervalSeconds < 10 {
		return jsonError("interval_seconds must be at least 10")
	}
	return nil
}

type jsonError string

func (e jsonError) Error() string { return string(e) }

func (s *Server) listEndpoints(w http.ResponseWriter, r *http.Request) {
	endpoints, err := s.db.ListEndpointsWithStats(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	notifierMap, err := s.db.GetEndpointNotifierIDs(r.Context())
	if err != nil {
		slog.Warn("failed to fetch endpoint notifier mappings", "error", err)
	} else {
		for i := range endpoints {
			if nids, ok := notifierMap[endpoints[i].ID]; ok {
				endpoints[i].NotifierIDs = nids
			} else {
				endpoints[i].NotifierIDs = make([]int64, 0)
			}
		}
	}

	respondJSON(w, http.StatusOK, endpoints)
}

func (s *Server) createEndpoint(w http.ResponseWriter, r *http.Request) {
	var ep model.Endpoint
	if err := json.NewDecoder(r.Body).Decode(&ep); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := validateEndpoint(&ep); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := s.db.CreateEndpoint(r.Context(), &ep); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if len(ep.NotifierIDs) > 0 {
		if err := s.db.SetEndpointNotifiers(r.Context(), ep.ID, ep.NotifierIDs); err != nil {
			slog.Error("failed to set notifiers for new endpoint", "error", err)
		}
	}

	if err := s.manager.Add(r.Context(), ep.ID); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusCreated, ep)
}

func (s *Server) getEndpoint(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid id")
		return
	}
	ep, err := s.db.GetEndpoint(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusNotFound, "endpoint not found")
		return
	}
	respondJSON(w, http.StatusOK, ep)
}

func (s *Server) updateEndpoint(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var ep model.Endpoint
	if err := json.NewDecoder(r.Body).Decode(&ep); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := validateEndpoint(&ep); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	ep.ID = id

	if err := s.db.UpdateEndpoint(r.Context(), &ep); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err := s.db.SetEndpointNotifiers(r.Context(), ep.ID, ep.NotifierIDs); err != nil {
		slog.Error("failed to update notifiers for endpoint", "endpoint_id", ep.ID, "error", err)
	}

	if err := s.manager.Edit(r.Context(), ep.ID); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, ep)
}

func (s *Server) deleteEndpoint(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid id")
		return
	}
	s.manager.Delete(id)
	if err := s.db.DeleteEndpoint(r.Context(), id); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusNoContent, nil)
}

func (s *Server) pauseEndpoint(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid id")
		return
	}
	paused, err := s.db.TogglePause(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if paused {
		s.manager.Pause(id)
	} else {
		if err := s.manager.Add(r.Context(), id); err != nil {
			respondError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	respondJSON(w, http.StatusOK, map[string]bool{"paused": paused})
}

func (s *Server) listChecks(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid id")
		return
	}
	limit := 100
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	checks, err := s.db.GetChecksByEndpoint(r.Context(), id, limit)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, checks)
}

func (s *Server) history(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid id")
		return
	}
	limit := 60
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	checks, err := s.db.GetChecksByEndpoint(r.Context(), id, limit)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	// Return lightweight objects
	type item struct {
		IsUp      bool      `json:"is_up"`
		CheckedAt time.Time `json:"checked_at"`
	}
	out := make([]item, len(checks))
	for i, c := range checks {
		out[i] = item{IsUp: c.IsUp, CheckedAt: c.CheckedAt}
	}
	respondJSON(w, http.StatusOK, out)
}

// --- Notifier Handlers ---

func (s *Server) listNotifiers(w http.ResponseWriter, r *http.Request) {
	notifiers, err := s.db.ListNotifiers(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if notifiers == nil {
		notifiers = make([]model.Notifier, 0)
	}
	respondJSON(w, http.StatusOK, notifiers)
}

func (s *Server) createNotifier(w http.ResponseWriter, r *http.Request) {
	var n model.Notifier
	if err := json.NewDecoder(r.Body).Decode(&n); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	if n.Name == "" || n.Type == "" || n.ConfigJSON == "" {
		respondError(w, http.StatusBadRequest, "name, type, and config_json are required")
		return
	}

	// Validate config by attempting to build the provider
	if _, err := notifier.Build(&n); err != nil {
		respondError(w, http.StatusBadRequest, fmt.Sprintf("invalid config: %v", err))
		return
	}

	if err := s.db.CreateNotifier(r.Context(), &n); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusCreated, n)
}

func (s *Server) getNotifier(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid id")
		return
	}
	n, err := s.db.GetNotifier(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusNotFound, "notifier not found")
		return
	}
	respondJSON(w, http.StatusOK, n)
}

func (s *Server) updateNotifier(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid id")
		return
	}

	var n model.Notifier
	if err := json.NewDecoder(r.Body).Decode(&n); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	n.ID = id

	if n.Name == "" || n.Type == "" || n.ConfigJSON == "" {
		respondError(w, http.StatusBadRequest, "name, type, and config_json are required")
		return
	}

	if _, err := notifier.Build(&n); err != nil {
		respondError(w, http.StatusBadRequest, fmt.Sprintf("invalid config: %v", err))
		return
	}

	if err := s.db.UpdateNotifier(r.Context(), &n); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, n)
}

func (s *Server) deleteNotifier(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := s.db.DeleteNotifier(r.Context(), id); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusNoContent, nil)
}

func (s *Server) testNotifier(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid id")
		return
	}

	n, err := s.db.GetNotifier(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusNotFound, "notifier not found")
		return
	}

	provider, err := notifier.Build(n)
	if err != nil {
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("failed to build notifier: %v", err))
		return
	}

	if err := provider.Send(r.Context(), "🧪 TinyPulse Test", "This is a test notification from TinyPulse!"); err != nil {
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("test failed: %v", err))
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "test sent successfully"})
}
