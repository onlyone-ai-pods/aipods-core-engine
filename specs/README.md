# 🛠️ Spec-Driven Development (SDD) Framework
## Proyecto AI Pods Enterprise SaaS Platform

Este directorio contiene las **24 especificaciones formales ejecutables** organizadas en 4 Dominios de Arquitectura que dirigen el desarrollo, las pruebas BDD y la gobernanza de la plataforma.

---

## 🎯 ¿Qué es Spec-Driven Development en este proyecto?

En este proyecto, las **Especificaciones (Specs)** preceden a la implementación de código y a la ingeniería de prompts. Cada componente técnico y AI Pod posee un archivo `.spec.md` estructurado que sirve como:

1. **Contrato de Interfaz y Datos (Schemas):** Entradas, salidas, eventos y payload JSON esperados.
2. **Escenarios de Comportamiento (BDD / Given-When-Then):** Casos de prueba funcionales ejecutables en Go (`godog`) para validar código y respuestas LLM.
3. **Restricciones de Seguridad e Invariantes:** Reglas estrictamente enforzadas (aislamiento tenant_id, caché semántico, plugins, AuthN/AuthZ RS256, DRP, ISO 9001/SOC 2, Dry-Run, GitHub CLI `gh`, BDD automatizado, Onboarding Cero Fricción, Autoconsumo Dogfooding, Integración GitHub API/Odoo.sh y SAP Enterprise/B1).

---

## 📂 Estructura Organizada por Dominios (`specs/`)

```text
specs/
├── README.md                           # Guía de la metodología SDD
├── 01_architecture_core/              # Dominios de Motor Backend & DBs
│   ├── 01_smart_router_spec.md         # Router & Clasificador de Intenciones
│   ├── 02_rag_pipeline_spec.md         # Ingesta, Vectorización y RAG
│   ├── 03_multi_tenant_spec.md         # Aislamiento por Metadatos & Seguridad
│   └── 04_semantic_cache_spec.md       # Caché Semántico y Colas Asíncronas
├── 02_security_and_compliance/        # Dominios de Seguridad, Ops & ISO/SOC2
│   ├── 07_enterprise_architecture_parameters_spec.md # AuthN/AuthZ, DRP, OTel, FinOps
│   ├── 08_clean_code_and_security_linting_spec.md # Linters (gosec, trivy) & CI Gates
│   ├── 14_dry_run_execution_protocol_spec.md # Protocolo Mandatorio Dry-Run
│   ├── 15_pod_standards_and_policy_governance_spec.md # Estándares & Perfiles de Política
│   ├── 16_iso9001_soc2_iso27001_compliance_spec.md # Marco ISO 9001, SOC 2 & ISO 27001
│   └── 19_github_cli_and_spec_pr_governance_spec.md # Gobernanza gh CLI & Spec PR Gate
├── 03_plugin_and_skills/              # Dominios de Plugins, Skills & Testing
│   ├── 05_plugin_architecture_spec.md  # Arquitectura de Plugins & WASM Sandbox
│   ├── 06_lifecycle_and_governance_spec.md # Gobernanza de Releases & Auto-Rollback
│   ├── 09_plugin_scaffold_and_agentic_skills_spec.md # SDK & Agentic Skills External
│   ├── 10_internal_core_agentic_skills_spec.md # Agentic Skills Internos Core
│   ├── 11_product_roadmap_and_marketplace_spec.md # Red-Teaming & Marketplace
│   ├── 18_incremental_execution_roadmap_and_vertical_slicing_spec.md # Vertical Slicing
│   └── 20_bdd_test_automation_and_tiered_evals_spec.md # Tests BDD godog en Go
├── 04_customer_portal_growth/          # Dominios de Portal Web, Sandbox & Growth
│   ├── 12_customer_portal_marketing_and_sandbox_spec.md # Landing & Interactive Sandbox
│   ├── 13_business_justifications_capabilities_and_limitations_spec.md # ROI & Limits
│   ├── 17_continuous_improvement_and_user_feedback_spec.md # Feedback Loop 👍/👎
│   ├── 21_customer_onboarding_and_provisioning_spec.md # Protocolo Onboarding & Provisioning
│   └── 22_self_consuming_dogfooding_crm_and_billing_spec.md # Autoconsumo CRM & Billing
├── pods/                               # Especificaciones de AI Pods por Dominio
│   ├── 01_afip_finance_spec.md         # Pod AFIP / ARCA & Balances Financieros
│   ├── 02_evocrm_helpdesk_spec.md      # Pod EvoCRM & Odoo Helpdesk
│   ├── 03_social_marketing_spec.md     # Pod Odoo Social Marketing
│   ├── 04_scm_logistics_spec.md        # Pod Cadena de Suministros (WMS/MRP/Compras)
│   ├── 05_github_devops_odoo_sh_spec.md # Pod GitHub API & Odoo.sh DevOps Integrator
│   └── 06_sap_enterprise_and_b1_spec.md # Pod SAP Enterprise (S/4HANA/ECC) & Business One
└── api/                                # Contrato OpenAPI / HTTP / WebSocket
    └── openapi_spec.yaml               # Especificación OpenAPI 3.0
```

