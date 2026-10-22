package sidecar

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Station struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type StationInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	StreamURL   string `json:"stream_url"`
	ContentType string `json:"content_type"`
	Title       string `json:"title,omitempty"`
}

type NowPlaying struct {
	Title string `json:"title,omitempty"`
}

type API struct {
	provider *MediaProvider
	baseURL  string
}

func NewAPI(provider *MediaProvider, baseURL string) *API {
	return &API{provider: provider, baseURL: strings.TrimRight(baseURL, "/")}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func (a *API) Handler() http.Handler {
	return http.HandlerFunc(a.serve)
}

func (a *API) serve(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	if strings.HasPrefix(path, "/app") {
		if !isLoopbackRequest(r) {
			http.NotFound(w, r)
			return
		}
		a.desktop(w, r)
		return
	}
	switch {
	case path == "/" && r.Method == http.MethodGet:
		if !isLoopbackRequest(r) {
			http.NotFound(w, r)
			return
		}
		http.Redirect(w, r, "/app", http.StatusTemporaryRedirect)
	case path == "/api/health":
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"))
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	case path == "/api/stations":
		a.stations(w, r)
	case strings.HasPrefix(path, "/api/stations/"):
		a.station(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (a *API) stations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"))
		return
	}
	writeJSON(w, http.StatusOK, a.provider.Stations())
}

func (a *API) station(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"))
		return
	}
	ref, action, routeFormat, err := stationRouteRef(r.URL.EscapedPath(), r.URL.RawQuery)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	station, _, err := a.provider.Resolve(ref)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()
	switch action {
	case "":
		info, err := a.provider.StationInfo(ctx, station, a.baseURL)
		if err != nil {
			writeProviderError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, info)
	case "now-playing":
		video, err := a.provider.Video(ctx, station.URL)
		if err != nil {
			writeProviderError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, NowPlaying{Title: video.Title})
	case "stream":
		format := strings.ToLower(strings.TrimSpace(routeFormat))
		if format == "" {
			format = strings.ToLower(strings.TrimSpace(r.URL.Query().Get("format")))
		}
		if format != "" && format != "audio" && format != "video" {
			writeError(w, http.StatusBadRequest, fmt.Errorf("format must be audio or video"))
			return
		}
		if err := a.provider.Stream(w, r, station, format == "video"); err != nil {
			if errors.Is(err, http.ErrAbortHandler) || r.Context().Err() != nil {
				return
			}
			if w.Header().Get("Content-Type") == "" {
				writeProviderError(w, err)
			}
		}
	default:
		writeError(w, http.StatusNotFound, fmt.Errorf("not found"))
	}
}

// stationRouteRef reads both station IDs and encoded full URLs from the route.
// Encoding the URL as one path segment preserves its query string and slashes.
func stationRouteRef(escapedPath, rawQuery string) (ref, action, format string, err error) {
	const prefix = "/api/stations/"
	if !strings.HasPrefix(escapedPath, prefix) {
		return "", "", "", fmt.Errorf("station not found")
	}
	path := strings.TrimPrefix(escapedPath, prefix)
	for _, suffix := range []string{"/now-playing", "/stream"} {
		if strings.HasSuffix(path, suffix) {
			path = strings.TrimSuffix(path, suffix)
			action = strings.TrimPrefix(suffix, "/")
			break
		}
	}
	// Also accept the readable form /api/stations/https://youtube.com/watch?v=ID/stream.
	// The HTTP URL parser puts everything after the video's ? into RawQuery, so
	// recover the endpoint suffix there when it was not percent-encoded.
	if action == "" && rawQuery != "" {
		videoQuery := rawQuery
		if marker := strings.LastIndex(videoQuery, "?format="); marker >= 0 {
			format = strings.TrimSpace(videoQuery[marker+len("?format="):])
			videoQuery = videoQuery[:marker]
		}
		for _, suffix := range []string{"/now-playing", "/stream"} {
			if strings.HasSuffix(videoQuery, suffix) {
				videoQuery = strings.TrimSuffix(videoQuery, suffix)
				action = strings.TrimPrefix(suffix, "/")
				break
			}
		}
		if action != "" {
			path = path + "?" + videoQuery
		} else if strings.HasPrefix(path, "https%3A") || strings.HasPrefix(strings.ToLower(path), "https:") || strings.HasPrefix(strings.ToLower(path), "http:") {
			path = path + "?" + rawQuery
		}
	}
	if path == "" {
		return "", "", "", fmt.Errorf("station not found")
	}
	ref, err = url.PathUnescape(path)
	if err != nil {
		return "", "", "", fmt.Errorf("station URL is incorrectly escaped")
	}
	return ref, action, format, nil
}

func writeProviderError(w http.ResponseWriter, err error) {
	status := http.StatusBadGateway
	if strings.Contains(err.Error(), "not found") {
		status = http.StatusNotFound
	} else if strings.Contains(err.Error(), "yt-dlp was not found") || strings.Contains(err.Error(), "ffmpeg") {
		status = http.StatusServiceUnavailable
	}
	writeError(w, status, err)
}

func isLoopbackRequest(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(strings.Trim(host, "[]"))
	return ip != nil && ip.IsLoopback()
}

type desktopState struct {
	StreamMode string         `json:"stream_mode"`
	Stations   []SavedStation `json:"stations"`
	BaseURL    string         `json:"base_url"`
}

func (a *API) desktop(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.URL.Path == "/app" && r.Method == http.MethodGet:
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(desktopHTML))
	case r.URL.Path == "/app/api/state" && r.Method == http.MethodGet:
		cfg := a.provider.store.Snapshot()
		writeJSON(w, http.StatusOK, desktopState{
			StreamMode: cfg.StreamMode, Stations: cfg.Stations, BaseURL: a.baseURL,
		})
	case r.URL.Path == "/app/api/mode" && r.Method == http.MethodPost:
		var req struct {
			Mode string `json:"mode"`
		}
		if !decodeJSON(w, r, &req) {
			return
		}
		if err := a.provider.SetStreamMode(req.Mode); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "saved"})
	case r.URL.Path == "/app/api/stations" && r.Method == http.MethodPost:
		var req struct {
			URL  string `json:"url"`
			Name string `json:"name"`
		}
		if !decodeJSON(w, r, &req) {
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
		defer cancel()
		station, err := a.provider.AddStation(ctx, req.URL, req.Name)
		if err != nil {
			writeProviderError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, station)
	case strings.HasPrefix(r.URL.Path, "/app/api/stations/") && r.Method == http.MethodDelete:
		id, err := url.PathUnescape(strings.TrimPrefix(r.URL.EscapedPath(), "/app/api/stations/"))
		if err != nil || strings.Contains(id, "/") {
			writeError(w, http.StatusBadRequest, fmt.Errorf("invalid station ID"))
			return
		}
		if err := a.provider.RemoveStation(id); err != nil {
			writeError(w, http.StatusNotFound, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "removed"})
	default:
		if strings.HasPrefix(r.URL.Path, "/app/api/") {
			writeError(w, http.StatusNotFound, fmt.Errorf("not found"))
			return
		}
		http.NotFound(w, r)
	}
}

func decodeJSON(w http.ResponseWriter, r *http.Request, value any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 32*1024)
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(value); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid request: %w", err))
		return false
	}
	return true
}

func ValidateBaseURL(baseURL string) error {
	parsed, err := url.ParseRequestURI(baseURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return fmt.Errorf("base URL must be a complete HTTP or HTTPS address")
	}
	if _, err := strconv.Atoi(parsed.Port()); parsed.Port() != "" && err != nil {
		return fmt.Errorf("base URL contains an invalid port")
	}
	return nil
}
