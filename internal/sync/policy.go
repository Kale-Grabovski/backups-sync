package sync

import (
	"sort"
	"strings"
	"time"

	"backupsync/internal/archivename"
)

// keepNewest is how many of the newest files per prefix the cleanup never
// touches, however old they are. If a host stops making archives, the
// retention must not eat the last backups one by one.
const keepNewest = 3

type remoteFile struct {
	Name     string
	ID       string
	Uploaded time.Time
}

// when is the moment a file was made: the timestamp in its name, the fallback
// (upload time or mtime) only for a name without one.
func when(name string, fallback time.Time) time.Time {
	if t, ok := archivename.Created(name); ok {
		return t
	}
	return fallback
}

// expired: the file is older than the retention. days <= 0 turns the retention
// off, like the db cleanup does: an unset value must never mean "delete all".
func expired(name string, fallback time.Time, days int, now time.Time) bool {
	if days <= 0 {
		return false
	}
	return when(name, fallback).Before(now.AddDate(0, 0, -days))
}

// scopeOf is the prefix that owns name. The bucket is shared by several hosts
// (gazen_, png_, mon_ ...), a host cleans only its own files. With no prefixes
// known the old behaviour stays: everything belongs to everyone, group "".
func scopeOf(name string, prefixes []string) (group string, ok bool) {
	if len(prefixes) == 0 {
		return "", true
	}
	for _, p := range prefixes {
		if p != "" && strings.HasPrefix(name, p) {
			return p, true
		}
	}
	return "", false
}

// selectExpired picks what the cleanup may delete: own files, past the
// retention, never the keepNewest newest of a prefix.
func selectExpired(files []remoteFile, prefixes []string, days int, now time.Time) []remoteFile {
	if days <= 0 {
		return nil
	}
	groups := map[string][]remoteFile{}
	for _, f := range files {
		if g, ok := scopeOf(f.Name, prefixes); ok {
			groups[g] = append(groups[g], f)
		}
	}

	var out []remoteFile
	for _, g := range groups {
		sort.SliceStable(g, func(i, j int) bool {
			return when(g[i].Name, g[i].Uploaded).After(when(g[j].Name, g[j].Uploaded))
		})
		for i, f := range g {
			if i < keepNewest {
				continue
			}
			if expired(f.Name, f.Uploaded, days, now) {
				out = append(out, f)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
