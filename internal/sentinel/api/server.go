// Package api serves the erebrus-sentinel local HTTP API on :8788.
package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/NetSepio/erebrus/internal/sentinel/policy"
)

// Server is the Sentinel control API (private to the Docker network).
type Server struct {
	addr      string
	confDir   string
	licensed  bool
	mu        sync.RWMutex
	lastApply time.Time
	rules     int
}

// New constructs a Server.
func New(addr, confDir string) *Server {
	if addr == "" {
		addr = ":8788"
	}
	if confDir == "" {
		confDir = "/etc/unbound/conf.d/generated"
	}
	licensed := os.Getenv("SENTINEL_LICENSED") != "false"
	return &Server{addr: addr, confDir: confDir, licensed: licensed}
}

// ListenAndServe blocks until the server stops.
func (s *Server) ListenAndServe() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/status", s.handleStatus)
	mux.HandleFunc("/license/check", s.handleLicenseCheck)
	mux.HandleFunc("/policy/apply", s.handlePolicyApply)
	mux.HandleFunc("/rules/sync", s.handleRulesSync)
	mux.HandleFunc("/reload", s.handleReload)
	mux.HandleFunc("/metrics", s.handleMetrics)
	srv := &http.Server{
		Addr: s.addr, Handler: mux, ReadHeaderTimeout: 10 * time.Second,
	}
	slog.Info("sentinel API listening", "addr", s.addr)
	return srv.ListenAndServe()
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]string{"status": "ok"})
}

func (s *Server) handleStatus(w http.ResponseWriter, _ *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	writeJSON(w, map[string]any{
		"licensed": s.licensed, "rules": s.rules, "last_apply": s.lastApply,
		"conf_dir": s.confDir,
	})
}

func (s *Server) handleLicenseCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Licensed *bool `json:"licensed"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	s.mu.Lock()
	if req.Licensed != nil {
		s.licensed = *req.Licensed
	}
	lic := s.licensed
	s.mu.Unlock()
	writeJSON(w, map[string]any{"licensed": lic})
}

func (s *Server) handlePolicyApply(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var p policy.Policy
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if err := s.apply(p); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]string{"status": "applied"})
}

func (s *Server) handleRulesSync(w http.ResponseWriter, r *http.Request) {
	s.handlePolicyApply(w, r)
}

func (s *Server) handleReload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := reloadUnbound(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]string{"status": "reloaded"})
}

func reloadUnbound() error {
	if out, err := exec.Command("unbound-control", "reload").CombinedOutput(); err == nil {
		return nil
	} else if len(out) > 0 {
		slog.Debug("unbound-control reload", "out", string(out), "err", err)
	}
	if out, err := exec.Command("unbound-control", "status").CombinedOutput(); err != nil {
		return err
	} else {
		_ = out
	}
	return exec.Command("killall", "-HUP", "unbound").Run()
}

func (s *Server) handleMetrics(w http.ResponseWriter, _ *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	writeJSON(w, map[string]any{"rules_active": s.rules, "licensed": s.licensed})
}

func (s *Server) apply(p policy.Policy) error {
	s.mu.RLock()
	lic := s.licensed
	s.mu.RUnlock()
	if !lic {
		return fmt.Errorf("sentinel unlicensed")
	}
	w := &policy.Writer{Dir: s.confDir}
	if err := w.Apply(p); err != nil {
		return err
	}
	s.mu.Lock()
	s.rules = len(p.Rules)
	s.lastApply = time.Now()
	s.mu.Unlock()
	return nil
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

// ConfigDir returns the generated config directory from env.
func ConfigDir() string {
	if d := os.Getenv("SENTINEL_CONF_DIR"); d != "" {
		return d
	}
	return "/etc/unbound/conf.d/generated"
}
