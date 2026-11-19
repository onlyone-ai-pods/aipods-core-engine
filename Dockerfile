# Build Stage
FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN apk add --no-cache git && git config --global --add safe.directory /app

# Build version = VERSION file + short commit, shown in /livez, /readyz,
# /healthz and aipods_build_info to tell blue from green (SPEC-CORE-73.R6).
RUN BUILD_VERSION="$(cat VERSION 2>/dev/null || echo dev)-$(git rev-parse --short HEAD 2>/dev/null || echo nogit)" \
    && CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s -X main.buildVersion=${BUILD_VERSION}" -o /app/server ./cmd/server

# CMMI metrics from the git history (SPEC-CORE-68). If .git is missing the
# report is skipped and /metrics leaves the CMMI gauges out.
RUN mkdir -p /out \
    && (go run ./cmd/cmmi-report -repo . -out /out/cmmi.json || echo "cmmi-report skipped")

# Production Stage (Scratch Distroless)
FROM alpine:3.19

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /root/

COPY --from=builder /app/server .
COPY --from=builder /out/ ./

EXPOSE 8080

# The binary is PID 1 (exec form) and drains on SIGTERM (SPEC-CORE-73.R1, R17).
# Give the container a stop grace period longer than DRAIN_DELAY + SHUTDOWN_TIMEOUT.
STOPSIGNAL SIGTERM

CMD ["./server"]
