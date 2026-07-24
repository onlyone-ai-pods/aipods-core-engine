# 🛠️ Spec-Driven Development (SDD) Framework
## Proyecto AI Pods para Consultoría Odoo (SaaS)

Este directorio contiene las **especificaciones formales ejecutables** que dirigen el desarrollo, las pruebas unitarias/integración y la validación de comportamiento de la plataforma SaaS y sus AI Pods.

---

## 🎯 ¿Qué es Spec-Driven Development en este proyecto?

En este proyecto, las **Especificaciones (Specs)** preceden a la implementación de código y a la ingeniería de prompts. Cada componente técnico y AI Pod posee un archivo `.spec.md` estructurado que sirve como:

1. **Contrato de Interfaz y Datos (Schemas):** Entradas, salidas, eventos y payload JSON esperados.
2. **Escenarios de Comportamiento (BDD / Given-When-Then):** Casos de prueba funcionales ejecutables para validar tanto código como respuestas de los modelos LLM.
3. **Restricciones de Seguridad e Invariantes:** Reglas estrictamente enforzadas (aislamiento tenant_id, caché semántico, arquitectura de plugins, gobernanza de releases, AuthN/AuthZ, DRP, FinOps, Clean Code, Agentic Skills Kit, Red-Teaming, Sandbox de Clientes).

---

## 📂 Estructura del Directorio `specs/`

```text
specs/
├── README.md                           # Guía de la metodología SDD
├── core/                               # Especificaciones del Engine Core
│   ├── 01_smart_router_spec.md         # Router & Clasificador de Intenciones
│   ├── 02_rag_pipeline_spec.md         # Ingesta, Vectorización y RAG
│   ├── 03_multi_tenant_spec.md         # Aislamiento por Metadatos & Seguridad
│   ├── 04_semantic_cache_spec.md       # Caché Semántico y Colas Asíncronas
│   ├── 05_plugin_architecture_spec.md  # Arquitectura de Plugins & Extensibilidad AI Pods
│   ├── 06_lifecycle_and_governance_spec.md # Ciclo de Vida, Gobernanza & Rollback Automático
│   ├── 07_enterprise_architecture_parameters_spec.md # AuthN/AuthZ, DRP, OpenTelemetry, FinOps & DevOps
│   ├── 08_clean_code_and_security_linting_spec.md # Código Limpio, Security Linters (gosec) & CI Gates
│   ├── 09_plugin_scaffold_and_agentic_skills_spec.md # SDK, Scaffold & Agentic Skills External Plugins
│   ├── 10_internal_core_agentic_skills_spec.md # Agentic Skills Internos para el Equipo Core (.aipods/skills/)
│   ├── 11_product_roadmap_and_marketplace_spec.md # Red-Teaming, Monetización SaaS & Marketplace de Pods
│   └── 12_customer_portal_marketing_and_sandbox_spec.md # Landing Pública, Sandbox Interactivo & Customer Dashboard
├── pods/                               # Especificaciones de AI Pods por Dominio
│   ├── 01_afip_finance_spec.md         # Pod AFIP / ARCA & Balances Financieros
│   ├── 02_evocrm_helpdesk_spec.md      # Pod EvoCRM & Odoo Helpdesk
│   ├── 03_social_marketing_spec.md     # Pod Odoo Social Marketing
│   └── 04_scm_logistics_spec.md        # Pod Cadena de Suministros (WMS/MRP/Compras)
└── api/                                # Contrato OpenAPI / HTTP / WebSocket
    └── openapi_spec.yaml               # Especificación OpenAPI 3.0
```

---

## 📋 Matriz de Trazabilidad (Backlog -> Specs)

