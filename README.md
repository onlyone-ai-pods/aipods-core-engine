# 🚀 AI Pods para Consultoría Odoo (SaaS) - Documentación & Especificaciones

Este repositorio contiene la documentación del proyecto y la suite de especificaciones ejecutables bajo la metodología **Spec-Driven Development (SDD)**.

## 📄 Estructura de Documentación

* **[`VERSION`](file:///home/martin/server/onlyone%20ai%20pods/VERSION):** Versión actual de la documentación (`1.5.0`).
* **[`LICENSE`](file:///home/martin/server/onlyone%20ai%20pods/LICENSE):** Licencia Propietaria y Confidencial.
* **[`docs/BACKLOG.md`](file:///home/martin/server/onlyone%20ai%20pods/docs/BACKLOG.md):** Backlog del producto con 10 Épicas e Historias de Usuario consolidadas.
* **[`docs/SDD.md`](file:///home/martin/server/onlyone%20ai%20pods/docs/SDD.md):** Documento de Diseño de Software (Arquitectura Enterprise en Go, PostgreSQL 16 Enterprise, Qdrant Cluster, Redis Active-Active, NATS JetStream, Portales Aislados, OAuth2/OIDC, OpenTelemetry, DRP RPO<1min y FinOps).
* **[`docs/DEVELOPMENT_ONBOARDING_CHECKLIST.md`](file:///home/martin/server/onlyone%20ai%20pods/docs/DEVELOPMENT_ONBOARDING_CHECKLIST.md):** Checklist de Requisitos Previos, Cuentas de Plataformas, APIs, Docker Compose local y Datasets de Prueba para iniciar codificación.
* **[`specs/`](file:///home/martin/server/onlyone%20ai%20pods/specs/README.md):** Suite de especificaciones SDD (Smart Router, RAG, Multi-Tenant, Caché, Arquitectura de Plugins, Gobernanza de Releases & Rollback Automático, Parámetros Empresariales AuthN/AuthZ/DRP/FinOps, Pods AFIP/EvoCRM/Social/SCM y OpenAPI contract).

---

## 🔒 Licencia

Este proyecto y su documentación están protegidos bajo una **Licencia Propietaria y Confidencial**. Todos los derechos reservados © 2026 Martin Llanos. Ver el archivo [`LICENSE`](file:///home/martin/server/onlyone%20ai%20pods/LICENSE) para más detalles.

---

## 📌 Estándar de Commits (Odoo Format)

Todos los commits de este repositorio siguen el estándar de contribución de **Odoo**:

```text
[TAG] module: short summary in present imperative

Detailed explanation of changes...
```

* `[ADD]`: Agregado de especificaciones, documentación o características.
* `[IMP]`: Mejoras en documentos o esquemas existentes.
* `[FIX]`: Correcciones en contratos o especificidades.
* `[REF]`: Reestructuración de especificaciones.
