package sidecar

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Video struct {
	ID        string  `json:"id"`
	Title     string  `json:"title,omitempty"`
	Duration  float64 `json:"duration,omitempty"`
	Thumbnail string  `json:"thumbnail,omitempty"`
	URL       string  `json:"webpage_url,omitempty"`
	Live      bool    `json:"is_live,omitempty"`
}

type MediaProvider struct {
	store  *ConfigStore
	ytDLP  string
	ffmpeg string
}

func NewMediaProvider(store *ConfigStore) *MediaProvider {
	return &MediaProvider{
		store:  store,
		ytDLP:  envOr("YT_DLP", "yt-dlp"),
		ffmpeg: envOr("FFMPEG", "ffmpeg"),
	}
}

func envOr(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func (p *MediaProvider) SetStreamMode(mode string) error {
	if mode != "direct" && mode != "download" {
		return fmt.Errorf("stream mode must be direct or download")
	}
	return p.store.Update(func(cfg *Config) error {
		cfg.StreamMode = mode
		return nil
	})
}

func (p *MediaProvider) Stations() []Station {
	configured := p.store.Snapshot().Stations
	stations := make([]Station, 0, len(configured))
	for _, station := range configured {
		stations = append(stations, Station{ID: station.ID, Name: station.Name})
	}
	return stations
}

func (p *MediaProvider) Resolve(ref string) (SavedStation, bool, error) {
	for _, station := range p.store.Snapshot().Stations {
		if station.ID == ref || legacyStationID(station.URL) == ref {
			return station, true, nil
		}
	}
	cleanURL, err := ValidateYouTubeURL(ref)
	if err != nil {
		return SavedStation{}, false, err
	}
	return SavedStation{ID: cleanURL, Name: "YouTube video", URL: cleanURL}, false, nil
}

func (p *MediaProvider) AddStation(ctx context.Context, rawURL, customName string) (SavedStation, error) {
	cleanURL, err := ValidateYouTubeURL(rawURL)
	if err != nil {
		return SavedStation{}, err
	}
	video, err := p.Video(ctx, cleanURL)
	if err != nil {
		return SavedStation{}, err
	}
	name := stationDisplayName(customName, video.Title)
	station := SavedStation{Name: name, Title: video.Title, URL: cleanURL}
	err = p.store.Update(func(cfg *Config) error {
		existingIndex := -1
		for i, existing := range cfg.Stations {
			if existing.URL == cleanURL {
				existingIndex = i
				break
			}
		}
		station.ID = uniqueStationName(name, cfg.Stations, existingIndex)
		station.Name = station.ID
		if existingIndex >= 0 {
			cfg.Stations[existingIndex] = station
		} else {
			cfg.Stations = append(cfg.Stations, station)
		}
		return nil
	})
	return station, err
}

func (p *MediaProvider) RemoveStation(id string) error {
	return p.store.Update(func(cfg *Config) error {
		for i, station := range cfg.Stations {
			if station.ID == id {
				cfg.Stations = append(cfg.Stations[:i], cfg.Stations[i+1:]...)
				return nil
			}
		}
		return fmt.Errorf("station not found")
	})
}

func legacyStationID(videoURL string) string {
	hash := sha256.Sum256([]byte(videoURL))
	return "youtube-" + hex.EncodeToString(hash[:6])
}

func stationDisplayName(name, title string) string {
	if name = strings.TrimSpace(name); name != "" {
		return name
	}
	if title = strings.TrimSpace(title); title != "" {
		return title
	}
	return "YouTube video"
}

func uniqueStationName(name string, stations []SavedStation, skip int) string {
	for suffix := 1; ; suffix++ {
		candidate := name
		if suffix > 1 {
			candidate = fmt.Sprintf("%s (%d)", name, suffix)
		}
		used := false
		for i, station := range stations {
			if i != skip && station.ID == candidate {
				used = true
				break
			}
		}
		if !used {
			return candidate
		}
	}
}

func ValidateYouTubeURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	parsed, err := url.Parse(raw)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.User != nil || parsed.Hostname() == "" {
		return "", fmt.Errorf("enter a complete YouTube or YouTube Music URL")
	}
	host := strings.ToLower(strings.TrimSuffix(parsed.Hostname(), "."))
	allowed := host == "youtu.be" || host == "youtube.com" || host == "music.youtube.com" || host == "www.youtube.com" || host == "m.youtube.com" || host == "youtube-nocookie.com" || strings.HasSuffix(host, ".youtube.com") || strings.HasSuffix(host, ".youtube-nocookie.com")
	if !allowed || (parsed.Port() != "" && parsed.Port() != "443") {
		return "", fmt.Errorf("only YouTube and YouTube Music links are supported")
	}
	parsed.Scheme = "https"
	if parsed.Port() == "443" {
		parsed.Host = host + ":443"
	} else {
		parsed.Host = host
	}
	parsed.Fragment = ""
	return parsed.String(), nil
}

