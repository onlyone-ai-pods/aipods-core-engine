# 🚀 AI Pods para Consultoría Odoo (SaaS) - Documentación & Especificaciones

Este repositorio contiene la documentación del proyecto y la suite de especificaciones ejecutables bajo la metodología **Spec-Driven Development (SDD)**.

## 📄 Estructura de Documentación

* **[`VERSION`](file:///home/martin/server/onlyone%20ai%20pods/VERSION):** Versión actual de la documentación (`3.3.0`).
* **[`LICENSE`](file:///home/martin/server/onlyone%20ai%20pods/LICENSE):** Licencia Propietaria y Confidencial.
* **[`docs/BACKLOG.md`](file:///home/martin/server/onlyone%20ai%20pods/docs/BACKLOG.md):** Backlog del producto con 10 Épicas e Historias de Usuario consolidadas.
* **[`docs/SDD.md`](file:///home/martin/server/onlyone%20ai%20pods/docs/SDD.md):** Documento de Diseño de Software (Arquitectura Enterprise en Go, PostgreSQL 16 Enterprise, Qdrant Cluster, Redis Active-Active, NATS JetStream, Portales Aislados, OAuth2/OIDC, OpenTelemetry, DRP RPO<1min, FinOps y Cumplimiento ISO 9001, SOC 2 Type II & ISO 27001).
* **[`docs/DEVELOPMENT_ONBOARDING_CHECKLIST.md`](file:///home/martin/server/onlyone%20ai%20pods/docs/DEVELOPMENT_ONBOARDING_CHECKLIST.md):** Checklist de Requisitos Previos, Cuentas de Plataformas, APIs, Docker Compose local y Datasets de Prueba para iniciar codificación.
* **[`docs/SDD_ENGINEERING_FEEDBACK.md`](file:///home/martin/server/onlyone%20ai%20pods/docs/SDD_ENGINEERING_FEEDBACK.md):** Informe de Devolución de Ingeniería SDD y Matriz de Cumplimiento 100% de Recomendaciones.
* **[`specs/`](file:///home/martin/server/onlyone%20ai%20pods/specs/README.md):** Suite Completa de 20 Especificaciones SDD organizada en **4 Dominios Temáticos** (`01_architecture_core/`, `02_security_and_compliance/`, `03_plugin_and_skills/`, `04_customer_portal_growth/`, `pods/` y `api/`).

---

## 🔒 Licencia

Este proyecto y su documentación están protegidos bajo una **Licencia Propietaria del Autor**. Todos los derechos reservados © 2026 Martin Llanos. Ver el archivo [`LICENSE`](file:///home/martin/server/onlyone%20ai%20pods/LICENSE) para más detalles.

---

## 📌 Estándar de Commits (Odoo Format)

Todos los commits de este repositorio siguen el estándar de contribución de **Odoo**:

```text
[TAG] module: short summary in present imperative

Detailed explanation of changes...
```

* `[ADD]`: Agregado de especificaciones, documentación o características.
* `[IMP]`: Mejoras en documentos o esquemas existentes.
* `[REF]`: Reestructuración de especificaciones.
