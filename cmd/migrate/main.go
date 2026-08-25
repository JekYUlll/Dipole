package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/JekYUlll/Dipole/db/migrations"
	"github.com/JekYUlll/Dipole/internal/config"
	"github.com/JekYUlll/Dipole/internal/data/migration"
	"github.com/JekYUlll/Dipole/internal/data/mysqlconfig"
	_ "github.com/go-sql-driver/mysql"
)

func main() {
	direction := flag.String("direction", "up", "migration direction: up or down")
	steps := flag.Int("steps", 1, "number of migrations to roll back when direction=down")
	allowDestructive := flag.Bool("allow-destructive", false, "confirm destructive down migrations")
	flag.Parse()

	if err := run(*direction, *steps, *allowDestructive); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(direction string, steps int, allowDestructive bool) error {
	if direction == "down" && !allowDestructive {
		return fmt.Errorf("down migrations require -allow-destructive")
	}

	if err := config.Load(); err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	db, err := sql.Open("mysql", mysqlconfig.DSN(config.MySQLConfig(), true))
	if err != nil {
		return fmt.Errorf("open mysql: %w", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("ping mysql: %w", err)
	}

	runner, err := migration.NewRunner(db, migrations.Files)
	if err != nil {
		return err
	}
	switch direction {
	case "up":
		err = runner.Up(ctx)
	case "down":
		err = runner.Down(ctx, steps)
	default:
		return fmt.Errorf("unsupported migration direction %q", direction)
	}
	if err != nil {
		return err
	}

	version, err := runner.CurrentVersion(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("database migration complete: version=%06d\n", version)
	return nil
}
