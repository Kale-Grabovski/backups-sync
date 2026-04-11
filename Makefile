all: build

deps:
	go install mvdan.cc/garble@latest

build-encrypt:
	make deps
	GOOS=linux GOARCH=amd64 garble -tiny -literals build -o bin/bsync .

build:
	GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o bin/bsync

upload:
	make build && rsync -av bin/bsync vpn@vpngate:~/bsync/ && \
	ssh vpn@vpngate sudo supervisorctl restart {bsync,bbackup,bdb}

mac:
	go build -trimpath -ldflags="-s -w" -o bin/bsync-mac
