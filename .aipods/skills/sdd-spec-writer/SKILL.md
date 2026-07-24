---
name: sdd-spec-writer
description: Guía para redactar y auditar especificaciones ejecutables bajo la metodología Spec-Driven Development (SDD).
---

# 📜 Skill: Spec-Driven Development (SDD) Writer

Esta habilidad instruye a asistentes de IA en la redacción y mantenimiento de especificaciones ejecutables `.spec.md`:

## Reglas de Redacción SDD

1. **Precedencia de la Especificación:**
   Ningún código Go o React se escribe sin una especificación `.spec.md` previa dentro de `/specs/`.

2. **Estructura Estándar de una Spec:**
   - **Metadatos:** ID, Épica Relacionada, Estado.
   - **Visión General & Objetivos.**
   - **Contratos de Datos / Schemas JSON.**
   - **Escenarios BDD Executables (`Given-When-Then`).**

3. **Gobernanza Git:**
   Las especificaciones se integran mediante Spec PRs en ramas `spec/` antes de abrir Code PRs en ramas `feat/`.
