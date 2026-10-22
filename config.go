package sidecar

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type SavedStation struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	URL   string `json:"url"`
	Title string `json:"title,omitempty"`
}

type Config struct {
	StreamMode string         `json:"stream_mode,omitempty"`
	Stations   []SavedStation `json:"stations,omitempty"`
}

type ConfigStore struct {
	mu   sync.RWMutex
	path string
	cfg  Config
}

func DefaultConfigPath() (string, error) {
	if override := strings.TrimSpace(os.Getenv("YOUTUBE_SIDECAR_CONFIG")); override != "" {
		abs, err := filepath.Abs(override)
		return abs, err
	}
	root, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("find the user config folder: %w", err)
	}
	return filepath.Join(root, "YouTube-Sidecar", "config.json"), nil
}

func OpenConfig(path string) (*ConfigStore, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve config path: %w", err)
	}
	store := &ConfigStore{path: abs}
	body, err := os.ReadFile(abs)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("read config %q: %w", abs, err)
	}
	if err == nil {
		if err := json.Unmarshal(body, &store.cfg); err != nil {
			return nil, fmt.Errorf("parse config %q: %w", abs, err)
		}
	}
	if store.cfg.StreamMode == "" {
		store.cfg.StreamMode = "direct"
	}
	if store.cfg.Stations == nil {
		store.cfg.Stations = []SavedStation{}
	}
	for i, station := range store.cfg.Stations {
		name := stationDisplayName(station.Name, station.Title)
		stationName := uniqueStationName(name, store.cfg.Stations, i)
		store.cfg.Stations[i].Name = stationName
		store.cfg.Stations[i].ID = stationName
	}
	if store.cfg.StreamMode != "direct" && store.cfg.StreamMode != "download" {
		return nil, fmt.Errorf("config stream_mode must be direct or download")
	}
	return store, nil
}

func (s *ConfigStore) Snapshot() Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	copy := s.cfg
	copy.Stations = append([]SavedStation(nil), s.cfg.Stations...)
	return copy
}

func (s *ConfigStore) Update(change func(*Config) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.cfg
	next.Stations = append([]SavedStation(nil), s.cfg.Stations...)
	if err := change(&next); err != nil {
		return err
	}
	if next.StreamMode != "direct" && next.StreamMode != "download" {
		return fmt.Errorf("stream mode must be direct or download")
	}
	if next.Stations == nil {
		next.Stations = []SavedStation{}
	}
	if err := s.write(next); err != nil {
		return err
	}
	s.cfg = next
	return nil
}

func (s *ConfigStore) write(cfg Config) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return fmt.Errorf("create config folder: %w", err)
	}
	body, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, append(body, '\n'), 0o600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("save config: %w", err)
	}
	return nil
}
