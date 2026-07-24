# 📋 Checklist de Requisitos Previos y Onboarding para Desarrollo

**Proyecto:** AI Pods para Consultoría Odoo (SaaS)  
**Documento:** Guía de Inicio de Codificación & Cuentas de Prueba  
**Estado:** LISTO PARA SPRINT 1  

---

## 1. Cuentas en Plataformas & API Keys Necesarias

Para comenzar a desarrollar y probar las diferentes capas del sistema, el equipo de desarrollo precisa las siguientes cuentas y credenciales:

| Plataforma / Servicio | Propósito en el Proyecto | Tipo de Cuenta / Permiso Requerido | Credenciales a Obtener |
| :--- | :--- | :--- | :--- |
| **Anthropic Console** | LLM Principal para Generación RAG (Claude 3.5 Sonnet) y Smart Router (Claude 3 Haiku). | Developer Account con créditos de API. | `ANTHROPIC_API_KEY` |
| **OpenAI Developer Platform** | Embeddings (`text-embedding-3-small`) y LLM de respaldo (`gpt-4o-mini`). | Developer Account con créditos de API. | `OPENAI_API_KEY` |
| **Qdrant Cloud / Self-Hosted** | Base de Datos Vectorial para almacenamiento de embeddings RAG. | Cluster en Qdrant Cloud (o local en Docker). | `QDRANT_HOST`, `QDRANT_API_KEY` |
| **Redis Enterprise / Cloud** | Caché Semántico y Rate Limiting por tenant. | Instancia Redis Cloud o local Docker. | `REDIS_URL`, `REDIS_PASSWORD` |
| **AWS / Hetzner Cloud** | Buckets S3 para almacenamiento de documentos inmutables y PostgreSQL Managed. | Cuenta AWS IAM con permisos S3 y RDS / Cloud. | `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_REGION` |
| **EvoCRM / WhatsApp Business** | Pruebas de integración de Webhooks y mensajería en el Pod EvoCRM (Épica 2). | Account Sandbox / Developer Access. | `EVOCRM_API_TOKEN`, `EVOCRM_WEBHOOK_SECRET` |
| **Meta for Developers** | Pruebas de Troubleshooting del Pod Social Marketing (Épica 3). | Meta Developer App (Instagram Graph API). | `META_APP_ID`, `META_APP_SECRET` |
| **Amazon SES / Mailtrap** | Envíos de correo transaccionales y notificaciones. | Mailtrap Sandbox para Devs o AWS SES. | `SMTP_HOST`, `SMTP_USER`, `SMTP_PASS` |

---

## 2. Entorno de Desarrollo Local (Toolchain del Desarrollador)

Cada desarrollador debe contar con las siguientes herramientas instaladas en su máquina local:

* **Go 1.22+:** Compilador y runtime de Go (`go version`).
* **Docker & Docker Compose v2:** Para levantar la pila de infraestructura local con un solo comando.
* **`golangci-lint`:** Linter oficial para análisis estático de código en Go.
* **Make / Taskfile:** Gestor de tareas para automatizar builds y tests (`make test`, `make run`).
* **Postman / Bruno / HTTPie:** Para probar los endpoints OpenAPI del API Gateway.

---

## 3. Infraestructura Local con Docker Compose (`docker-compose.yml`)

Para desarrollar sin depender de la nube desde el día 1, el archivo `docker-compose.dev.yml` levantará los siguientes servicios locales:

```yaml
version: '3.8'
services:
  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_DB: aipods_db
      POSTGRES_USER: dev_user
      POSTGRES_PASSWORD: dev_password
    ports:
      - "5432:5432"

  qdrant:
    image: qdrant/qdrant:latest
    ports:
      - "6333:6333"
      - "6334:6334"

  redis:
    image: redis:7.2-alpine
    ports:
      - "6379:6379"

  nats:
    image: nats:latest
    command: ["-js"]
    ports:
      - "4222:4222"
      - "8222:8222"

  odoo_mock:
    image: odoo:16.0
    ports:
      - "8069:8069"
    environment:
      - HOST=postgres
      - USER=dev_user
      - PASSWORD=dev_password
```

---

## 4. Archivo de Variables de Entorno (`.env.example`)

Se debe crear un archivo `.env.example` en la raíz de la aplicación con la siguiente estructura:

```bash
# SERVER CONFIG
PORT=8000
ENVIRONMENT=development
LOG_LEVEL=debug

# DATABASE CONFIG
DATABASE_URL=postgres://dev_user:dev_password@localhost:5432/aipods_db?sslmode=disable

# VECTOR STORE (QDRANT)
QDRANT_HOST=localhost:6333
QDRANT_API_KEY=

# CACHE & QUEUE
REDIS_URL=localhost:6379
NATS_URL=nats://localhost:4222

# AI PROVIDERS
ANTHROPIC_API_KEY=your_anthropic_key_here
OPENAI_API_KEY=your_openai_key_here

# AUTH & SECURITY
JWT_PRIVATE_KEY_PATH=./keys/private.pem
JWT_PUBLIC_KEY_PATH=./keys/public.pem
```

---

## 5. Datasets de Prueba Dorados (Golden Datasets)

Para iniciar el desarrollo del **Sprint 1 (Pod AFIP & Motor RAG Multi-Tenant)** se precisan los siguientes archivos de prueba en `/testdata/`:

1. **PDFs de Normativas AFIP de Prueba:**
   * `Manual_AFIP_Certificados_2026.pdf`
   * `Resolucion_AFIP_Facturacion_Electronica.pdf`
2. **Exportaciones de Balances Odoo de Prueba:**
   * `balance_general_ejemplo_odoo.csv`
   * `estado_resultados_ejemplo_odoo.pdf`
3. **Certificados y Claves Falsas de Prueba:**
   * `test_privada.key` y `test_pedido.csr` para validar el formateador de comandos OpenSSL.
