package restore

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"backupsync/internal/config"

	"go.uber.org/zap"
)

func TestPick(t *testing.T) {
	a := []Archive{{Name: "gazen_b.7z"}, {Name: "gazen_a.7z"}}
	got, err := Pick(a, "")
	if err != nil || got.Name != "gazen_b.7z" {
		t.Fatalf("latest: %v %v", got, err)
	}
	if got, err = Pick(a, "gazen_a.7z"); err != nil || got.Name != "gazen_a.7z" {
		t.Fatalf("by name: %v %v", got, err)
	}
	if _, err = Pick(a, "nope.7z"); err == nil {
		t.Fatal("unknown name must fail")
	}
	if _, err = Pick(nil, ""); err == nil {
		t.Fatal("empty bucket must fail")
	}
}

func TestMatchesAndSort(t *testing.T) {
	if !matches("gazen_2026.7z", "gazen_") || matches("other_2026.7z", "gazen_") || matches("gazen_x.dump", "gazen_") {
		t.Fatal("prefix/suffix filter is wrong")
	}
	now := time.Now()
	a := []Archive{{Name: "old", Uploaded: now.Add(-time.Hour)}, {Name: "new", Uploaded: now}}
	sortNewest(a)
	if a[0].Name != "new" {
		t.Fatal("not newest first")
	}
}

func TestVerify(t *testing.T) {
	const sha = "da39a3ee5e6b4b0d3255bfef95601890afd80709" // sha1 of empty input
	if err := verify(0, sha, Archive{Name: "a", Size: 0, SHA1: sha}); err != nil {
		t.Fatal(err)
	}
	if verify(5, sha, Archive{Name: "a", Size: 6, SHA1: sha}) == nil {
		t.Fatal("size mismatch accepted")
	}
	if verify(0, sha, Archive{Name: "a", SHA1: "0000000000000000000000000000000000000000"}) == nil {
		t.Fatal("sha1 mismatch accepted")
	}
	if err := verify(7, sha, Archive{Name: "a", Size: 7, SHA1: "none"}); err != nil {
		t.Fatalf("large file (no sha1) must pass on size: %v", err)
	}
}

func TestFindDumps(t *testing.T) {
	dir := t.TempDir()
	old := filepath.Join(dir, "opt", "backups", "old.dump")
	fresh := filepath.Join(dir, "opt", "backups", "fresh.dump")
	for _, p := range []string{old, fresh, filepath.Join(dir, "x.txt")} {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	past := time.Now().Add(-time.Hour)
	if err := os.Chtimes(old, past, past); err != nil {
		t.Fatal(err)
	}
	got, err := FindDumps(dir)
	if err != nil || len(got) != 2 || got[0] != fresh {
		t.Fatalf("got %v %v", got, err)
	}
}

// Round trip with the same 7z flags the archiver uses (-spf2, encrypted
// names): proves Extract puts everything under the target dir, not on the
// live system, and that a wrong password fails without echoing the password.
func TestExtractRoundTrip(t *testing.T) {
	bin, err := exec.LookPath("7z")
	if err != nil {
		t.Skip("7z not installed")
	}
	src := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "app.yaml"), []byte("secret: yes\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(t.TempDir(), "gazen_test.7z")
	const pwd = "p@ss!#word"
	out, err := exec.Command(bin, "a", "-spf2", "-p"+pwd, "-mhe=on", "-mx=5", "-mmt=1", "-y", archive, src).CombinedOutput()
	if err != nil {
		t.Fatalf("7z a: %v %s", err, out)
	}

	r := &Restorer{lg: zap.NewNop(), cfg: config.Config{Backups: config.Backups{Pwd: pwd, Path7z: bin}}}
	dst := t.TempDir()
	if err := r.Extract(archive, dst); err != nil {
		t.Fatal(err)
	}
	// stored with an absolute path, but unpacked INSIDE dst
	got := filepath.Join(dst, src, "app.yaml")
	if b, err := os.ReadFile(got); err != nil || string(b) != "secret: yes\n" {
		t.Fatalf("file not under the target dir: %v %q", err, b)
	}

	bad := &Restorer{lg: zap.NewNop(), cfg: config.Config{Backups: config.Backups{Pwd: "wrong-pwd", Path7z: bin}}}
	err = bad.Extract(archive, t.TempDir())
	if err == nil {
		t.Fatal("wrong password accepted")
	}
	if strings.Contains(err.Error(), "wrong-pwd") {
		t.Fatal("password leaked into the error")
	}
}

func TestLatestIgnoresReuploadedOldArchive(t *testing.T) {
	// the real list of a gate: an archive of 11-11 re-uploaded on 11-21 after
	// the bucket retention dropped it, one hour after the genuine newest one
	mk := func(name string, up time.Time) Archive {
		return Archive{Name: name, Uploaded: up, Created: createdFromName(name)}
	}
	day := func(d, h int) time.Time { return time.Date(2026, 11, d, h, 24, 0, 0, time.UTC) }
	a := []Archive{
		mk("gazen_2026-11-11_05-24_c08904a1.7z", day(21, 6)), // re-uploaded old
		mk("gazen_2026-11-21_05-24_baf1d68e.7z", day(21, 5)), // the real newest
		mk("gazen_2026-11-20_17-24_0534be98.7z", day(20, 17)),
	}
	sortNewest(a)
	got, err := Pick(a, "")
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "gazen_2026-11-21_05-24_baf1d68e.7z" {
		t.Errorf("latest = %s, want the 11-21 archive, not the re-uploaded 11-11 one", got.Name)
	}
}

func TestCreatedFromName(t *testing.T) {
	if got := createdFromName("gazen_2026-11-21_05-24_baf1d68e.7z"); got.Format("2006-01-02 15:04") != "2026-11-21 05:24" {
		t.Errorf("parsed %v", got)
	}
	if got := createdFromName("gazen_garbage.7z"); !got.IsZero() {
		t.Errorf("a name without a timestamp must give zero, got %v", got)
	}
}
