package backup

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"backupsync/internal/config"

	"go.uber.org/zap"
)

// touchDump creates a dump file with the given age.
func touchDump(t *testing.T, dir, name string, age time.Duration) {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	mod := time.Now().Add(-age)
	if err := os.Chtimes(p, mod, mod); err != nil {
		t.Fatal(err)
	}
}

func names(t *testing.T, dir string) map[string]bool {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]bool{}
	for _, e := range entries {
		out[e.Name()] = true
	}
	return out
}

func TestKeepLastKeepsNewest(t *testing.T) {
	dir := t.TempDir()
	touchDump(t, dir, "db_a.dump", 30*time.Hour)
	touchDump(t, dir, "db_b.dump", 20*time.Hour)
	touchDump(t, dir, "db_c.dump", 1*time.Hour)
	touchDump(t, dir, "other_x.dump", 40*time.Hour) // foreign prefix
	touchDump(t, dir, "db_d.txt", 50*time.Hour)     // not a dump

	b := NewDBackuper(zap.NewNop(), config.DB{Path: dir, Prefix: "db_", KeepLast: 1})
	if err := b.CleanupOldBackups(); err != nil {
		t.Fatal(err)
	}

	got := names(t, dir)
	for _, want := range []string{"db_c.dump", "other_x.dump", "db_d.txt"} {
		if !got[want] {
			t.Errorf("%s must stay, left: %v", want, got)
		}
	}
	for _, gone := range []string{"db_a.dump", "db_b.dump"} {
		if got[gone] {
			t.Errorf("%s must be deleted, left: %v", gone, got)
		}
	}
}

func TestKeepLastZeroIsOff(t *testing.T) {
	dir := t.TempDir()
	touchDump(t, dir, "db_a.dump", 30*time.Hour)
	touchDump(t, dir, "db_b.dump", 1*time.Hour)

	b := NewDBackuper(zap.NewNop(), config.DB{Path: dir, Prefix: "db_"})
	if err := b.CleanupOldBackups(); err != nil {
		t.Fatal(err)
	}
	if got := names(t, dir); len(got) != 2 {
		t.Errorf("nothing may be deleted without rules, left: %v", got)
	}
}

func TestRetentionDaysStillWorks(t *testing.T) {
	dir := t.TempDir()
	touchDump(t, dir, "db_old.dump", 72*time.Hour)
	touchDump(t, dir, "db_new.dump", 1*time.Hour)

	b := NewDBackuper(zap.NewNop(), config.DB{Path: dir, Prefix: "db_", RetentionDays: 1})
	if err := b.CleanupOldBackups(); err != nil {
		t.Fatal(err)
	}
	got := names(t, dir)
	if got["db_old.dump"] || !got["db_new.dump"] {
		t.Errorf("age retention broke, left: %v", got)
	}
}

func TestKeepLastMoreThanFiles(t *testing.T) {
	dir := t.TempDir()
	touchDump(t, dir, "db_a.dump", 2*time.Hour)

	b := NewDBackuper(zap.NewNop(), config.DB{Path: dir, Prefix: "db_", KeepLast: 5})
	if err := b.CleanupOldBackups(); err != nil {
		t.Fatal(err)
	}
	if got := names(t, dir); !got["db_a.dump"] {
		t.Errorf("a lone dump must stay, left: %v", got)
	}
}
