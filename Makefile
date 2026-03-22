all: build

build:
	GOOS=linux GOARCH=amd64 go build -o bin/bsync

mac:
	go build -o bin/bsync-mac

upload:
	make build && rsync -av bin/bsync vpn@vpngate:~/bsync/ && \
	ssh vpn@vpngate sudo supervisorctl restart bsync && \
	ssh vpn@vpngate sudo supervisorctl restart bbackup && \
	ssh vpn@vpngate sudo supervisorctl restart bdb
