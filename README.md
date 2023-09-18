## Backup postgres dumps to dropbox

`sudo apt install bzip2`

Create a crontabs to create and clear backups:

`su postgres && cd ~ && mkdir -p backups`

`crontab -e`

Add crontabs and don't forget to change dbname, password and dropboxfolder:

```
29 20 * * * pg_dump dbname --use-set-session-authorization | bzip2 | openssl enc -aes-256-cbc -pbkdf2 -k secretPassword > /var/lib/postgresql/backups/backup-$(date +\%Y-xx-\%d).sql.bz2
46 * * * * find /var/lib/postgresql/backups -mtime +7 -exec rm {} \;
44 22 * * * sh /var/lib/postgresql/uploader.sh dropboxfolder
```

To decrypt the backup run:

`openssl enc -d -aes-256-cbc -k secretPassword -pbkdf2 -in backup-2023-xx-04.sql.bz2 | bzip2 -d > dump.sql`

We user `xx` instead on the month because we plan to store backups for 1 month only, so the file will be replaced
after 30 days.

Clone this tool on your local machine:

https://github.com/dropbox/dbxcli

Change appKey and appSecret under `cmd/root.go`. You can find the keys at your app
https://www.dropbox.com/developers/apps/create .

Then build to tool by running `CGO_ENABLED=0 go build`.
Now you can upload the binary and `uploader.sh` to your host:

`scp uploader.sh dbxcli/dbxcli root@host:/tmp`

Change the owner of these files to postgres:

```
ssh root@host
cd /var/lib/postgresql
sudo chown postgres:postgres /tmp/uploader.sh /tmp/dbxcli
sudo su postgres
mv /tmp/uploader.sh . && mv /tmp/dbxcli .
```

Run uploader.sh from the postgres user for the first time to get auth key from Dropbox app:

`sh /var/lib/postgresql/uploader.sh dropboxfolder`

It asks for the token which you could find at 'App panel', just click 'Generate'.
Important! Set expiration for the token as 'No expiration'.

If you need to reinstall access token for some reason you should remove this shit before:

`rm ~/.config/dbxcli/auth.json`
