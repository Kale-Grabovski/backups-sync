package archivename

import "testing"

func TestCreated(t *testing.T) {
	for _, c := range []struct {
		name string
		ok   bool
		want string
	}{
		{"gazen_2026-11-21_05-24_baf1d68e.7z", true, "2026-11-21 05:24"},
		{"png_2026-11-01_17-24_x.7z", true, "2026-11-01 17:24"},
		// a db dump has seconds too, minutes are enough
		{"vyalik_postgres_2026-03-22_18-15-43.dump", true, "2026-03-22 18:15"},
		{"gazen_garbage.7z", false, ""},
		{"gazen_2026-13-45_99-99_x.7z", false, ""},
	} {
		got, ok := Created(c.name)
		if ok != c.ok || (ok && got.Format("2006-01-02 15:04") != c.want) {
			t.Errorf("%s: got %v %v, want %v %q", c.name, got, ok, c.ok, c.want)
		}
	}
}
