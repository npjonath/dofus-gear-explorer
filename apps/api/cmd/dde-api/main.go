// Command dde-api imports Dofus game data into SQLite and serves it.
//
//	dde-api serve   start the HTTP API (default)
//	dde-api sync    import the latest release, or --version / --source
//	dde-api check   report whether a newer release is available
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"dofusdb/api/internal/httpapi"
	"dofusdb/api/internal/release"
	"dofusdb/api/internal/store"
	"dofusdb/api/internal/updater"
)

type config struct {
	addr          string
	dataDir       string
	langs         string
	repo          string
	imageSize     int
	checkInterval time.Duration
	autoUpdate    bool
	keepCache     bool
	corsOrigin    string
	webDir        string
	version       string
	source        string
	followStdin   bool
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func parseFlags(name string, args []string) (*config, error) {
	cfg := &config{}
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.StringVar(&cfg.addr, "addr", env("DDE_ADDR", "127.0.0.1:8787"), "address to listen on (use port 0 for a free port)")
	fs.StringVar(&cfg.dataDir, "data-dir", env("DDE_DATA_DIR", "data"), "directory for the SQLite database and item icons")
	fs.StringVar(&cfg.langs, "langs", env("DDE_LANGS", "fr,en,es,pt"), "languages to import, comma separated")
	fs.StringVar(&cfg.repo, "repo", env("DDE_RELEASE_REPO", release.DefaultRepo), "GitHub repository publishing the game data")
	fs.IntVar(&cfg.imageSize, "image-size", 128, "item icon size to import: 64 or 128")
	fs.DurationVar(&cfg.checkInterval, "check-interval", 6*time.Hour, "how often to look for a new release")
	fs.BoolVar(&cfg.autoUpdate, "auto-update", env("DDE_AUTO_UPDATE", "") == "true", "import new releases automatically instead of only reporting them")
	fs.BoolVar(&cfg.keepCache, "keep-cache", false, "keep downloaded release files after import")
	fs.StringVar(&cfg.corsOrigin, "cors-origin", env("DDE_CORS_ORIGIN", "http://localhost:5173,http://127.0.0.1:5173"), "browser origins allowed to call the API, comma separated (\"*\" allows any)")
	fs.StringVar(&cfg.webDir, "web-dir", env("DDE_WEB_DIR", ""), "serve a built web app from this directory")
	fs.BoolVar(&cfg.followStdin, "exit-with-stdin", false, "serve: stop when standard input closes, so the API never outlives the app that started it")
	fs.StringVar(&cfg.version, "version", "", "sync: release tag to import instead of the latest")
	fs.StringVar(&cfg.source, "source", "", "sync: import release files from a local directory instead of GitHub")
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	return cfg, nil
}

func main() {
	command := "serve"
	args := os.Args[1:]
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		command, args = args[0], args[1:]
	}
	cfg, err := parseFlags(command, args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		os.Exit(2)
	}
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if err := run(command, cfg, log); err != nil {
		log.Error(err.Error())
		os.Exit(1)
	}
}

func run(command string, cfg *config, log *slog.Logger) error {
	st, err := store.Open(filepath.Join(cfg.dataDir, "dofus.db"))
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer st.Close()

	var langs []string
	for _, l := range strings.Split(cfg.langs, ",") {
		if l = strings.TrimSpace(l); l != "" {
			langs = append(langs, l)
		}
	}
	var origins []string
	for _, o := range strings.Split(cfg.corsOrigin, ",") {
		if o = strings.TrimSpace(o); o != "" {
			origins = append(origins, o)
		}
	}
	// Bound to this computer only, the API also rejects requests addressed to another name.
	access := httpapi.Access{AllowedOrigins: origins, LocalOnly: httpapi.IsLoopbackHost(cfg.addr)}
	server := httpapi.New(st, log, access, cfg.webDir)
	client := release.NewClient(cfg.repo, os.Getenv("GITHUB_TOKEN"))
	up := updater.New(st, client, updater.Options{
		DataDir: cfg.dataDir, Langs: langs, ImageSize: cfg.imageSize,
		CheckInterval: cfg.checkInterval, AutoUpdate: cfg.autoUpdate, KeepCache: cfg.keepCache,
	}, log, server.SetDataset)
	server.SetUpdater(up)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	switch command {
	case "sync":
		if cfg.source != "" {
			version := cfg.version
			if version == "" {
				version = updater.LocalVersionPrefix + time.Now().UTC().Format("20060102-150405")
			}
			return up.ImportLocal(ctx, cfg.source, version)
		}
		return up.Update(ctx, cfg.version)

	case "check":
		status, err := up.Check(ctx)
		if err != nil {
			return err
		}
		switch {
		case status.InstalledVersion == "":
			fmt.Printf("No data installed. Latest release: %s. Run `dde-api sync`.\n", status.LatestVersion)
		case status.UpdateAvailable:
			fmt.Printf("Update available: %s → %s. Run `dde-api sync`.\n", status.InstalledVersion, status.LatestVersion)
		default:
			fmt.Printf("Up to date: %s.\n", status.InstalledVersion)
		}
		return nil

	case "serve":
		if err := up.Reload(ctx); err != nil {
			return err
		}
		listener, err := net.Listen("tcp", cfg.addr)
		if err != nil {
			return err
		}
		// The desktop shell reads this line to learn which port was picked.
		fmt.Printf("DDE_API_LISTENING http://%s\n", listener.Addr().String())
		go up.Run(ctx)
		if cfg.followStdin {
			go func() {
				_, _ = io.Copy(io.Discard, os.Stdin)
				stop()
			}()
		}

		httpServer := &http.Server{Handler: server.Handler(), ReadHeaderTimeout: 10 * time.Second}
		go func() {
			<-ctx.Done()
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = httpServer.Shutdown(shutdownCtx)
		}()
		if err := httpServer.Serve(listener); !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	}
	return fmt.Errorf("unknown command %q (use serve, sync or check)", command)
}
