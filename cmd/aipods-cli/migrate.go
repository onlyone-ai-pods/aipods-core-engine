package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/martinllanos/only-ai-pods/internal/store"
)

const migrateUsage = `Usage: aipods-cli migrate contract [--database-url=URL]

Applies internal/store/migrations/contract/*.sql (SPEC-CORE-73.R16).
Run it only when the previous color no longer exists: these migrations remove
or change structure that the old code may still read. DATABASE_URL is used
when --database-url is not given.`

// handleMigrateCommand implements "aipods-cli migrate contract".
func handleMigrateCommand(args []string) {
	if len(args) < 1 || args[0] != "contract" {
		fmt.Println(migrateUsage)
		os.Exit(1)
	}
	fs := flag.NewFlagSet("migrate contract", flag.ExitOnError)
	dbURL := fs.String("database-url", "", "Postgres URL (default: $DATABASE_URL)")
	_ = fs.Parse(args[1:])

	url := strings.TrimSpace(*dbURL)
	if url == "" {
		url = strings.TrimSpace(os.Getenv("DATABASE_URL"))
	}
	if url == "" {
		fmt.Println("❌ DATABASE_URL is not set and --database-url was not given")
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	applied, err := runContractMigrations(ctx, url)
	for _, v := range applied {
		fmt.Printf("✅ applied %s\n", v)
	}
	if err != nil {
		fmt.Printf("❌ %v\n", err)
		stop()
		os.Exit(1)
	}
	if len(applied) == 0 {
		fmt.Println("No pending contract migrations.")
	}
}

func runContractMigrations(ctx context.Context, url string) ([]string, error) {
	p, err := store.OpenPostgres(ctx, url)
	if err != nil {
		return nil, err
	}
	defer p.Close()
	return p.MigrateContract(ctx)
}
