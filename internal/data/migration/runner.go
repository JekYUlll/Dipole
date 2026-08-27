package migration

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"time"
)

const ledgerTable = "schema_migrations"
const migrationLockTimeoutSeconds = 30

var migrationFilePattern = regexp.MustCompile(`^(\d{6})_([a-z0-9_]+)\.(up|down)\.sql$`)

type Migration struct {
	Version int64
	Name    string
	UpSQL   string
	DownSQL string
}

type Runner struct {
	db         *sql.DB
	migrations []Migration
}

type migrationExecutor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func NewRunner(db *sql.DB, files fs.FS) (*Runner, error) {
	if db == nil {
		return nil, errors.New("migration database is required")
	}

	migrations, err := load(files)
	if err != nil {
		return nil, err
	}
	return &Runner{db: db, migrations: migrations}, nil
}

func (r *Runner) Up(ctx context.Context) error {
	return r.withMigrationLock(ctx, func(connection migrationExecutor) error {
		return r.up(ctx, connection)
	})
}

func (r *Runner) up(ctx context.Context, connection migrationExecutor) error {
	if err := r.ensureLedger(ctx, connection); err != nil {
		return err
	}

	applied, err := r.appliedVersions(ctx, connection)
	if err != nil {
		return err
	}
	for _, migration := range r.migrations {
		if _, ok := applied[migration.Version]; ok {
			continue
		}
		if _, err := connection.ExecContext(ctx, migration.UpSQL); err != nil {
			return fmt.Errorf("apply migration %06d_%s: %w", migration.Version, migration.Name, err)
		}
		if _, err := connection.ExecContext(ctx,
			"INSERT INTO schema_migrations (version, name) VALUES (?, ?)",
			migration.Version,
			migration.Name,
		); err != nil {
			return fmt.Errorf("record migration %06d_%s: %w", migration.Version, migration.Name, err)
		}
	}
	return nil
}

func (r *Runner) Down(ctx context.Context, steps int) error {
	if steps < 1 {
		return errors.New("migration rollback steps must be positive")
	}
	return r.withMigrationLock(ctx, func(connection migrationExecutor) error {
		return r.down(ctx, connection, steps)
	})
}

func (r *Runner) down(ctx context.Context, connection migrationExecutor, steps int) error {
	if err := r.ensureLedger(ctx, connection); err != nil {
		return err
	}

	applied, err := r.appliedVersionsDescending(ctx, connection)
	if err != nil {
		return err
	}
	if steps > len(applied) {
		return fmt.Errorf("rollback %d migration(s): only %d applied", steps, len(applied))
	}
	byVersion := make(map[int64]Migration, len(r.migrations))
	for _, migration := range r.migrations {
		byVersion[migration.Version] = migration
	}
	for _, version := range applied[:steps] {
		migration, ok := byVersion[version]
		if !ok {
			return fmt.Errorf("rollback migration %06d: migration file is missing", version)
		}
		if _, err := connection.ExecContext(ctx, migration.DownSQL); err != nil {
			return fmt.Errorf("rollback migration %06d_%s: %w", migration.Version, migration.Name, err)
		}
		if _, err := connection.ExecContext(ctx, "DELETE FROM schema_migrations WHERE version = ?", version); err != nil {
			return fmt.Errorf("remove migration ledger entry %06d: %w", version, err)
		}
	}
	return nil
}

func (r *Runner) withMigrationLock(ctx context.Context, run func(migrationExecutor) error) error {
	connection, err := r.db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("acquire migration lock connection: %w", err)
	}
	defer connection.Close()

	var acquired sql.NullInt64
	if err := connection.QueryRowContext(ctx,
		"SELECT GET_LOCK(CONCAT('dipole:migrate:', LEFT(SHA2(DATABASE(), 256), 48)), ?)",
		migrationLockTimeoutSeconds,
	).Scan(&acquired); err != nil {
		return fmt.Errorf("acquire migration lock: %w", err)
	}
	if !acquired.Valid || acquired.Int64 != 1 {
		return errors.New("acquire migration lock: timed out")
	}
	defer func() {
		releaseCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = connection.ExecContext(releaseCtx,
			"SELECT RELEASE_LOCK(CONCAT('dipole:migrate:', LEFT(SHA2(DATABASE(), 256), 48)))",
		)
	}()
	return run(connection)
}

