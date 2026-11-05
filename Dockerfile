# Self-contained image: web app built and embedded into one static Go binary.
FROM node:22-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.24-alpine AS server
WORKDIR /src/backend
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
COPY --from=web /src/web/build/ ./internal/webui/dist/
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /larder ./cmd/server && mkdir /data

# distroless: no shell, runs as non-root, includes CA certificates for model APIs.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=server /larder /larder
COPY --from=server --chown=65532:65532 /data /data
ENV NUTRI_DATA=/data/larder.json NUTRI_ADDR=:8080
VOLUME /data
EXPOSE 8080
ENTRYPOINT ["/larder"]
