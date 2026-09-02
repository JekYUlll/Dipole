package cassandraschema

import (
	"embed"
	"fmt"
	"sort"
	"strings"
)

//go:embed *.cql
var files embed.FS

func Statements() ([]string, error) {
	entries, err := files.ReadDir(".")
	if err != nil {
		return nil, fmt.Errorf("read Cassandra schema files: %w", err)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })

	var statements []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".cql") {
			continue
		}
		content, err := files.ReadFile(entry.Name())
		if err != nil {
			return nil, fmt.Errorf("read Cassandra schema %s: %w", entry.Name(), err)
		}
		for _, statement := range strings.Split(string(content), ";") {
			if statement = strings.TrimSpace(statement); statement != "" {
				statements = append(statements, statement)
			}
		}
	}
	return statements, nil
}
