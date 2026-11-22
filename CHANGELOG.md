# Changelog

Newest first. One section per day, plain English, what changed and why.

## 2026-11-21

### Fixed
- `bsync restore --latest` picked the wrong archive. The list and the newest
  pick went by the Backblaze upload time, and the sync re-uploads a local
  archive that the bucket retention (`backblaze.retention_days`, by upload time)
  already deleted while the local copy (`backups.retention_days`) is still
  there. With 10 days in the bucket and 14 on disk, an archive of day D comes
  back at D+10 with a fresh upload time and sorts above the genuine newest one.
  Real list of a gate: `gazen_2026-11-11_05-24` uploaded 11-21 06:24 sat above
  `gazen_2026-11-21_05-24`. Newest and `--latest` now go by the timestamp in the
  archive name, `--list` shows made and uploaded side by side. The cause is
  fixed in the sync, see the next entries.
- The sync no longer uploads a local file that is already past
  `backblaze.retention_days` (by the timestamp in its name, mtime as a
  fallback). It used to put it back every day after the cleanup deleted it, as
  long as the local copy lived (`backups.retention_days` is longer).
- The bucket cleanup goes by the timestamp in the name, not by the upload time,
  and only deletes files that start with a prefix of this config
  (`backups.prefix`, `db.prefix`). The bucket is shared by gate, ing and nlg,
  and a host used to delete everything older than ITS retention, other hosts'
  archives included. With no prefix configured the old behaviour stays.
- The cleanup never deletes the 3 newest files of a prefix, however old: a host
  that stopped making archives must not lose its last backups to the retention.
- `retention_days` <= 0 now switches the retention off (cleanup and the skip of
  old uploads). It used to mean "delete everything older than now", that is
  the whole bucket, if the value was simply missing from the config.
- The sync walks the bucket page by page. One call returns at most 1000 names,
  the rest was ignored: files beyond it were uploaded again and never cleaned.

### Changed
- `bsync restore --db --yes` now runs `pg_restore --single-transaction`: all or
  nothing, a failure half way rolls the whole restore back instead of leaving a
  database of half old and half new objects.

### Added
- `make upload-gate` (the gate part of `upload`) and `make seed-gate` (only the
  binary on a NEW gate host, no supervisor restart: its programs do not exist
  there until the archive is restored).
- `make seed-ing` and `make seed-nlg`: the same binary-only seeding for a NEW
  ingress or monitoring host.

## 2026-11-20

### Added
- `bsync restore`: list the archives of a Backblaze bucket (`--list`), download
  and unpack one into an empty directory (`--latest` or `--file`, size and sha1
  are checked, the 7z password is hidden in errors) and feed a pg dump to a
  postgres container (`--db`, a dry run `pg_restore --list` unless `--yes`).
  It never overwrites the live system and, unlike `sync`, never creates a
  missing bucket. Until now backups could only be made, never brought back.