func (p *MediaProvider) Video(ctx context.Context, videoURL string) (Video, error) {
	args := []string{"--quiet", "--no-warnings", "--no-playlist", "--skip-download", "--dump-single-json", videoURL}
	output, err := p.commandOutput(ctx, p.ytDLP, args...)
	if err != nil {
		return Video{}, fmt.Errorf("yt-dlp could not read this video: %s", summarizeCommandError(err, output))
	}
	var video Video
	if err := json.Unmarshal(output, &video); err != nil {
		return Video{}, fmt.Errorf("yt-dlp returned video details that could not be read")
	}
	if video.URL == "" {
		video.URL = videoURL
	}
	return video, nil
}

func (p *MediaProvider) commandOutput(ctx context.Context, executable string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, executable, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		return stdout.Bytes(), fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}

func summarizeCommandError(err error, output []byte) string {
	message := strings.TrimSpace(err.Error())
	if strings.Contains(message, "executable file not found") || strings.Contains(message, "cannot find the file") {
		return "yt-dlp was not found; install it or set YT_DLP to its executable path"
	}
	if len(output) != 0 {
		message = strings.TrimSpace(string(output)) + " " + message
	}
	if len(message) > 360 {
		message = message[len(message)-360:]
	}
	return message
}

func (p *MediaProvider) StationInfo(ctx context.Context, station SavedStation, baseURL string) (StationInfo, error) {
	video, err := p.Video(ctx, station.URL)
	if err != nil {
		return StationInfo{}, err
	}
	name := station.Name
	if name == "" || name == "YouTube video" {
		name = video.Title
	}
	return StationInfo{
		ID: station.ID, Name: name,
		StreamURL:   baseURL + "/api/stations/" + url.PathEscape(station.ID) + "/stream",
		ContentType: "audio/mpeg", Title: video.Title,
	}, nil
}

func (p *MediaProvider) Stream(w http.ResponseWriter, r *http.Request, station SavedStation, videoMode bool) error {
	mode := p.store.Snapshot().StreamMode
	if mode == "download" {
		return p.streamDownloaded(w, r, station.URL, videoMode)
	}
	return p.streamDirect(w, r, station.URL, videoMode)
}

func (p *MediaProvider) streamDirect(w http.ResponseWriter, r *http.Request, videoURL string, videoMode bool) error {
	format := "bestaudio/best"
	if videoMode {
		format = "best/bestvideo+bestaudio"
	}
	ytArgs := []string{"--quiet", "--no-warnings", "--no-progress", "--no-playlist", "--format", format, "--output", "-", videoURL}
	yt := exec.CommandContext(r.Context(), p.ytDLP, ytArgs...)
	var ytStderr bytes.Buffer
	yt.Stderr = &ytStderr
	ytOut, err := yt.StdoutPipe()
	if err != nil {
		return fmt.Errorf("start yt-dlp output: %w", err)
	}
	if err := yt.Start(); err != nil {
		return fmt.Errorf("start yt-dlp: %w", err)
	}
	return p.transcode(w, r, ytOut, videoMode, func() error {
		if err := yt.Wait(); err != nil {
			return fmt.Errorf("yt-dlp stream failed: %s", summarizeCommandError(err, ytStderr.Bytes()))
		}
		return nil
	})
}

func (p *MediaProvider) streamDownloaded(w http.ResponseWriter, r *http.Request, videoURL string, videoMode bool) error {
	workDir, err := os.MkdirTemp("", "youtube-sidecar-*")
	if err != nil {
		return fmt.Errorf("create temporary download folder: %w", err)
	}
	defer os.RemoveAll(workDir)
	format := "bestaudio/best"
	if videoMode {
		format = "bestvideo+bestaudio/best"
	}
	output := filepath.Join(workDir, "source.%(ext)s")
	args := []string{"--quiet", "--no-warnings", "--no-progress", "--no-playlist", "--format", format, "--no-part", "--output", output, videoURL}
	cmd := exec.CommandContext(r.Context(), p.ytDLP, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("temporary download failed: %s", summarizeCommandError(err, stderr.Bytes()))
	}
	files, err := filepath.Glob(filepath.Join(workDir, "source.*"))
	if err != nil || len(files) == 0 {
		return fmt.Errorf("yt-dlp completed without creating a media file")
	}
	input, err := os.Open(files[0])
	if err != nil {
		return fmt.Errorf("open temporary media: %w", err)
	}
	defer input.Close()
	return p.transcode(w, r, input, videoMode, nil)
}

