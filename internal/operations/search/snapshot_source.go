package searchops

import (
	"fmt"
	"strings"

	searchbackfill "github.com/JekYUlll/Dipole/internal/operations/search/backfill"
	searchmysql "github.com/JekYUlll/Dipole/internal/operations/search/backfill/mysql"
	platformmysql "github.com/JekYUlll/Dipole/internal/platform/mysql"
)

const (
	SearchSourceMySQL   = "mysql"
	SearchSourceArchive = "archive"
)

func openSearchSnapshotSource(kind, manifestPath string, store *platformmysql.Store) (searchbackfill.Source, error) {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "", SearchSourceMySQL:
		return searchmysql.NewSearchBackfillSource(store)
	case SearchSourceArchive:
		if strings.TrimSpace(manifestPath) == "" {
			return nil, fmt.Errorf("Search archive source requires archive manifest")
		}
		return searchbackfill.OpenArchive(manifestPath)
	default:
		return nil, fmt.Errorf("unsupported Search snapshot source: %s", kind)
	}
}
