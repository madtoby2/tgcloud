.PHONY: build run clean

build:
	GOPROXY=https://goproxy.cn,direct go build -ldflags="-s -w" -o tgcloud.exe ./cmd/tgcloud/

run: build
	./tgcloud.exe --api-id $(API_ID) --api-hash $(API_HASH) --addr :8080

clean:
	rm -f tgcloud.exe tgcloud.db tgcloud.db-wal tgcloud.db-shm
