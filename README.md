# 🚀 AI Pods Enterprise (SaaS) - Documentación & Especificaciones

Este repositorio contiene la documentación del proyecto y la suite completa de especificaciones ejecutables de **AI Pods Enterprise**, una plataforma SaaS universal, multi-tenant y agnóstica para la creación, ejecución y orquestación de agentes inteligentes (**AI Pods**) conectables a cualquier ERP (Odoo, SAP, Salesforce), CRM (EvoCRM), APIs, bases de datos o portales web.

## 📄 Estructura de Documentación

* **[`VERSION`](file:///home/martin/server/onlyone%20ai%20pods/VERSION):** Versión actual de la plataforma (`4.8.0`).
* **[`LICENSE`](file:///home/martin/server/onlyone%20ai%20pods/LICENSE):** Licencia Propietaria del Autor (Martin Llanos).
* **[`.aipods/skills/`](file:///home/martin/server/onlyone%20ai%20pods/.aipods/skills/):** Kit Físico de Agentic Skills (`core-go-architect`, `multi-tenant-security`, `frontend-ui-architect`, `sdd-spec-writer`) y reglas Git (`github_workflow.md`).
* **[`docs/BACKLOG.md`](file:///home/martin/server/onlyone%20ai%20pods/docs/BACKLOG.md):** Backlog del producto con 11 Épicas e Historias de Usuario consolidadas.
* **[`docs/SDD.md`](file:///home/martin/server/onlyone%20ai%20pods/docs/SDD.md):** Documento de Diseño de Software (Arquitectura Enterprise en Go, PostgreSQL 16 Enterprise, Qdrant Cluster, Redis Active-Active, NATS JetStream, Portales Aislados, OAuth2/OIDC, OpenTelemetry, DRP RPO<1min, FinOps y Cumplimiento ISO 9001, SOC 2 Type II & ISO 27001).
* **[`docs/DEVELOPMENT_ONBOARDING_CHECKLIST.md`](file:///home/martin/server/onlyone%20ai%20pods/docs/DEVELOPMENT_ONBOARDING_CHECKLIST.md):** Guía Completa de Onboarding, Arquitectura Multi-Repositorio (`onlyone-ai-pods`), Flujo `gh` CLI y Estándar de Ramas.
* **[`docs/SDD_ENGINEERING_FEEDBACK.md`](file:///home/martin/server/onlyone%20ai%20pods/docs/SDD_ENGINEERING_FEEDBACK.md):** Informe de Devolución de Ingeniería SDD.
* **[`specs/`](file:///home/martin/server/onlyone%20ai%20pods/specs/README.md):** Suite Completa de 23 Especificaciones SDD organizada en **4 Dominios Temáticos** (`01_architecture_core/`, `02_security_and_compliance/`, `03_plugin_and_skills/`, `04_customer_portal_growth/`, `pods/` y `api/`).
* **[`cmd/server/main.go`](file:///home/martin/server/onlyone%20ai%20pods/cmd/server/main.go):** Backend en Go 1.22+ compilado con endpoints `/healthz` y `/api/v1/chat/completions` integrados con `tenant`, `router`, y `pod/afip`.

---

## 🔒 Licencia

Este proyecto y su documentación están protegidos bajo una **Licencia Propietaria del Autor**. Todos los derechos reservados © 2026 Martin Llanos. Ver el archivo [`LICENSE`](file:///home/martin/server/onlyone%20ai%20pods/LICENSE) para más detalles.

---

## 📌 Estándar de Commits (Odoo Commit Standard)

Todos los commits de este repositorio siguen el estándar de contribución de la industria (**Odoo Commit Format**):

```text
[TAG] module: short summary in present imperative

Detailed explanation of changes...
```

* `[ADD]`: Agregado de especificaciones, documentación o características.
* `[IMP]`: Mejoras en documentos o esquemas existentes.
* `[REF]`: Reestructuración de especificaciones.
