package cassandra

import "testing"

func TestNormalizedHostsTrimsAndDropsEmptyEntries(t *testing.T) {
	hosts := normalizedHosts([]string{" cassandra-1:9042 ", "", " cassandra-2:9042"})
	if len(hosts) != 2 || hosts[0] != "cassandra-1:9042" || hosts[1] != "cassandra-2:9042" {
		t.Fatalf("unexpected normalized hosts: %v", hosts)
	}
}
