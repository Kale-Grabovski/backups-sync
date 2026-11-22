// Package archivename reads the timestamp the archiver and the db dumper put
// into every file name: <prefix>2006-01-02_15-04[-05]_<hash>.<ext>
package archivename

import (
	"regexp"
	"time"
)

const layout = "2006-01-02_15-04"

var stamp = regexp.MustCompile(`\d{4}-\d{2}-\d{2}_\d{2}-\d{2}`)

// Created is when the file was made, taken from its name, in the local time of
// the host that wrote it (the name is written with time.Now()). ok is false if
// the name carries no timestamp.
//
// The upload time of a bucket file is not a substitute: a file the sync
// uploads again gets a fresh one.
func Created(name string) (t time.Time, ok bool) {
	m := stamp.FindString(name)
	if m == "" {
		return time.Time{}, false
	}
	t, err := time.ParseInLocation(layout, m, time.Local)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}
