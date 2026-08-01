# 🛡️ REGLAS SISTÉMICAS MANDATORIAS PARA AGENTES DE IA (ANTIGRAVITY / CLAUDE / CURSOR)

---

## ⚠️ DIRECTIVA MANDATORIA DE INICIO DE SESIÓN (PASO 0 INMEDIATO)

Cualquier Agente de IA o Asistente de Codificación (Antigravity, Claude Code, Cursor, Copilot, Windsurf) que inicie una conversación o tarea en este repositorio DEBE ejecutar las siguientes acciones de forma automática e inmediata antes de proponer código, cambios o responder a solicitudes:

1. **Lectura de Agentic Skills:**
   Consultar y respetar estrictamente las habilidades en `.aipods/skills/` (especialmente `sdd-spec-writer`, `ui-layout-governance`, `core-go-architect` y `frontend-ui-architect`).

2. **Lectura de la Guía de Onboarding:**
   Leer el archivo [`DEVELOPER_ONBOARDING_AND_GIT_WORKFLOW.md`](file:///home/martin/server/aipods-docs/DEVELOPER_ONBOARDING_AND_GIT_WORKFLOW.md).

3. **Paso 0 Obligatorio para GitHub Issues:**
   Ninguna especificación o cambio de código se realiza sin la previa creación o existencia de un **GitHub Issue oficial** asignado mediante las plantillas dinámicas YAML en `.github/ISSUE_TEMPLATE/` y con las etiquetas obligatorias (`--label "feature,needs-spec"` o `--label "bug"`).

4. **Prohibición Absoluta de Rutas Hardcoded:**
   Jamás escribir rutas locales fijas (ej. `/home/martin/...`). Usar siempre rutas relativas portables.
