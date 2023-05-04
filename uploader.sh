#!/bin/bash
FILES="/var/lib/postgresql/backups/*.bz2"
for f in $FILES
do
  fn="$(basename -- $f)"
  /var/lib/postgresql/dbxcli put $f $1/`basename -- $fn`
done
