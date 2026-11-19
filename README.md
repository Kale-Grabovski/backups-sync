## Backup postgres dumps to dropbox

```
cp app.example.yaml app.yaml
```

## Restore

Needs `backblaze` (id, key, bucket), `backups.pwd`, `backups.path_7z` and, for
`--db`, the `db` section. Keep the password OUTSIDE the backup it protects.

    bsync restore --list
    bsync restore --latest --to /root/restore    # or --file NAME
    bsync restore --db DUMP                      # dry run: pg_restore --list
    bsync restore --db DUMP --yes                # really restores

Archives unpack into an EMPTY directory, never over the live system.
