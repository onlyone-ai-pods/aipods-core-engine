# 🔒 GitHub CLI & Spec PR Governance Rules for AI Assistants

Al asistir en el desarrollo de este proyecto, todo asistente de IA DEBE obedecer estrictamente estas reglas:

1. **PROHIBIDO PUSH DIRECTO A MAIN:**
   - NUNCA sugieras ni intentes ejecutar `git push origin main`. La rama `main` está protegida.

2. **CONVENCIÓN DE NOMBRES DE RAMAS (Conventional Branch Naming):**
   Toda rama DEBE seguir la estructura `<tipo>/<id>-<descripcion-kebab-case>`:
   - `spec/`     : Para PRs de especificaciones `.spec.md` (ej: `spec/01-smart-router`)
   - `feat/`     : Para PRs de características de código Go/React (ej: `feat/01-go-router-impl`)
   - `fix/`      : Para corrección de bugs (ej: `fix/02-qdrant-timeout`)
   - `refactor/` : Para reestructuración de código o specs (ej: `refactor/04-semantic-cache`)
   - `docs/`     : Para actualización de documentación general (ej: `docs/update-readme`)
   - `ci/`       : Para cambios en GitHub Actions o linters (ej: `ci/setup-gosec`)

3. **SECUENCIA OBLIGATORIA SPEC PR -> CODE PR:**
   - Todo cambio de código debe tener antes una especificación aprobada en `/specs/`.
   - La especificación debe enviarse primero mediante un Spec PR usando GitHub CLI:
     `gh pr create --title "[SPEC] ..." --body "..." --base main`

4. **VERIFICACIÓN DE CI CON GH CLI:**
   - Verifica el estado de las pruebas automatizadas y linters usando:
     `gh pr checks`

5. **ESTÁNDAR DE COMMITS DE ODOO:**
   - Todos los commits deben seguir el formato `[TAG] module: summary` (`[ADD]`, `[IMP]`, `[FIX]`, `[REF]`).
