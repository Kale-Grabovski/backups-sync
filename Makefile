all: build

build:
	GOOS=linux GOARCH=amd64 go build -o bsync

upload:
	make build && rsync -av bsync vpn@vpnnl1:~/bsync/ && \
	ssh vpn@vpnnl1 sudo supervisorctl restart bsync && \
	rm bsync

