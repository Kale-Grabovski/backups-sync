package sync

import (
	"testing"
	"time"
)

var now = time.Date(2026, 11, 21, 12, 0, 0, 0, time.Local)

func rf(name string, up time.Time) remoteFile {
	return remoteFile{Name: name, ID: "id-" + name, Uploaded: up}
}

func names(fs []remoteFile) []string {
	var out []string
	for _, f := range fs {
		out = append(out, f.Name)
	}
	return out
}

func TestOldLocalFileIsNotUploadedAgain(t *testing.T) {
	// the 11-day-old archive that used to come back to the bucket every day
	if !expired("gazen_2026-11-10_05-24_x.7z", now, 10, now) {
		t.Error("an archive past the retention must be skipped by the upload")
	}
	if expired("gazen_2026-11-15_05-24_x.7z", now, 10, now) {
		t.Error("a fresh archive must be uploaded")
	}
}

func TestRetentionByNameNotByUploadTime(t *testing.T) {
	// made 11 days ago, uploaded just now: old by its name
	files := []remoteFile{
		rf("gazen_2026-11-10_05-24_a.7z", now),
		rf("gazen_2026-11-21_05-24_b.7z", now),
		rf("gazen_2026-11-20_17-24_c.7z", now),
		rf("gazen_2026-11-20_05-24_d.7z", now),
		rf("gazen_2026-11-19_17-24_e.7z", now),
	}
	got := names(selectExpired(files, []string{"gazen_"}, 10, now))
	if len(got) != 1 || got[0] != "gazen_2026-11-10_05-24_a.7z" {
		t.Errorf("got %v", got)
	}
}

func TestCleanupTouchesOnlyOwnPrefix(t *testing.T) {
	files := []remoteFile{
		rf("png_2026-10-01_05-24_x.7z", now), rf("png_2026-10-02_05-24_x.7z", now),
		rf("png_2026-10-03_05-24_x.7z", now), rf("png_2026-10-04_05-24_x.7z", now),
		rf("gazen_2026-10-01_05-24_x.7z", now), rf("gazen_2026-10-02_05-24_x.7z", now),
		rf("gazen_2026-10-03_05-24_x.7z", now), rf("gazen_2026-10-04_05-24_x.7z", now),
	}
	for _, f := range selectExpired(files, []string{"png_"}, 10, now) {
		if f.Name[:4] != "png_" {
			t.Errorf("a host with prefix png_ would delete %s", f.Name)
		}
	}
	// no prefixes known: the old behaviour, one mixed group, the 3 newest of
	// all 8 stay and the other 5 go
	if got := selectExpired(files, nil, 10, now); len(got) != 5 {
		t.Errorf("legacy mode: want 5 expired, got %v", names(got))
	}
}

func TestNewestAreNeverDeleted(t *testing.T) {
	// the host stopped making archives, everything is past the retention
	files := []remoteFile{
		rf("gazen_2026-09-01_05-24_a.7z", now), rf("gazen_2026-09-02_05-24_b.7z", now),
		rf("gazen_2026-09-03_05-24_c.7z", now), rf("gazen_2026-09-04_05-24_d.7z", now),
		rf("gazen_2026-09-05_05-24_e.7z", now),
	}
	got := names(selectExpired(files, []string{"gazen_"}, 10, now))
	want := []string{"gazen_2026-09-01_05-24_a.7z", "gazen_2026-09-02_05-24_b.7z"}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("got %v, want %v (the 3 newest stay)", got, want)
	}
}

func TestZeroRetentionDeletesNothing(t *testing.T) {
	files := []remoteFile{rf("gazen_2020-01-01_05-24_a.7z", now)}
	if got := selectExpired(files, []string{"gazen_"}, 0, now); len(got) != 0 {
		t.Errorf("retention 0 must switch the cleanup off, got %v", names(got))
	}
	if expired("gazen_2020-01-01_05-24_a.7z", now, 0, now) {
		t.Error("retention 0 must not skip uploads either")
	}
}

func TestNameWithoutTimestampFallsBack(t *testing.T) {
	old := now.AddDate(0, 0, -30)
	if !expired("manual-copy.7z", old, 10, now) {
		t.Error("no stamp in the name: the fallback time decides")
	}
}
