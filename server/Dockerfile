# Server image (D-70). Build context is the repo root, so all paths are prefixed with server/.
# Two stages: a Go builder produces two static CGO-free binaries (the API and the seed tool),
# and a small alpine runtime carries them plus the sample dataset. alpine (not distroless) so
# the compose healthcheck can probe /api/health with busybox wget without adding Go code.

# --- build ---------------------------------------------------------------------------------
FROM golang:1.23-alpine AS build
WORKDIR /src

# Module cache layer: only re-runs when go.mod/go.sum change.
COPY server/go.mod server/go.sum ./
RUN go mod download

# Source layer.
COPY server/ ./

# Pure-Go build (modernc.org/sqlite needs no C toolchain — CLAUDE.md stack rule).
RUN CGO_ENABLED=0 go build -trimpath -o /out/api ./cmd/api \
 && CGO_ENABLED=0 go build -trimpath -o /out/seed ./cmd/seed

# --- runtime -------------------------------------------------------------------------------
FROM alpine:3.20
WORKDIR /app

# Non-root user; /data holds the SQLite file (mounted as a named volume by compose).
RUN addgroup -S app && adduser -S -G app app \
 && mkdir -p /data

COPY --from=build /out/api /out/seed /app/
COPY server/data/sample.json /app/data/sample.json
RUN chown -R app:app /app /data

# Container-only paths (do NOT enter .env.example — D-70). SQLite lives on the /data volume;
# the sample dataset ships in the image. JWT_SECRET/APP_ENV/etc. come from compose or -e.
ENV SQLITE_PATH=/data/twitter.db \
    SAMPLE_DATA_PATH=/app/data/sample.json

USER app
EXPOSE 3000
CMD ["/app/api"]
