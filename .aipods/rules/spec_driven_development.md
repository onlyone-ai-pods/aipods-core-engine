# 📜 Regla Invariante: Spec-Driven Development (SDD) & Gates de Seguridad

Toda asistencia de IA en este proyecto DEBE cumplir estrictamente con las siguientes reglas antes y después de escribir cualquier línea de código:

## 1. Regla de Precedencia de Especificaciones (Pre-Code Spec Traceability)
- NINGÚN código en Go, React o Python puede ser escrito sin antes consultar la especificación ejecutable correspondiente en `specs/`.
- El código fuente DEBE coincidir al 100% con los esquemas JSON de herramientas, los escenarios BDD (`Given-When-Then`) y las estructuras definidas en las specs.

## 2. Regla Mandatoria de Linters & Chequeo de Seguridad Backend Go (Post-Code Quality Gate)
Antes de declarar completada cualquier tarea de código Go en `aipods-core-engine`, el asistente de IA DEBE ejecutar automáticamente:
1. `go vet ./...` (Análisis estático de tipos y sintaxis)
2. `/home/martin/go/bin/gosec ./...` (Escáner AST de vulnerabilidades de seguridad de Go)
3. `go test -v ./...` (Pruebas unitarias y escenarios BDD)

## 3. Regla Mandatoria de Auditoría NPM & Linters Frontend React (Post-Code Quality Gate)
Antes de declarar completada cualquier tarea de código React 18 / Vite en `aipods-frontend-customer` o `aipods-frontend-admin`, el asistente de IA DEBE ejecutar automáticamente:
1. `npm audit` (Auditoría de vulnerabilidades de dependencias NPM)
2. `npx eslint "src/**/*.{js,jsx}"` (Análisis estático de código React y JSX)
3. `npm run build` (Verificación de compilación de producción)

Si cualquier linter o escáner de seguridad reporta advertencias o vulnerabilidades, el asistente DEBE corregirlos de inmediato antes de realizar cualquier commit o finalizar el turno.
