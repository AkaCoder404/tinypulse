package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gopkg.in/yaml.v3"

	"tinypulse/internal/config"
	"tinypulse/internal/db"
	"tinypulse/internal/model"
	"tinypulse/internal/monitor"
	"tinypulse/internal/notifier"
	"tinypulse/internal/server"
)

func getEnvOrDefault(key, fallback string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return fallback
}

func main() {
	var (
		addr         = flag.String("addr", getEnvOrDefault("TINYPULSE_ADDR", ":8080"), "HTTP listen address")
		dbPath       = flag.String("db", getEnvOrDefault("TINYPULSE_DB", "./tinypulse.db"), "SQLite database path")
		pass         = flag.String("password", getEnvOrDefault("TINYPULSE_PASSWORD", ""), "Admin password for UI and API")
		configPath   = flag.String("config", getEnvOrDefault("TINYPULSE_CONFIG", ""), "Path to YAML configuration file (optional)")
		dryRun       = flag.Bool("dry-run", false, "Parse config, preview DB changes, and exit without applying them")
		ejectConfig  = flag.Bool("eject-config", false, "Unlock all config-managed items in the database, reverting them to UI control")
		exportConfig = flag.String("export-config", "", "Export the current database state to a YAML configuration file")
	)
	flag.Parse()

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, nil)))

	// ---------------------------------------------------------
	// Utility Command: Export Config
	// Reads the current database and exports it to a YAML file.
	// ---------------------------------------------------------
	if *exportConfig != "" {
		database, err := db.New(*dbPath)
		if err != nil {
			slog.Error("failed to open database to export config", "error", err)
			os.Exit(1)
		}
		defer database.Close()

		cfg, err := database.ExportConfig(context.Background())
		if err != nil {
			slog.Error("failed to export database", "error", err)
			os.Exit(1)
		}

		yamlData, err := yaml.Marshal(cfg)
		if err != nil {
			slog.Error("failed to marshal config to YAML", "error", err)
			os.Exit(1)
		}

		if err := os.WriteFile(*exportConfig, yamlData, 0644); err != nil {
			slog.Error("failed to write YAML file", "error", err)
			os.Exit(1)
		}

		slog.Info("successfully exported database to YAML configuration", "path", *exportConfig)
		os.Exit(0)
	}

	// ---------------------------------------------------------
	// Utility Command: Eject Config
	// Unlocks all config-managed items so they can be edited in the UI again.
	// ---------------------------------------------------------
	if *ejectConfig {
		database, err := db.New(*dbPath)
		if err != nil {
			slog.Error("failed to open database to eject config", "error", err)
			os.Exit(1)
		}
		defer database.Close()
		if err := database.EjectConfigItems(context.Background()); err != nil {
			slog.Error("failed to eject config items", "error", err)
			os.Exit(1)
		}
		slog.Info("successfully ejected config items. They are now managed by the UI.")
		os.Exit(0)
	}

	// ---------------------------------------------------------
	// Primary Execution Flow
	// ---------------------------------------------------------

	// 1. If a config file is provided, parse and deeply validate it before touching the DB.
	if *configPath != "" {
		cfg, err := config.Parse(*configPath)
		if err != nil {
			slog.Error("failed to parse config", "error", err)
			os.Exit(1)
		}

		// Deep validate all notifier configurations (checking for missing bot tokens, etc.)
		for uid, nc := range cfg.Notifiers {
			configJSON, err := json.Marshal(nc.Config)
			if err != nil {
				slog.Error(fmt.Sprintf("notifier %q has an invalid config block", uid), "error", err)
				os.Exit(1)
			}
			dummyNotifier := &model.Notifier{
				Name:       nc.Name,
				Type:       nc.Type,
				ConfigJSON: string(configJSON),
			}
			if _, err := notifier.Build(dummyNotifier); err != nil {
				slog.Error(fmt.Sprintf("invalid configuration for %q notifier %q", nc.Type, uid), "error", err)
				os.Exit(1)
			}
		}

		// 2. Handle --dry-run
		// If dry-run is enabled, we preview changes and immediately exit.
		// We avoid creating the DB file if it doesn't already exist.
		if *dryRun {
			if _, err := os.Stat(*dbPath); os.IsNotExist(err) {
				if err := db.PrintAllAsCreates(cfg); err != nil {
					slog.Error("failed to run dry run diff", "error", err)
					os.Exit(1)
				}
				os.Exit(0)
			}

			database, err := db.New(*dbPath)
			if err != nil {
				slog.Error("failed to open database for dry run", "error", err)
				os.Exit(1)
			}
			defer database.Close()

			if err := database.PrintDryRunDiff(context.Background(), cfg); err != nil {
				slog.Error("failed to run dry run diff", "error", err)
				os.Exit(1)
			}
			os.Exit(0)
		}
	} else if *dryRun {
		slog.Error("--dry-run requires a --config file to be specified")
		os.Exit(1)
	}

	// 3. Open the main Database connection
	database, err := db.New(*dbPath)
	if err != nil {
		slog.Error("failed to open database", "error", err)
		os.Exit(1)
	}
	defer database.Close()

	// 4. Synchronize the YAML Config into the Database
	if *configPath != "" {
		cfg, err := config.Parse(*configPath)
		if err != nil {
			slog.Error("failed to parse config after db initialization", "error", err)
			os.Exit(1)
		}
		if err := database.SyncConfig(context.Background(), cfg); err != nil {
			slog.Error("failed to sync configuration to database", "error", err)
			os.Exit(1)
		}
		slog.Info("successfully synced configuration")
	} else {
		// Warn the user if they started without a config file, but locked config items still exist
		orphanCount, err := database.CountConfigItems(context.Background())
		if err != nil {
			slog.Warn("failed to check for orphaned config items", "error", err)
		} else if orphanCount > 0 {
			slog.Warn(fmt.Sprintf("found %d config-managed items in the database, but no --config file was provided. These items will remain locked in the UI.", orphanCount))
			slog.Warn("to safely unlock them back to UI control, run 'tinypulse --eject-config'")
		}
	}

	// 5. Start the background monitoring and alert managers
	dispatcher := notifier.NewDispatcher(database)
	manager := monitor.New(database, dispatcher)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := manager.Start(ctx); err != nil {
		slog.Error("failed to start manager", "error", err)
		os.Exit(1)
	}

	// 6. Start the HTTP Server and UI
	configActive := *configPath != ""
	srv := server.New(database, manager, *pass, configActive)
	httpServer := &http.Server{
		Addr:         *addr,
		Handler:      srv.Handler(),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		slog.Info("server starting", "addr", *addr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			stop()
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down gracefully")

	manager.Stop()
	dispatcher.Stop()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		slog.Error("shutdown error", "error", err)
	}

	slog.Info("shutdown complete")
	fmt.Println("TinyPulse stopped.")
}
