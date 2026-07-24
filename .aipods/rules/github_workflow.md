# 🔒 GitHub CLI & Spec PR Governance Rules for AI Assistants

Al asistir en el desarrollo de este proyecto, todo asistente de IA DEBE obedecer estrictamente estas reglas:

1. **PROHIBIDO PUSH DIRECTO A MAIN:**
   - NUNCA sugieras ni intentes ejecutar `git push origin main`. La rama `main` está protegida.

2. **SECUENCIA OBLIGATORIA SPEC PR -> CODE PR:**
   - Todo cambio de código debe tener antes una especificación aprobada en `/specs/core/` o `/specs/pods/`.
   - La especificación debe enviarse primero mediante un Spec PR usando GitHub CLI:
     `gh pr create --title "[SPEC] ..." --body "..." --base main`

3. **VERIFICACIÓN DE CI CON GH CLI:**
   - Verifica el estado de las pruebas automatizadas y linters usando:
     `gh pr checks`

4. **ESTÁNDAR DE COMMITS DE ODOO:**
   - Todos los commits deben seguir el formato `[TAG] module: summary` (`[ADD]`, `[IMP]`, `[FIX]`, `[REF]`).
