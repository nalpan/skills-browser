.PHONY: build build-linux build-mac build-windows clean

BINARY = skills-browser
CMD    = .

# ローカルビルド（現在のOS/ARCH向け）
build:
	go build -ldflags="-s -w" -o $(BINARY) $(CMD)

# クロスコンパイル
build-linux:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o $(BINARY)_linux_amd64 $(CMD)

build-mac:
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -ldflags="-s -w" -o $(BINARY)_darwin_arm64 $(CMD)
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o $(BINARY)_darwin_amd64 $(CMD)

build-windows:
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o $(BINARY)_windows_amd64.exe $(CMD)

# Dockerでビルド（Go不要）
docker-build:
	docker build -t skills-browser .
	docker create --name sb_tmp skills-browser
	docker cp sb_tmp:/skills-browser ./$(BINARY)_linux_amd64
	docker rm sb_tmp

clean:
	rm -f $(BINARY) $(BINARY)_*
