FROM node:24-alpine AS web-build
WORKDIR /src/web/app
COPY web/app/package.json web/app/package-lock.json ./
RUN npm ci
COPY web/app/ ./
RUN npm run build

FROM golang:1.27-bookworm AS go-build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web-build /src/web/app/dist ./web/app/dist
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/scriptagent ./cmd/server

FROM debian:bookworm-slim
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates ffmpeg sqlite3 \
    && rm -rf /var/lib/apt/lists/* \
    && useradd --system --uid 10001 --create-home scriptagent \
    && mkdir -p /app/data /app/uploads \
    && chown -R scriptagent:scriptagent /app
WORKDIR /app
COPY --from=go-build /out/scriptagent /app/scriptagent
COPY --from=web-build /src/web/app/dist /app/web/app/dist
USER scriptagent
ENV APP_PORT=8080 DATA_DIR=/app/data UPLOAD_DIR=/app/uploads STATIC_DIR=/app/web/app/dist
EXPOSE 8080
VOLUME ["/app/data", "/app/uploads"]
ENTRYPOINT ["/app/scriptagent"]
