package snapshot

import (
	"testing"
)

// makeBaselines builds a *Baselines from variadic BaselineEntry values.
func makeBaselines(entries ...BaselineEntry) *Baselines {
	if entries == nil {
		return &Baselines{Entries: []BaselineEntry{}}
	}
	return &Baselines{Entries: entries}
}

// makeMeta builds a *SnapshotMeta with the given id and urls.
func makeMeta(id string, urls ...string) *SnapshotMeta {
	return &SnapshotMeta{ID: id, URLs: urls}
}

// containsURL returns true if url is present in the slice.
func containsURL(slice []string, url string) bool {
	for _, s := range slice {
		if s == url {
			return true
		}
	}
	return false
}

// entryByURL finds a BaselineEntry by URL; returns zero value if not found.
func entryByURL(entries []BaselineEntry, url string) (BaselineEntry, bool) {
	for _, e := range entries {
		if e.URL == url {
			return e, true
		}
	}
	return BaselineEntry{}, false
}

// TestAccept_EmptyBaselines: promoting two URLs into an empty baseline set.
func TestAccept_EmptyBaselines(t *testing.T) {
	current := makeBaselines()
	meta := makeMeta("snap-1", "/home", "/about")

	updated, changed := Accept(current, meta, nil)

	if updated == nil {
		t.Fatal("Accept returned nil updated")
	}
	if len(updated.Entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(updated.Entries))
	}
	if len(changed) != 2 {
		t.Errorf("expected 2 changed URLs, got %d", len(changed))
	}
	if !containsURL(changed, "/home") {
		t.Error("expected /home in changed")
	}
	if !containsURL(changed, "/about") {
		t.Error("expected /about in changed")
	}
	for _, url := range []string{"/home", "/about"} {
		e, ok := entryByURL(updated.Entries, url)
		if !ok {
			t.Errorf("expected entry for %s", url)
			continue
		}
		if e.SnapshotID != "snap-1" {
			t.Errorf("entry for %s: expected snapshotId snap-1, got %s", url, e.SnapshotID)
		}
	}
}

// TestAccept_AllURLs: promote all URLs when urls arg is nil (replaces existing baselines).
func TestAccept_AllURLs(t *testing.T) {
	current := makeBaselines(
		BaselineEntry{URL: "/home", SnapshotID: "snap-old"},
		BaselineEntry{URL: "/about", SnapshotID: "snap-old"},
	)
	meta := makeMeta("snap-new", "/home", "/about")

	updated, changed := Accept(current, meta, nil)

	if updated == nil {
		t.Fatal("Accept returned nil updated")
	}
	if len(updated.Entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(updated.Entries))
	}
	if len(changed) != 2 {
		t.Errorf("expected 2 changed URLs, got %d: %v", len(changed), changed)
	}
	for _, url := range []string{"/home", "/about"} {
		e, ok := entryByURL(updated.Entries, url)
		if !ok {
			t.Errorf("missing entry for %s", url)
			continue
		}
		if e.SnapshotID != "snap-new" {
			t.Errorf("%s: expected snap-new, got %s", url, e.SnapshotID)
		}
	}
}

// TestAccept_SingleURL: --url filter leaves other entries unchanged.
func TestAccept_SingleURL(t *testing.T) {
	current := makeBaselines(
		BaselineEntry{URL: "/dashboard", SnapshotID: "snap-old"},
		BaselineEntry{URL: "/login", SnapshotID: "snap-old"},
	)
	meta := makeMeta("snap-new", "/dashboard", "/login")

	updated, changed := Accept(current, meta, []string{"/dashboard"})

	if updated == nil {
		t.Fatal("Accept returned nil updated")
	}

	// Only /dashboard should be changed
	if len(changed) != 1 || changed[0] != "/dashboard" {
		t.Errorf("expected changed=[/dashboard], got %v", changed)
	}

	// /login must be unchanged
	loginEntry, ok := entryByURL(updated.Entries, "/login")
	if !ok {
		t.Error("expected /login entry in updated")
	} else if loginEntry.SnapshotID != "snap-old" {
		t.Errorf("/login: expected snap-old, got %s", loginEntry.SnapshotID)
	}

	// /dashboard must be updated
	dashEntry, ok := entryByURL(updated.Entries, "/dashboard")
	if !ok {
		t.Error("expected /dashboard entry in updated")
	} else if dashEntry.SnapshotID != "snap-new" {
		t.Errorf("/dashboard: expected snap-new, got %s", dashEntry.SnapshotID)
	}
}

// TestAccept_AlreadyBaseline: accepting an already-current baseline is a no-op.
func TestAccept_AlreadyBaseline(t *testing.T) {
	current := makeBaselines(
		BaselineEntry{URL: "/dashboard", SnapshotID: "snap-A"},
	)
	meta := makeMeta("snap-A", "/dashboard")

	updated, changed := Accept(current, meta, []string{"/dashboard"})

	if updated == nil {
		t.Fatal("Accept returned nil updated")
	}
	if len(changed) != 0 {
		t.Errorf("expected no changes (idempotent), got changed=%v", changed)
	}
	e, ok := entryByURL(updated.Entries, "/dashboard")
	if !ok {
		t.Error("expected /dashboard entry in updated")
	} else if e.SnapshotID != "snap-A" {
		t.Errorf("/dashboard: expected snap-A, got %s", e.SnapshotID)
	}
}

// TestAccept_UpdatesExisting: accepting a new snapshot updates an existing baseline entry.
func TestAccept_UpdatesExisting(t *testing.T) {
	current := makeBaselines(
		BaselineEntry{URL: "/login", SnapshotID: "snap-old"},
	)
	meta := makeMeta("snap-new", "/login")

	updated, changed := Accept(current, meta, []string{"/login"})

	if updated == nil {
		t.Fatal("Accept returned nil updated")
	}
	if len(changed) != 1 || changed[0] != "/login" {
		t.Errorf("expected changed=[/login], got %v", changed)
	}
	e, ok := entryByURL(updated.Entries, "/login")
	if !ok {
		t.Error("expected /login entry in updated")
	} else if e.SnapshotID != "snap-new" {
		t.Errorf("/login: expected snap-new, got %s", e.SnapshotID)
	}
	// No duplicate entries
	if len(updated.Entries) != 1 {
		t.Errorf("expected 1 entry (no duplicates), got %d", len(updated.Entries))
	}
}

// TestAccept_DoesNotMutateCurrent: Accept must not mutate the input Baselines.
func TestAccept_DoesNotMutateCurrent(t *testing.T) {
	current := makeBaselines(
		BaselineEntry{URL: "/home", SnapshotID: "snap-old"},
	)
	meta := makeMeta("snap-new", "/home")

	_, _ = Accept(current, meta, nil)

	// current must be unchanged
	if len(current.Entries) != 1 {
		t.Fatalf("Accept mutated current: expected 1 entry, got %d", len(current.Entries))
	}
	if current.Entries[0].SnapshotID != "snap-old" {
		t.Errorf("Accept mutated current.Entries[0].SnapshotID: got %s", current.Entries[0].SnapshotID)
	}
}

// TestAccept_ChangedNeverNil: changed must be an empty slice (not nil) when there are no changes.
func TestAccept_ChangedNeverNil(t *testing.T) {
	current := makeBaselines(
		BaselineEntry{URL: "/home", SnapshotID: "snap-A"},
	)
	meta := makeMeta("snap-A", "/home")

	_, changed := Accept(current, meta, nil)

	if changed == nil {
		t.Error("expected changed to be non-nil empty slice, got nil")
	}
}
