# Changelog

Newest first. One section per day, plain English, what changed and why.

## 2026-11-20

### Added
- `bsync restore`: list the archives of a Backblaze bucket (`--list`), download
  and unpack one into an empty directory (`--latest` or `--file`, size and sha1
  are checked, the 7z password is hidden in errors) and feed a pg dump to a
  postgres container (`--db`, a dry run `pg_restore --list` unless `--yes`).
  It never overwrites the live system and, unlike `sync`, never creates a
  missing bucket. Until now backups could only be made, never brought back.
