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
	make build && \
	rsync -av bin/bsync vpn@vpngate:~/bsync/ && \
	ssh vpn@vpngate sudo supervisorctl restart {bsync,bbackup,bdb} && \
	rsync -av bin/bsync vpn@vpning:~/bsync/ && \
	ssh vpn@vpning sudo supervisorctl restart all \
	rsync -av bin/bsync vpn@vpnnlg:~/bsync/ && \
	ssh vpn@vpnnlg sudo supervisorctl restart all

mac:
	go build -trimpath -ldflags="-s -w" -o bin/bsync-mac
