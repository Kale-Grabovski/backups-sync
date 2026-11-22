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
	ssh vpn@vpning sudo supervisorctl restart all && \
	rsync -av bin/bsync vpn@vpnnlg:~/bsync/ && \
	ssh vpn@vpnnlg sudo supervisorctl restart all

upload-ing:
	make build && \
	rsync -av bin/bsync vpn@vpning:~/bsync/ && \
	ssh vpn@vpning sudo supervisorctl restart all

upload-nlg:
	make build && \
	rsync -av bin/bsync vpn@vpnnlg:~/bsync/ && \
	ssh vpn@vpnnlg sudo supervisorctl restart all

# gate only (the gate part of `upload`)
upload-gate:
	make build && \
	rsync -av bin/bsync vpn@vpngate:~/bsync/ && \
	ssh vpn@vpngate sudo supervisorctl restart {bsync,bbackup,bdb}

# A NEW gate host: only the binary, supervisor has no programs there yet (they
# come from the archive), so no restart. Then `bsync restore` runs on the host.
seed-gate:
	make build && \
	ssh vpn@vpngate mkdir -p ~/bsync && \
	rsync -av bin/bsync vpn@vpngate:~/bsync/

# the same for a NEW ingress or monitoring host
seed-ing:
	make build && \
	ssh vpn@vpning mkdir -p ~/bsync && \
	rsync -av bin/bsync vpn@vpning:~/bsync/

seed-nlg:
	make build && \
	ssh vpn@vpnnlg mkdir -p ~/bsync && \
	rsync -av bin/bsync vpn@vpnnlg:~/bsync/

mac:
	go build -trimpath -ldflags="-s -w" -o bin/bsync-mac
