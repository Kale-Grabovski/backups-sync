## Backup postgres dumps to dropbox

Create a crontabs to create and clear backups:

`su postgres && cd ~ && mkdir -p backups`

`crontab -e`

Add crontabs and don't forget to change dbname, password and dropbox folder:

```
29 8,20 * * * pg_dump dbname --use-set-session-authorization | bzip2 | openssl  enc -aes-256-cbc -k secretPassword > /var/lib/postgresql/backups/backup-$(date +\%Y-\%m-\%d).sql.bz2
46 * * * * find /var/lib/postgresql/backups -mtime +7 -exec rm {} \;
44 9,21 * * * sh /var/lib/postgresql/uploader.sh dropboxfolder
```

Clone this tool on your local machine:

https://github.com/dropbox/dbxcli

Change appKey and appSecret under `cmd/root.go`. You can find the keys at you app
https://www.dropbox.com/developers/app

Then build to tool by running `go build`.
Now you can upload the binary and `uploader.sh` to your host:

`scp uploder.sh dbxcli/dbxcli root@host:/var/lib/postgresql/`

Change the owner of these files to postgres:

`
ssh root@host
cd /var/lib/postgresql
sudo chown postgres:postgres uploader.sh dbxcli
`

Run uploader.sh from the postgres user for the first time to get auth key from Dropbox app:

`sh /var/lib/postgresql/uploader.sh dropboxfolder`
