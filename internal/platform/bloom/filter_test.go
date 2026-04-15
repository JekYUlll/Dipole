package bloom

import "testing"

func TestFilterAddAndTest(t *testing.T) {
	t.Parallel()

	filter := NewFilter(100, 0.01)
	filter.Add("U100")
	filter.Add("G200")

	if !filter.Test("U100") {
		t.Fatalf("expected U100 to be present")
	}
	if !filter.Test("G200") {
		t.Fatalf("expected G200 to be present")
	}
	if filter.Test("U404") {
		t.Fatalf("expected U404 to be absent in low-volume test")
	}
}

func TestRegistryLoadAndReset(t *testing.T) {
	t.Parallel()

	Reset()
	Load([]string{"U100", "U200"}, []string{"G100"})
	if !UserMayExist("U100") {
		t.Fatalf("expected U100 to exist after registry load")
	}
	if !GroupMayExist("G100") {
		t.Fatalf("expected G100 to exist after registry load")
	}
	userCount, groupCount := Stats()
	if userCount != 2 || groupCount != 1 {
		t.Fatalf("unexpected bloom stats: users=%d groups=%d", userCount, groupCount)
	}

	AddUser("U300")
	AddGroup("G200")
	userCount, groupCount = Stats()
	if userCount != 3 || groupCount != 2 {
		t.Fatalf("unexpected bloom stats after add: users=%d groups=%d", userCount, groupCount)
	}

	Reset()
	if !UserMayExist("U404") {
		t.Fatalf("expected uninitialized registry to fall back to maybe-exists")
	}
	userCount, groupCount = Stats()
	if userCount != 0 || groupCount != 0 {
		t.Fatalf("expected bloom stats reset to zero, users=%d groups=%d", userCount, groupCount)
	}
}
