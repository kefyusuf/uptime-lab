package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/kefyusuf/uptime-lab/apps/api/migrations"
)

const migrationTimeout = 2 * time.Minute

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 || !validCommand(args[0]) {
		printUsage(stderr)
		return 2
	}
	command := args[0]

	logger := slog.New(slog.NewJSONHandler(stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	config, err := pgx.ParseConfig("")
	if err != nil {
		return fail(logger, command, "config")
	}

	db := stdlib.OpenDB(*config)
	defer db.Close()

	provider, err := migrations.NewProvider(db)
	if err != nil {
		return fail(logger, command, "provider")
	}

	ctx, cancel := context.WithTimeout(context.Background(), migrationTimeout)
	defer cancel()

	switch command {
	case "up":
		results, err := provider.Up(ctx)
		if err != nil {
			return fail(logger, command, "execute")
		}
		fmt.Fprintf(stdout, "applied %d migration(s)\n", len(results))
	case "down":
		result, err := provider.Down(ctx)
		if err != nil {
			return fail(logger, command, "execute")
		}
		if result == nil {
			fmt.Fprintln(stdout, "rolled back 0 migration(s)")
		} else {
			fmt.Fprintln(stdout, "rolled back 1 migration(s)")
		}
	case "status":
		status, err := provider.Status(ctx)
		if err != nil {
			return fail(logger, command, "execute")
		}
		for _, migration := range status {
			fmt.Fprintf(stdout, "%s\t%05d\t%s\n", migration.State, migration.Source.Version, migration.Source.Path)
		}
	}

	return 0
}

func validCommand(command string) bool {
	switch command {
	case "up", "down", "status":
		return true
	default:
		return false
	}
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "usage: uptime-lab-migrate {up|down|status}")
}

func fail(logger *slog.Logger, command, stage string) int {
	logger.Error(
		"migration command failed",
		"service", "migrate",
		"component", "database-migrations",
		"event", "migration_failed",
		"command", command,
		"stage", stage,
	)
	return 1
}
