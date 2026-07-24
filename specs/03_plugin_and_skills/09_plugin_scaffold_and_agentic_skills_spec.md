# 📜 SPEC: Scaffold de Plugins, SDK y Skills para Agentes de IA (Agentic Skills Kit)
**ID:** SPEC-CORE-09  
**Épica Relacionada:** Extensibilidad de Plugins, Ecosistema de Desarrolladores & AI Agent Tooling  
**Estado:** PROPOSED / SPEC-DRIVEN  

---

## 1. Visión y Objetivos

Esta especificación establece la estrategia para **desarrolladores externos** que crean nuevos **AI Pod Plugins**, permitiéndoles a ellos (y a sus **agentes de Inteligencia Artificial** como Antigravity, Cursor, Claude Code, etc.) construir plugins perfectamente integrados, seguros y gobernados desde el primer minuto.

---

## 2. Estrategia de Agentic Skills & Scaffold (`aipod-cli` + `.aipods/`)

Para garantizar que los desarrolladores externos y sus asistentes de IA no adivinen ni cometan errores de arquitectura o seguridad multi-tenant, se provee un **Scaffold Oficial** con habilidades nativas para agentes de IA (**Agentic Skills**).

```text
my-custom-pod-plugin/
├── .aipods/                            # Contexto & Skills para Agentes de IA
│   ├── skills/
│   │   └── aipod-builder/
│   │       └── SKILL.md                # Guía ejecutable para asistentes de IA
│   └── rules/
│       └── pod_governance.md           # Reglas de seguridad estáticas para IA
├── pod_manifest.json                   # Manifiesto con validación por JSON Schema
├── plugin.go                           # Implementación de la interfaz BaseAIPod en Go
├── tools/                              # Herramientas / Functions especializadas
│   └── custom_tools.go
├── tests/
│   └── pod_eval_test.go                # Suite de Evals local pre-aprobación
└── README.md                           # Guía humana & prompt de inicialización
```

---

## 3. Especificación del Agentic Skill (`.aipods/skills/aipod-builder/SKILL.md`)

Todo repositorio plantilla de plugin incluirá un archivo `SKILL.md` que los asistentes de IA leen automáticamente al abrir el proyecto:

```markdown
---
name: aipod-builder
description: Guía experta para construir AI Pod Plugins seguros y compatibles con la plataforma SaaS AI Pods Odoo.
---

# AI Pod Builder Skill

Al generar o modificar código para este AI Pod Plugin, DEBES obedecer estrictamente las siguientes reglas:

1. **Implementar la Interfaz BaseAIPod:**
   - Todo plugin debe exponer el método `EvaluateIntentConfidence(query string) float64`.
   - Todo plugin debe exponer `GetSystemPrompt(tenantID string) string`.
   - Todo plugin debe exponer `GetTools(tenantID string) []Tool`.

2. **Seguridad Multi-Tenant Estricta:**
   - NUNCA generes código RAG que omita el filtro `tenant_id == X OR tenant_id == 'GLOBAL'`.
   - NUNCA almacenes datos de un tenant en memoria global del plugin.

3. **Gobernanza & Limites:**
   - La ejecución de herramientas debe responder en < 3,000 ms.
   - Todo input de usuario recibido en una Tool debe ser validado con esquemas JSON / Pydantic.
```

---

## 4. CLI de Validación de Plugins (`aipod-cli`)

Se proveerá una herramienta de línea de comandos para que los desarrolladores humanos y pipelines de CI ejecuten en local:

```bash
# Inicializar un nuevo proyecto de Plugin con el scaffold completo
aipod-cli init --name="PodLegalCompliance" --domain="LEGAL"

# Validar el plugin localmente antes de enviarlo
aipod-cli validate .
```

### El comando `aipod-cli validate` verifica automáticamente:
1. **Validación del Manifiesto:** Valida `pod_manifest.json` contra el JSON Schema oficial.
2. **Chequeo de Interfaz Go:** Confirma que `plugin.go` implementa todos los métodos de `BaseAIPod`.
3. **Escaneo de Seguridad `gosec`:** Verifica que no existan inyecciones ni fuga de `tenant_id`.
4. **Ejecución de Evals BDD Locales:** Corre la suite de pruebas locales contra los Golden Datasets del plugin.

---

## 5. Escenario BDD de Integración de Plugins Externos

```gherkin
Given un desarrollador externo creando el plugin "PodLogisticaInternacional"
And utilizando un asistente de IA (ej. Antigravity / Cursor) en su IDE
When el asistente de IA lee el archivo ".aipods/skills/aipod-builder/SKILL.md"
Then el código Go y el manifiesto generados por la IA deben cumplir 100% con la interfaz BaseAIPod
And el comando "aipod-cli validate ." debe retornar el estado PASSED en < 10 segundos
```
