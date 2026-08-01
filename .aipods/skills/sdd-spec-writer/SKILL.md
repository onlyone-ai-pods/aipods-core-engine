---
name: sdd-spec-writer
description: Guía para redactar y auditar especificaciones ejecutables bajo la metodología Spec-Driven Development (SDD) y el Protocolo de Consolidación 3-Tier.
---

# 📜 Skill: Spec-Driven Development (SDD) Writer

Esta habilidad instruye a asistentes de IA en la redacción y mantenimiento de especificaciones ejecutables `.spec.md`:

## 1. Precedencia de la Especificación
Ningún código Go o React se escribe sin una especificación `.spec.md` previa dentro de `/specs/active/`.

## 2. Ciclo de Vida de Especificaciones (Protocolo Consolidado 3-Tier - ISO/IEC 26514)

1. **Specs Activas Temporales (`IN_PROGRESS`)**:
   Toda spec en desarrollo se crea en `specs/active/XX_feature_spec.md`.

2. **Consolidación en Documento Maestro (`CLOSED`)**:
   Una vez que el código se audita, testea al 100% y se cierra el Issue en GitHub:
   - FUSIONAR la spec como sub-capítulo etiquetado (`# SPEC-CORE-XX`) en el Documento Maestro correspondiente (`01_CORE`, `02_SECURITY`, `03_ADMIN_HUB` o `04_CUSTOMER_PORTAL`).
   - ACTUALIZAR la tabla navegable en `specs/SPEC_MASTER_INDEX.md`.
   - ELIMINAR el archivo temporal de `specs/active/`.

## 3. Estructura Estándar de una Spec
- **Metadatos:** ID (`SPEC-CORE-XX`), Épica Relacionada, Estado.
- **Visión General & Objetivos.**
- **Contratos de Datos / Schemas JSON.**
- **Escenarios BDD Executables (`Given-When-Then`).**

## 4. Gobernanza Git
Las especificaciones se integran mediante Spec PRs en ramas `spec/` antes de abrir Code PRs en ramas `feat/`.
