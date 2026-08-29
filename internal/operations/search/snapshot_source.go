package searchops

import (
	"fmt"
	"strings"

	mysqldata "github.com/JekYUlll/Dipole/internal/data/mysql"
	searchbackfill "github.com/JekYUlll/Dipole/internal/operations/search/backfill"
)

const (
	SearchSourceMySQL   = "mysql"
	SearchSourceArchive = "archive"
)

func openSearchSnapshotSource(kind, manifestPath string, store *mysqldata.Store) (searchbackfill.Source, error) {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "", SearchSourceMySQL:
		return mysqldata.NewSearchBackfillSource(store)
	case SearchSourceArchive:
		if strings.TrimSpace(manifestPath) == "" {
			return nil, fmt.Errorf("Search archive source requires archive manifest")
		}
		return searchbackfill.OpenArchive(manifestPath)
	default:
		return nil, fmt.Errorf("unsupported Search snapshot source: %s", kind)
	}
}
