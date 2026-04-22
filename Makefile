all: build

deps:
	go install mvdan.cc/garble@latest

build-encrypt:
	make deps
	GOOS=linux GOARCH=amd64 garble -tiny -literals build -o bin/bsync .

build-mac:
	go build -trimpath -ldflags="-s -w" -o bin/bsync-mac

build:
	GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o bin/bsync

upload:
	make build && rsync -av -e "ssh -p 14888" bin/bsync vpn@vpngate:~/bsync/ && \
	ssh -p 14888 vpn@vpngate sudo supervisorctl restart {bsync,bbackup,bdb}

mac:
	go build -trimpath -ldflags="-s -w" -o bin/bsync-mac
