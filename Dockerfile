# Build uses Go + the system SQLite library. No npm or Go module downloads.
FROM golang:1.27.1-bookworm AS build
RUN apt-get update && apt-get install -y --no-install-recommends gcc libc6-dev libsqlite3-dev && rm -rf /var/lib/apt/lists/*
WORKDIR /src
COPY . .
RUN CGO_ENABLED=1 go build -trimpath -ldflags='-s -w' -o /bolty ./cmd/bolty

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates libsqlite3-0 python3 python3-reportlab \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd -g 10001 bolty && useradd -u 10001 -g bolty -d /app -s /usr/sbin/nologin bolty \
    && mkdir -p /app/tools /data && chown -R bolty:bolty /app /data
COPY --from=build /bolty /app/bolty
COPY tools/render.py /app/tools/render.py
WORKDIR /app
USER 10001:10001
ENV LISTEN_ADDR=0.0.0.0:8080 DATA_DIR=/data PYTHON_BIN=python3 PDF_RENDERER=/app/tools/render.py
EXPOSE 8080
ENTRYPOINT ["/app/bolty"]
