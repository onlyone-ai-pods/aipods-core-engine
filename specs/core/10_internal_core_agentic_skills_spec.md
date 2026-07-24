# 📜 SPEC: Habilidades de IA para el Equipo Interno (Internal Core Agentic Skills)
**ID:** SPEC-CORE-10  
**Épica Relacionada:** Ecosistema de Desarrollo Interno & Asistencia con IA  
**Estado:** PROPOSED / SPEC-DRIVEN  

---

## 1. Visión y Objetivos

Esta especificación establece la suite de **Agentic Skills Internos (`.aipods/skills/`)** para el equipo principal de desarrollo (los 3 desarrolladores internos). Garantiza que cuando los miembros del equipo utilicen asistentes de IA (Antigravity, Cursor, Claude Code, etc.) en el repositorio central del Core, **las IAs generen código que cumpla de forma automática e infalible** con las decisiones de arquitectura en Go 1.22+, seguridad multi-tenant y estándares de calidad del proyecto.

---

## 2. Estructura de Skills para el Repositorio Core

El repositorio principal del proyecto incluirá la carpeta `.aipods/` con las siguientes 4 Habilidades Especializadas:

```text
.aipods/
├── skills/
│   ├── core-go-architect/
│   │   └── SKILL.md             # Guía para arquitectura backend en Go 1.22+
│   ├── multi-tenant-security/
│   │   └── SKILL.md             # Guía de aislamiento estricto tenant_id
│   ├── rag-eval-engineer/
│   │   └── SKILL.md             # Guía de pruebas Evals y Golden Datasets
│   └── sdd-spec-writer/
│       └── SKILL.md             # Guía para redacción de especificaciones BDD (.spec.md)
└── rules/
    └── odoo_commit_style.md     # Regla estricta para formato de commits en Odoo
```

---

## 3. Especificación de los Agentic Skills Internos

### 3.1 Skill: `core-go-architect/SKILL.md`
```markdown
---
name: core-go-architect
description: Guía de arquitectura e idioma oficial Go 1.22+ para el backend del proyecto AI Pods.
---

# Core Go Architect Skill

Al generar o refactorizar código Go en el repositorio principal, el asistente de IA DEBE:
1. Usar Go 1.22+ idiomático, pasando siempre `ctx context.Context` como primer argumento en funciones I/O.
2. Manejar TODO error devuelto explícitamente envolviéndolo: `fmt.Errorf("contexto: %w", err)`. Prohibido usar `_` para ignorar errores.
3. No agregar dependencias de terceros sin justificación (mantener el estándar de auditabilidad).
4. Pasar todas las verificaciones del linter `golangci-lint` y `gosec` sin excepciones.
```

### 3.2 Skill: `multi-tenant-security/SKILL.md`
```markdown
---
name: multi-tenant-security
description: Invariantes de seguridad multi-tenant para consultas PostgreSQL y Qdrant.
---

# Multi-Tenant Security Skill

Al construir cualquier consulta a la base de datos relacional o vectorial:
1. NUNCA generes una consulta a PostgreSQL o Qdrant sin incluir el filtro estricto:
   `WHERE (tenant_id == CurrentTenantID OR tenant_id == 'GLOBAL') AND status == 'ACTIVE'`
2. Extrae siempre el `tenant_id` del contexto JWT autenticado (`ctx`), nunca confíes en inputs del cuerpo de la consulta sin validar.
```

### 3.3 Skill: `sdd-spec-writer/SKILL.md`
```markdown
---
name: sdd-spec-writer
description: Reglas de Spec-Driven Development (SDD) para redactar especificaciones BDD.
---

# SDD Spec Writer Skill

1. Ninguna funcionalidad se codifica sin tener antes un archivo `.spec.md` en `/specs/`.
2. Toda especificación debe incluir escenarios ejecutables BDD en formato `Given / When / Then`.
3. Mantener actualizada la matriz de trazabilidad en `specs/README.md`.
```

---

## 4. Escenario BDD de Asistencia Interna con IA

```gherkin
Given un desarrollador interno implementando un nuevo endpoint de historial de consultas RAG en Go
And utilizando un asistente de IA en su entorno de desarrollo
When el asistente de IA consulta la habilidad ".aipods/skills/multi-tenant-security/SKILL.md"
Then la consulta SQL y vectorial generada por la IA debe incluir el filtro tenant_id de forma mandatoria
And el código en Go resultante debe compilar limpiamente bajo golangci-lint sin advertencias de gosec
```
