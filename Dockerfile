# STAGE 1: Builder (Compilación Estática Ligera en Go 1.22 Alpine)
FROM golang:1.22-alpine AS builder

# Instalar certificados CA mínimos
RUN apk add --no-cache ca-certificates git

WORKDIR /app

# Aprovechar caché de capas de Docker para dependencias
COPY go.mod go.sum ./
RUN go mod download

# Copiar código fuente
COPY . .

# Compilar binario estático optimizado (-w -s elimina símbolos de depuración para reducir tamaño)
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags="-w -s" -o server ./cmd/server/main.go

# STAGE 2: Imagen Final Ultraligera (Alpine 3.19 Mínimo < 20MB)
FROM alpine:3.19

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /root/

# Copiar solo el binario compilado estático desde el builder
COPY --from=builder /app/server .

# Puerto expuesto por el servidor Go
EXPOSE 8080

# Comando de ejecución
ENTRYPOINT ["./server"]
