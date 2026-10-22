package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	sidecar "github.com/codester28351/youtube-sidecar"
)

func detectLANIP() string {
	interfaces, err := net.Interfaces()
	if err != nil {
		return "127.0.0.1"
	}
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if ok && ipNet.IP != nil && ipNet.IP.To4() != nil && !ipNet.IP.IsLoopback() {
				return ipNet.IP.String()
			}
		}
	}
	return "127.0.0.1"
}

func main() {
	configPath, err := sidecar.DefaultConfigPath()
	if err != nil {
		log.Fatal(err)
	}
	if len(os.Args) > 1 {
		if len(os.Args) == 2 && (os.Args[1] == "-h" || os.Args[1] == "--help") {
			fmt.Println("YouTube Sidecar runs an STR-compatible YouTube and YouTube Music stream API.")
			fmt.Println("Usage: youtube-sidecar [CONFIG.json]")
			fmt.Println("Set YOUTUBE_SIDECAR_CONFIG, SIDECAR_HOST, SIDECAR_PORT, SIDECAR_BASE_URL, YT_DLP, or FFMPEG to override defaults.")
			return
		}
		if len(os.Args) != 2 {
			log.Fatal("usage: youtube-sidecar [CONFIG.json]")
		}
		configPath = os.Args[1]
	}
	store, err := sidecar.OpenConfig(configPath)
	if err != nil {
		log.Fatal(err)
	}
	provider := sidecar.NewMediaProvider(store)
	host := envOr("SIDECAR_HOST", "0.0.0.0")
	port := envOr("SIDECAR_PORT", "8093")
	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 1 || portNumber > 65535 {
		log.Fatal("SIDECAR_PORT must be a number from 1 to 65535")
	}
	baseURL := envOr("SIDECAR_BASE_URL", "http://"+detectLANIP()+":"+port)
	if err := sidecar.ValidateBaseURL(baseURL); err != nil {
		log.Fatal(err)
	}
	addr := net.JoinHostPort(host, port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("listen on %s: %v", addr, err)
	}
	server := &http.Server{
		Addr: addr, Handler: sidecar.NewAPI(provider, baseURL).Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       90 * time.Second,
	}
	stopped := make(chan os.Signal, 1)
	signal.Notify(stopped, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-stopped
		ctx, cancel := contextWithTimeout()
		defer cancel()
		_ = server.Shutdown(ctx)
	}()
	log.Printf("YouTube Sidecar API listening on %s", addr)
	log.Printf("LAN base URL: %s", baseURL)
	log.Printf("Station list: %s/api/stations", baseURL)
	log.Printf("Desktop setup page: http://127.0.0.1:%s/app", port)
	if err := openDesktopPage(port); err != nil {
		log.Printf("Open the setup page in your browser: http://127.0.0.1:%s/app (%v)", port, err)
	}
	if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func envOr(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func openDesktopPage(port string) error {
	// A short delay lets the HTTP listener become ready before the browser opens.
	time.Sleep(250 * time.Millisecond)
	url := "http://127.0.0.1:" + port + "/app"
	var command string
	var args []string
	switch runtime.GOOS {
	case "windows":
		command, args = "rundll32", []string{"url.dll,FileProtocolHandler", url}
	case "darwin":
		command, args = "open", []string{url}
	default:
		command, args = "xdg-open", []string{url}
	}
	return exec.Command(command, args...).Start()
}

func contextWithTimeout() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 10*time.Second)
}
