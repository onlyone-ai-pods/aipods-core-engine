# 📋 Checklist de Requisitos Previos, Arquitectura Multi-Repo y Onboarding

**Proyecto:** AI Pods Enterprise SaaS Platform (Universal Multi-Tenant API & ERP Suite)  
**Organización en GitHub:** `https://github.com/onlyone-ai-pods`  
**Estado:** COMPLETO Y LISTO PARA SPRINT 1  

---

## 🏛️ 1. Arquitectura Multi-Repositorio & Ecosistema

El proyecto se gestiona mediante 4 repositorios oficiales dentro de la Organización de GitHub **`onlyone-ai-pods`**:

```text
github.com/onlyone-ai-pods/
├── aipods-docs/                  [DOCS] Specs SDD, Backlog, SDD.md, OpenAPI yaml
├── aipods-core-engine/           [BACKEND] Go 1.22 API Server, Smart Router, RAG, DBs
├── aipods-frontend-customer/     [FRONTEND] React 18 / Vite Portal Público & Sandbox Interactivo
└── aipods-frontend-admin/        [ADMIN] React 18 / Vite Portal Interno & Senior Review Hub
```

---

## 🛠️ 2. Flujo de Clonado y Onboarding para Desarrolladores Internos

Cualquier desarrollador interno del equipo ejecuta 3 pasos sencillos para obtener y levantar todo el proyecto localmente:

```bash
# 1. Crear el directorio workspace en su máquina local
mkdir -p ~/workspace/onlyone-ai-pods && cd ~/workspace/onlyone-ai-pods

# 2. Clonar los 4 repositorios con GitHub CLI (gh)
gh repo clone onlyone-ai-pods/aipods-docs
gh repo clone onlyone-ai-pods/aipods-core-engine
gh repo clone onlyone-ai-pods/aipods-frontend-customer
gh repo clone onlyone-ai-pods/aipods-frontend-admin

# 3. Levantar la pila de infraestructura local
cd aipods-core-engine
docker compose -f docker-compose.dev.yml up -d
```

---

## 🌿 3. Convención Estándar de Nombres de Ramas (Branch Naming)

Toda rama creada DEBE seguir la estructura: `<tipo>/<id-o-modulo>-<descripcion-kebab-case>`

* `spec/`     : Para PRs de especificaciones en `aipods-docs` (ej: `spec/01-smart-router`)
* `feat/`     : Para PRs de características en Go/React (ej: `feat/01-go-router-impl`)
* `fix/`      : Para corrección de errores (ej: `fix/02-qdrant-timeout`)
* `refactor/` : Para reestructuración de código (ej: `refactor/04-semantic-cache`)
* `docs/`     : Para actualización de documentación (ej: `docs/update-onboarding`)
* `ci/`       : Para GitHub Actions y linters (ej: `ci/setup-gosec`)

---

## 🔑 4. Cuentas en Plataformas & API Keys Necesarias

| Servicio | Propósito | Permiso | Credenciales |
| :--- | :--- | :--- | :--- |
| **Anthropic Console** | LLM Principal RAG (Claude 3.5 Sonnet) y Smart Router. | Developer Account | `ANTHROPIC_API_KEY` |
| **OpenAI Platform** | Embeddings (`text-embedding-3-small`) y respaldo LLM. | Developer Account | `OPENAI_API_KEY` |
| **Qdrant Cloud / Local** | Base de Datos Vectorial para embeddings RAG. | Cluster / Local | `QDRANT_HOST`, `QDRANT_PORT` |
| **Redis Enterprise** | Caché Semántico y Rate Limiting por tenant. | Instancia / Local | `REDIS_HOST`, `REDIS_PORT` |
| **NATS JetStream** | Colas asíncronas para ingesta de balances extensos. | Instancia / Local | `NATS_URL` |
| **AWS S3 / Managed** | Almacenamiento de documentos inmutables y PostgreSQL. | IAM Credentials | `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY` |

---

## 🧪 5. Infraestructura Local (`docker-compose.dev.yml`)

El archivo `docker-compose.dev.yml` en `aipods-core-engine` levanta el stack local con un solo comando:

```yaml
version: '3.8'
services:
  postgres:
    image: postgres:16-alpine
    container_name: aipods_postgres
    environment:
      POSTGRES_DB: aipods_db
      POSTGRES_USER: aipods_user
      POSTGRES_PASSWORD: aipods_password
    ports:
      - "5432:5432"

  qdrant:
    image: qdrant/qdrant:latest
    container_name: aipods_qdrant
    ports:
      - "6333:6333"

  redis:
    image: redis:7-alpine
    container_name: aipods_redis
    ports:
      - "6379:6379"

  nats:
    image: nats:2.10-alpine
    container_name: aipods_nats
    ports:
      - "4222:4222"
```
