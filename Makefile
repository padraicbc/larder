# Development: run `make api` and `make web` in two terminals, open http://localhost:5173
# Everything else builds on `web-build`, which embeds the web app into the Go binary.
WEB_DIST := backend/internal/webui/dist

api:
	cd backend && go run ./cmd/server

web:
	cd web && npm run dev

test:
	cd backend && go test ./...
	cd backend/mobile && go test ./...
	cd web && npm test

check:
	cd backend && go vet ./...
	cd backend/mobile && go vet ./...
	cd web && npm run check

web-build:
	cd web && npm ci && npm run build
	find $(WEB_DIST) -mindepth 1 ! -name README.txt -exec rm -rf {} +
	cp -r web/build/. $(WEB_DIST)/

# One self-contained binary (web app embedded) at backend/bin/larder
build: web-build
	cd backend && CGO_ENABLED=0 go build -ldflags="-s -w" -o bin/larder ./cmd/server

run: build
	cd backend && ./bin/larder

docker:
	docker build -t larder .

# Phone build: see docs/ANDROID.md
android: web-build
	./scripts/build-android.sh

# Try the phone build on your computer
preview: web-build
	cd backend/mobile && go run ./cmd/preview

.PHONY: api web test check web-build build run docker android preview
