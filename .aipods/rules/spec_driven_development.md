# 📜 Regla Invariante: Spec-Driven Development (SDD) & Gates de Seguridad

Toda asistencia de IA en este proyecto DEBE cumplir estrictamente con las siguientes dos reglas antes y después de escribir cualquier línea de código:

## 1. Regla de Precedencia de Especificaciones (Pre-Code Spec Traceability)
- NINGÚN código en Go, React o Python puede ser escrito sin antes consultar la especificación ejecutable correspondiente en `specs/`.
- El código fuente DEBE coincidir al 100% con los esquemas JSON de herramientas, los escenarios BDD (`Given-When-Then`) y las estructuras definidas en las specs.

## 2. Regla Mandatoria de Linters & Chequeo de Seguridad (Post-Code Quality Gate)
Antes de declarar completada cualquier tarea de código Go en `aipods-core-engine`, el asistente de IA DEBE ejecutar automáticamente:
1. `go vet ./...` (Análisis estático de tipos y sintaxis)
2. `gosec ./...` (Escáner AST de vulnerabilidades de seguridad de Go)

Si cualquier linter o escáner de seguridad reporta advertencias o errores, el asistente DEBE corregirlos de inmediato antes de realizar cualquier commit o finalizar el turno.