func (p *MediaProvider) transcode(w http.ResponseWriter, r *http.Request, input io.Reader, videoMode bool, waitInput func() error) error {
	// yt-dlp and temporary downloads can provide data faster than real time.
	// Pace FFmpeg's reads so speakers receive a live stream instead of a burst
	// that drains before their playback buffer catches up.
	args := []string{"-hide_banner", "-loglevel", "error", "-nostdin", "-re", "-i", "pipe:0"}
	contentType := "audio/mpeg"
	if videoMode {
		args = append(args, "-c:v", "libx264", "-preset", "ultrafast", "-tune", "zerolatency", "-c:a", "aac", "-b:a", "192k", "-f", "mp4", "-movflags", "frag_keyframe+empty_moov")
		contentType = "video/mp4"
	} else {
		args = append(args, "-vn", "-ac", "2", "-ar", "44100", "-c:a", "libmp3lame", "-b:a", "256k", "-f", "mp3")
	}
	args = append(args, "pipe:1")
	ffmpeg := exec.CommandContext(r.Context(), p.ffmpeg, args...)
	ffmpeg.Stdin = input
	var ffmpegStderr bytes.Buffer
	ffmpeg.Stderr = &ffmpegStderr
	stdout, err := ffmpeg.StdoutPipe()
	if err != nil {
		return fmt.Errorf("start ffmpeg output: %w", err)
	}
	if err := ffmpeg.Start(); err != nil {
		if closer, ok := input.(io.Closer); ok {
			_ = closer.Close()
		}
		if waitInput != nil {
			_ = waitInput()
		}
		return fmt.Errorf("start ffmpeg: %w", err)
	}
	var firstByte [1]byte
	firstCount, firstErr := io.ReadFull(stdout, firstByte[:])
	if firstCount == 0 {
		ffmpegErr := ffmpeg.Wait()
		if closer, ok := input.(io.Closer); ok {
			_ = closer.Close()
		}
		if waitInput != nil {
			if sourceErr := waitInput(); sourceErr != nil {
				return sourceErr
			}
		}
		if ffmpegErr != nil {
			return fmt.Errorf("ffmpeg could not start playback: %s", strings.TrimSpace(ffmpegStderr.String()))
		}
		if firstErr != nil {
			return fmt.Errorf("media stream ended before playback started")
		}
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Connection", "close")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	flusher, _ := w.(http.Flusher)
	_, writeErr := w.Write(firstByte[:firstCount])
	copyErr := writeErr
	if copyErr == nil && flusher != nil {
		flusher.Flush()
	}
	if copyErr == nil {
		copyErr = copyAndFlush(w, stdout, flusher)
	}
	if copyErr != nil {
		_ = stdout.Close()
	}
	ffmpegErr := ffmpeg.Wait()
	if closer, ok := input.(io.Closer); ok {
		_ = closer.Close()
	}
	if waitInput != nil {
		ytErr := waitInput()
		if ytErr != nil && r.Context().Err() == nil && copyErr == nil && ffmpegErr == nil {
			return fmt.Errorf("yt-dlp stream ended unexpectedly: %w", ytErr)
		}
	}
	if r.Context().Err() != nil {
		return r.Context().Err()
	}
	if copyErr != nil {
		return copyErr
	}
	if ffmpegErr != nil {
		return fmt.Errorf("ffmpeg stream failed: %w", ffmpegErr)
	}
	return nil
}

func copyAndFlush(dst io.Writer, src io.Reader, flusher http.Flusher) error {
	buf := make([]byte, 64*1024)
	for {
		n, readErr := src.Read(buf)
		if n > 0 {
			written, writeErr := dst.Write(buf[:n])
			if writeErr != nil {
				return writeErr
			}
			if written != n {
				return io.ErrShortWrite
			}
			if flusher != nil {
				flusher.Flush()
			}
		}
		if readErr != nil {
			if readErr == io.EOF {
				return nil
			}
			return readErr
		}
	}
}