| Componente / Épica | Historia de Usuario | Archivo de Especificación (Spec) |
| :--- | :--- | :--- |
| **Épica 1** | HU 1.1, HU 1.2 | [`specs/pods/01_afip_finance_spec.md`](file:///home/martin/server/onlyone%20ai%20pods/specs/pods/01_afip_finance_spec.md) |
| **Épica 2** | HU 2.1, HU 2.2 | [`specs/pods/02_evocrm_helpdesk_spec.md`](file:///home/martin/server/onlyone%20ai%20pods/specs/pods/02_evocrm_helpdesk_spec.md) |
| **Épica 3** | HU 3.1 | [`specs/pods/03_social_marketing_spec.md`](file:///home/martin/server/onlyone%20ai%20pods/specs/pods/03_social_marketing_spec.md) |
| **Épica 4** | HU 4.1, HU 4.2, HU 4.3 | [`specs/pods/04_scm_logistics_spec.md`](file:///home/martin/server/onlyone%20ai%20pods/specs/pods/04_scm_logistics_spec.md) |
| **Épica 5** | HU 5.1 | [`specs/core/01_smart_router_spec.md`](file:///home/martin/server/onlyone%20ai%20pods/specs/core/01_smart_router_spec.md) |
| **Épica 6 & 7** | HU 6.1, HU 7.1 | [`specs/core/02_rag_pipeline_spec.md`](file:///home/martin/server/onlyone%20ai%20pods/specs/core/02_rag_pipeline_spec.md) |
| **Épica 8** | HU 8.1 | [`specs/core/03_multi_tenant_spec.md`](file:///home/martin/server/onlyone%20ai%20pods/specs/core/03_multi_tenant_spec.md) |
| **Épica 9** | HU 9.1, HU 9.2 | [`specs/core/04_semantic_cache_spec.md`](file:///home/martin/server/onlyone%20ai%20pods/specs/core/04_semantic_cache_spec.md) |
| **Épica 10** | HU 10.1 | [`specs/core/02_rag_pipeline_spec.md`](file:///home/martin/server/onlyone%20ai%20pods/specs/core/02_rag_pipeline_spec.md) |
| **Extensibilidad & Plugins**| Requisito Arquitectura | [`specs/core/05_plugin_architecture_spec.md`](file:///home/martin/server/onlyone%20ai%20pods/specs/core/05_plugin_architecture_spec.md) |
| **Gobernanza & Lifecycle** | Requisito Plataforma | [`specs/core/06_lifecycle_and_governance_spec.md`](file:///home/martin/server/onlyone%20ai%20pods/specs/core/06_lifecycle_and_governance_spec.md) |
| **Parámetros Empresariales**| Seguridad/Ops/FinOps | [`specs/core/07_enterprise_architecture_parameters_spec.md`](file:///home/martin/server/onlyone%20ai%20pods/specs/core/07_enterprise_architecture_parameters_spec.md) |
| **Código Limpio & Linters** | Calidad & Audibilidad | [`specs/core/08_clean_code_and_security_linting_spec.md`](file:///home/martin/server/onlyone%20ai%20pods/specs/core/08_clean_code_and_security_linting_spec.md) |
| **Plugin Scaffold & AI Skills**| Dev External Ecosystem | [`specs/core/09_plugin_scaffold_and_agentic_skills_spec.md`](file:///home/martin/server/onlyone%20ai%20pods/specs/core/09_plugin_scaffold_and_agentic_skills_spec.md) |
| **Internal Team Agentic Skills**| Dev Internal Core Team | [`specs/core/10_internal_core_agentic_skills_spec.md`](file:///home/martin/server/onlyone%20ai%20pods/specs/core/10_internal_core_agentic_skills_spec.md) |
| **Roadmap, Red-Teaming & Marketplace**| Estrategia de Producto | [`specs/core/11_product_roadmap_and_marketplace_spec.md`](file:///home/martin/server/onlyone%20ai%20pods/specs/core/11_product_roadmap_and_marketplace_spec.md) |
| **Landing, Sandbox & Dashboard**| Portal Clientes & Growth | [`specs/core/12_customer_portal_marketing_and_sandbox_spec.md`](file:///home/martin/server/onlyone%20ai%20pods/specs/core/12_customer_portal_marketing_and_sandbox_spec.md) |
