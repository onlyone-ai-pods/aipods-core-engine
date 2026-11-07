# Build Stage
FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/server ./cmd/server

# CMMI metrics from the git history (SPEC-CORE-68). If .git is missing the
# report is skipped and /metrics leaves the CMMI gauges out.
RUN apk add --no-cache git && git config --global --add safe.directory /app && mkdir -p /out \
    && (go run ./cmd/cmmi-report -repo . -out /out/cmmi.json || echo "cmmi-report skipped")

# Production Stage (Scratch Distroless)
FROM alpine:3.19

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /root/

COPY --from=builder /app/server .
COPY --from=builder /out/ ./

EXPOSE 8080

CMD ["./server"]