---

## 📋 Matriz de Trazabilidad por Dominio (Backlog -> Specs)

| Dominio | Componente / Épica | Archivo de Especificación (Spec) |
| :--- | :--- | :--- |
| **01 Core** | Épica 5 (Smart Router) | [`specs/01_architecture_core/01_smart_router_spec.md`](file:///home/martin/server/onlyone%20ai%20pods/specs/01_architecture_core/01_smart_router_spec.md) |
| **01 Core** | Épica 6 & 7 (RAG) | [`specs/01_architecture_core/02_rag_pipeline_spec.md`](file:///home/martin/server/onlyone%20ai%20pods/specs/01_architecture_core/02_rag_pipeline_spec.md) |
| **01 Core** | Épica 8 (Multi-Tenant) | [`specs/01_architecture_core/03_multi_tenant_spec.md`](file:///home/martin/server/onlyone%20ai%20pods/specs/01_architecture_core/03_multi_tenant_spec.md) |
| **01 Core** | Épica 9 (Caché Semántico) | [`specs/01_architecture_core/04_semantic_cache_spec.md`](file:///home/martin/server/onlyone%20ai%20pods/specs/01_architecture_core/04_semantic_cache_spec.md) |
| **02 Seguridad** | AuthN/AuthZ/DRP | [`specs/02_security_and_compliance/07_enterprise_architecture_parameters_spec.md`](file:///home/martin/server/onlyone%20ai%20pods/specs/02_security_and_compliance/07_enterprise_architecture_parameters_spec.md) |
| **02 Seguridad** | Linters & CI Gates | [`specs/02_security_and_compliance/08_clean_code_and_security_linting_spec.md`](file:///home/martin/server/onlyone%20ai%20pods/specs/02_security_and_compliance/08_clean_code_and_security_linting_spec.md) |
| **02 Seguridad** | Dry-Run Protocol | [`specs/02_security_and_compliance/14_dry_run_execution_protocol_spec.md`](file:///home/martin/server/onlyone%20ai%20pods/specs/02_security_and_compliance/14_dry_run_execution_protocol_spec.md) |
| **02 Seguridad** | Gobernanza de Políticas | [`specs/02_security_and_compliance/15_pod_standards_and_policy_governance_spec.md`](file:///home/martin/server/onlyone%20ai%20pods/specs/02_security_and_compliance/15_pod_standards_and_policy_governance_spec.md) |
| **02 Seguridad** | ISO 9001 / SOC 2 | [`specs/02_security_and_compliance/16_iso9001_soc2_iso27001_compliance_spec.md`](file:///home/martin/server/onlyone%20ai%20pods/specs/02_security_and_compliance/16_iso9001_soc2_iso27001_compliance_spec.md) |
| **02 Seguridad** | GitHub CLI & Spec PR Gate | [`specs/02_security_and_compliance/19_github_cli_and_spec_pr_governance_spec.md`](file:///home/martin/server/onlyone%20ai%20pods/specs/02_security_and_compliance/19_github_cli_and_spec_pr_governance_spec.md) |
| **03 Plugins** | Plugins Architecture | [`specs/03_plugin_and_skills/05_plugin_architecture_spec.md`](file:///home/martin/server/onlyone%20ai%20pods/specs/03_plugin_and_skills/05_plugin_architecture_spec.md) |
| **03 Plugins** | Lifecycle & Rollback | [`specs/03_plugin_and_skills/06_lifecycle_and_governance_spec.md`](file:///home/martin/server/onlyone%20ai%20pods/specs/03_plugin_and_skills/06_lifecycle_and_governance_spec.md) |
| **03 Plugins** | External Developer SDK | [`specs/03_plugin_and_skills/09_plugin_scaffold_and_agentic_skills_spec.md`](file:///home/martin/server/onlyone%20ai%20pods/specs/03_plugin_and_skills/09_plugin_scaffold_and_agentic_skills_spec.md) |
| **03 Plugins** | Core Agentic Skills | [`specs/03_plugin_and_skills/10_internal_core_agentic_skills_spec.md`](file:///home/martin/server/onlyone%20ai%20pods/specs/03_plugin_and_skills/10_internal_core_agentic_skills_spec.md) |
| **03 Plugins** | Roadmap & Marketplace | [`specs/03_plugin_and_skills/11_product_roadmap_and_marketplace_spec.md`](file:///home/martin/server/onlyone%20ai%20pods/specs/03_plugin_and_skills/11_product_roadmap_and_marketplace_spec.md) |
| **03 Plugins** | Vertical Slicing | [`specs/03_plugin_and_skills/18_incremental_execution_roadmap_and_vertical_slicing_spec.md`](file:///home/martin/server/onlyone%20ai%20pods/specs/03_plugin_and_skills/18_incremental_execution_roadmap_and_vertical_slicing_spec.md) |
| **03 Plugins** | BDD Testing `godog` | [`specs/03_plugin_and_skills/20_bdd_test_automation_and_tiered_evals_spec.md`](file:///home/martin/server/onlyone%20ai%20pods/specs/03_plugin_and_skills/20_bdd_test_automation_and_tiered_evals_spec.md) |
| **04 Portal** | Customer Portal & Sandbox | [`specs/04_customer_portal_growth/12_customer_portal_marketing_and_sandbox_spec.md`](file:///home/martin/server/onlyone%20ai%20pods/specs/04_customer_portal_growth/12_customer_portal_marketing_and_sandbox_spec.md) |
| **04 Portal** | Business ROI & Limits | [`specs/04_customer_portal_growth/13_business_justifications_capabilities_and_limitations_spec.md`](file:///home/martin/server/onlyone%20ai%20pods/specs/04_customer_portal_growth/13_business_justifications_capabilities_and_limitations_spec.md) |
| **04 Portal** | Feedback Loop 👍/👎 | [`specs/04_customer_portal_growth/17_continuous_improvement_and_user_feedback_spec.md`](file:///home/martin/server/onlyone%20ai%20pods/specs/04_customer_portal_growth/17_continuous_improvement_and_user_feedback_spec.md) |
| **04 Portal** | Onboarding & Provisioning | [`specs/04_customer_portal_growth/21_customer_onboarding_and_provisioning_spec.md`](file:///home/martin/server/onlyone%20ai%20pods/specs/04_customer_portal_growth/21_customer_onboarding_and_provisioning_spec.md) |
| **04 Portal** | Autoconsumo Dogfooding | [`specs/04_customer_portal_growth/22_self_consuming_dogfooding_crm_and_billing_spec.md`](file:///home/martin/server/onlyone%20ai%20pods/specs/04_customer_portal_growth/22_self_consuming_dogfooding_crm_and_billing_spec.md) |
| **Pods** | Pod AFIP / ARCA | [`specs/pods/01_afip_finance_spec.md`](file:///home/martin/server/onlyone%20ai%20pods/specs/pods/01_afip_finance_spec.md) |
| **Pods** | Pod EvoCRM / Helpdesk | [`specs/pods/02_evocrm_helpdesk_spec.md`](file:///home/martin/server/onlyone%20ai%20pods/specs/pods/02_evocrm_helpdesk_spec.md) |
| **Pods** | Pod Social Marketing | [`specs/pods/03_social_marketing_spec.md`](file:///home/martin/server/onlyone%20ai%20pods/specs/pods/03_social_marketing_spec.md) |
| **Pods** | Pod Cadena Suministros | [`specs/pods/04_scm_logistics_spec.md`](file:///home/martin/server/onlyone%20ai%20pods/specs/pods/04_scm_logistics_spec.md) |
| **Pods** | Pod GitHub API & Odoo.sh | [`specs/pods/05_github_devops_odoo_sh_spec.md`](file:///home/martin/server/onlyone%20ai%20pods/specs/pods/05_github_devops_odoo_sh_spec.md) |
| **Pods** | Pod SAP Enterprise & B1 | [`specs/pods/06_sap_enterprise_and_b1_spec.md`](file:///home/martin/server/onlyone%20ai%20pods/specs/pods/06_sap_enterprise_and_b1_spec.md) |
