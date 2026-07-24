# 📜 SPEC: Roadmap de Ejecución Incremental & Estrategia de Rebanadas Verticales (Vertical Slicing)
**ID:** SPEC-CORE-18  
**Épica Relacionada:** Metodología de Desarrollo, Ejecución Ágil SDD & Control de Hitos  
**Estado:** PROPOSED / SPEC-DRIVEN  

---

## 1. Visión y Objetivos

Esta especificación establece la estrategia de **Ejecución Incremental Guiada por Especificaciones (Vertical Slicing Strategy)**. Garantiza que el equipo de desarrollo progrese paso a paso entregando software funcional de punta a punta en cada Sprint, sin perder el horizonte técnico ni la arquitectura global documentada en las 18 especificaciones.

---

## 2. Metodología de Rebanadas Verticales (Vertical Slicing vs Horizontal Layers)

En lugar de construir la arquitectura por capas horizontales (ej. "hacer todas las tablas SQL primero", "hacer todo el router después"), el proyecto se ejecuta mediante **Rebanadas Verticales delgadas y funcionales (Thin Vertical Slices)**:

```mermaid
graph TD
    subgraph Traditional Horizontal (Evitar)
        Layer1[Mes 1: Solo Bases de Datos] --> Layer2[Mes 2: Solo API Core]
        Layer2 --> Layer3[Mes 3: Solo Frontends]
    end

    subgraph Vertical Slicing SDD (Recomendado)
        Sprint1[Sprint 1: Thin Vertical Slice - Pod AFIP End-to-End]
        Sprint2[Sprint 2: Multi-Tenant & Smart Router + Pod SCM]
        Sprint3[Sprint 3: Geo-Cache Active-Active & Async Ingestion]
        Sprint4[Sprint 4: Dual React Portals & Interactive Sandbox]
    end
```

---

## 3. Plan de Sprints Incremental (4-Sprint Execution Roadmap)

### 📌 Sprint 1: MVP Vertical Funcional (Pod AFIP End-to-End)
* **Alcance Vertical:**
  - Servidor base en **Go 1.22+** con endpoint `/api/v1/chat/completions`.
  - Ingesta de 1 documento PDF de AFIP en **Qdrant** con tag `tenant_id = 'GLOBAL'`.
  - Ejecución del **Pod AFIP** respondiendo el comando OpenSSL para archivos CSR.
  - Verificación de linters `golangci-lint` / `gosec` y pruebas BDD de `01_afip_finance_spec.md`.

### 📌 Sprint 2: Multi-Tenant & Smart Router (Pods SCM & EvoCRM)
* **Alcance Vertical:**
  - Aislamiento estricto por metadatos `tenant_id` en PostgreSQL y Qdrant.
  - Clasificador de intenciones **Smart Router** derivando entre Pod AFIP, Pod EvoCRM y Pod SCM.
  - Implementación del protocolo mandatorio **Dry-Run (`dry_run = true`)**.

### 📌 Sprint 3: Geo-Caché Active-Active & Ingesta Asíncrona (NATS & Purga)
* **Alcance Vertical:**
  - Caché semántico en **Redis Active-Active** con purga reactiva ante feedback 👎.
  - Colas **NATS JetStream** para ingesta asíncrona de balances extensos en PDF/CSV.
  - Sistema de telemetría **OpenTelemetry** e inmutabilidad de `audit_logs`.

### 📌 Sprint 4: Portales Web Aislados & Sandbox Interactivo (Growth)
* **Alcance Vertical:**
  - **Portal Público de Clientes** (`app.aipods.com`) en React 18 / Vite con Sandbox interactivo sin login ("Sube tu PDF y prueba").
  - **Portal de Administración** (`admin-internal.aipods.com`) con el *Senior Consultant Review Hub*.
  - Despliegue CI/CD en Kubernetes mediante Helm Charts y Terraform IaC.

---

## 4. Regla de Oro de Ejecución SDD & Checklist del Horizonte

Para mantener la alineación total entre los desarrolladores (humanos e IAs) y la arquitectura documentada:

> 💡 **REGLA DE ORO SDD:** *"Ningún desarrollador o asistente de IA escribe una sola línea de código en el proyecto si no está respaldada por su correspondiente `.spec.md` en el directorio `/specs/`."*

### Checklist del Horizonte de Sprint (Sprint Horizon Checklist):
Al inicio de cada tarea o commit, el equipo verifica:
* `[ ]` **Spec Traceability:** ¿La tarea está trazada con su historia en `docs/BACKLOG.md` y su archivo `.spec.md`?
* `[ ]` **Multi-Tenant Security:** ¿Las queries incluyen `WHERE (tenant_id == X OR tenant_id == 'GLOBAL') AND status == 'ACTIVE'`?
* `[ ]` **Clean Code & Security:** ¿Pasa `golangci-lint` y `gosec` sin errores?
* `[ ]` **Commit Standard:** ¿El commit sigue la convención oficial de Odoo (`[ADD]`, `[IMP]`, `[FIX]`)?

---

## 5. Escenario BDD de Validación del Sprint Horizon

```gherkin
Given el equipo iniciando el Sprint 1 para codificar el MVP del Pod AFIP
When el desarrollador ejecuta "go test ./..." y "golangci-lint run"
Then la rebanada vertical debe responder correctamente una consulta sobre certificados AFIP
And verificar la cita explícita del documento PDF de AFIP en < 3,000 ms
And todas las pruebas BDD de las especificaciones 01, 02 y 03 deben estar en estado PASSED
```
