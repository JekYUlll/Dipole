package bootstrap

import (
	"context"

	searchbackfill "github.com/JekYUlll/Dipole/internal/backfill/search"
	mysqldata "github.com/JekYUlll/Dipole/internal/data/mysql"
)

type SearchArchiveOptions struct {
	ManifestPath string
	SnapshotID   string
	BatchSize    int
}

func RunSearchArchive(ctx context.Context, options SearchArchiveOptions) (searchbackfill.ArchiveManifest, error) {
	db, err := openSearchMaintenanceMySQL(ctx, "archive")
	if err != nil {
		return searchbackfill.ArchiveManifest{}, err
	}
	defer db.Close()
	store, err := mysqldata.NewStore(db)
	if err != nil {
		return searchbackfill.ArchiveManifest{}, err
	}
	source, err := mysqldata.NewSearchBackfillSource(store)
	if err != nil {
		return searchbackfill.ArchiveManifest{}, err
	}
	return searchbackfill.CreateArchive(ctx, source, options.ManifestPath, options.SnapshotID, options.BatchSize)
}