func (r *Runner) CurrentVersion(ctx context.Context) (int64, error) {
	if err := r.ensureLedger(ctx, r.db); err != nil {
		return 0, err
	}
	var version sql.NullInt64
	if err := r.db.QueryRowContext(ctx, "SELECT MAX(version) FROM schema_migrations").Scan(&version); err != nil {
		return 0, fmt.Errorf("read current migration version: %w", err)
	}
	return version.Int64, nil
}

func (r *Runner) ValidateCurrent(ctx context.Context) error {
	applied, err := r.appliedVersions(ctx, r.db)
	if err != nil {
		return err
	}
	for _, migration := range r.migrations {
		if _, ok := applied[migration.Version]; !ok {
			return fmt.Errorf("database migration %06d_%s is missing; run cmd/migrate", migration.Version, migration.Name)
		}
	}
	return nil
}

func (r *Runner) ensureLedger(ctx context.Context, connection migrationExecutor) error {
	_, err := connection.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
    version BIGINT NOT NULL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    applied_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`)
	if err != nil {
		return fmt.Errorf("ensure migration ledger: %w", err)
	}
	return nil
}

func (r *Runner) appliedVersions(ctx context.Context, connection migrationExecutor) (map[int64]struct{}, error) {
	rows, err := connection.QueryContext(ctx, "SELECT version FROM schema_migrations")
	if err != nil {
		return nil, fmt.Errorf("list applied migrations: %w", err)
	}
	defer rows.Close()

	versions := make(map[int64]struct{})
	for rows.Next() {
		var version int64
		if err := rows.Scan(&version); err != nil {
			return nil, fmt.Errorf("scan applied migration: %w", err)
		}
		versions[version] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate applied migrations: %w", err)
	}
	return versions, nil
}

func (r *Runner) appliedVersionsDescending(ctx context.Context, connection migrationExecutor) ([]int64, error) {
	rows, err := connection.QueryContext(ctx, "SELECT version FROM schema_migrations ORDER BY version DESC")
	if err != nil {
		return nil, fmt.Errorf("list applied migrations: %w", err)
	}
	defer rows.Close()

	var versions []int64
	for rows.Next() {
		var version int64
		if err := rows.Scan(&version); err != nil {
			return nil, fmt.Errorf("scan applied migration: %w", err)
		}
		versions = append(versions, version)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate applied migrations: %w", err)
	}
	return versions, nil
}

type migrationFiles struct {
	version int64
	name    string
	up      string
	down    string
}

func load(files fs.FS) ([]Migration, error) {
	entries, err := fs.ReadDir(files, ".")
	if err != nil {
		return nil, fmt.Errorf("read migration files: %w", err)
	}

	pairs := make(map[int64]*migrationFiles)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		matches := migrationFilePattern.FindStringSubmatch(filepath.Base(entry.Name()))
		if matches == nil {
			continue
		}
		version, err := strconv.ParseInt(matches[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parse migration version %q: %w", matches[1], err)
		}
		pair := pairs[version]
		if pair == nil {
			pair = &migrationFiles{version: version, name: matches[2]}
			pairs[version] = pair
		} else if pair.name != matches[2] {
			return nil, fmt.Errorf("migration version %06d has conflicting names %q and %q", version, pair.name, matches[2])
		}
		contents, err := fs.ReadFile(files, entry.Name())
		if err != nil {
			return nil, fmt.Errorf("read migration %s: %w", entry.Name(), err)
		}
		if matches[3] == "up" {
			pair.up = string(contents)
		} else {
			pair.down = string(contents)
		}
	}

	versions := make([]int64, 0, len(pairs))
	for version := range pairs {
		versions = append(versions, version)
	}
	sort.Slice(versions, func(i, j int) bool { return versions[i] < versions[j] })

	migrations := make([]Migration, 0, len(versions))
	for _, version := range versions {
		pair := pairs[version]
		if pair.up == "" || pair.down == "" {
			return nil, fmt.Errorf("migration %06d_%s requires paired up and down files", version, pair.name)
		}
		migrations = append(migrations, Migration{
			Version: version,
			Name:    pair.name,
			UpSQL:   pair.up,
			DownSQL: pair.down,
		})
	}
	return migrations, nil
}
