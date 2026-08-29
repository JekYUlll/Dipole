package repository

import (
	"github.com/JekYUlll/Dipole/internal/data/mysql/generated"
	searchmysql "github.com/JekYUlll/Dipole/internal/services/search/infrastructure/mysql"
)

// Search index aliases preserve embedded and maintenance callers while the
// implementation lives under the Search service boundary.
type SearchIndexRepository = searchmysql.SearchIndexRepository

func NewSearchIndexRepository(queries *generated.Queries) (*SearchIndexRepository, error) {
	return searchmysql.NewSearchIndexRepository(queries)
}
