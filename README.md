# ⚙️ AI Pods Enterprise (SaaS) - Backend Engine (`aipods-core-engine`)

Este repositorio contiene el motor backend en **Golang 1.22+**, microservicios de orquestación, Smart Router, evaluador multi-tenant y la pila de desarrollo local de **AI Pods Enterprise SaaS Platform**.

## 🛠️ Tecnologías del Backend

* **Golang 1.22+:** Runtime principal y servidor HTTP Gin.
* **Smart Router:** Clasificador de intenciones entre AI Pods.
* **Aislamiento Multi-Tenant:** Inyección de `tenant_id` obligatorio en contexto de Go.
* **PostgreSQL 16 Enterprise:** Base de datos relacional y particionada por tenant.
* **Qdrant Vector DB:** Motor de búsqueda de embeddings RAG.
* **Redis Active-Active & NATS JetStream:** Caché semántico y bus de eventos asíncronos.

---

## 💻 Guía de Desarrollo Local

```bash
# 1. Levantar la pila de infraestructura local con Docker
docker compose -f docker-compose.dev.yml up -d

# 2. Descargar dependencias y ejecutar pruebas BDD
go mod tidy
go test -v ./...

# 3. Compilar y ejecutar el servidor en Go
go run cmd/server/main.go
```

---

## 🔒 Licencia

Todos los derechos reservados © 2026 Martin Llanos. Licencia Propietaria.
