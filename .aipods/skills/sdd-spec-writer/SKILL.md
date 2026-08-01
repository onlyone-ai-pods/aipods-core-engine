---
name: sdd-spec-writer
description: Guía para redactar y auditar especificaciones ejecutables bajo la metodología Spec-Driven Development (SDD) y el Protocolo de Consolidación 3-Tier.
---

# 📜 Skill: Spec-Driven Development (SDD) Writer

Esta habilidad instruye a asistentes de IA en la redacción y mantenimiento de especificaciones ejecutables `.spec.md`:

## 1. Paso 0: Creación Obligatoria de GitHub Issue
Ningún código Go o React ni especificación se escribe sin la previa creación de un **GitHub Issue oficial (Paso 0)** utilizando las plantillas dinámicas YAML en `.github/ISSUE_TEMPLATE/` (`01_feature_request.yml` o `02_bug_report.yml`).

## 2. Precedencia de la Especificación
Ningún código Go o React se escribe sin una especificación `.spec.md` previa dentro de `/specs/active/` en estado `IN_PROGRESS`.

## 3. Ciclo de Vida de Especificaciones (Protocolo Consolidado 3-Tier - ISO/IEC 26514)

1. **Specs Activas Temporales (`IN_PROGRESS`)**:
   Toda spec en desarrollo se crea en `specs/active/XX_feature_spec.md` con sintaxis KaTeX validada (0 errores LaTeX).

2. **Consolidación en Documento Maestro (`CLOSED`)**:
   Una vez que el código se audita, testea al 100% y se cierra el Issue en GitHub (`gh issue close`):
   - FUSIONAR la spec como sub-capítulo etiquetado (`# SPEC-CORE-XX`) en el Documento Maestro correspondiente (`01_CORE`, `02_SECURITY`, `03_ADMIN_HUB` o `04_CUSTOMER_PORTAL`).
   - ACTUALIZAR la tabla navegable en `specs/SPEC_MASTER_INDEX.md` y `docs/TROUBLESHOOTING_GUIDE.md`.
   - ELIMINAR el archivo temporal de `specs/active/`.

## 4. Gobernanza Git
Las especificaciones se integran mediante Spec PRs en ramas `spec/` antes de abrir Code PRs en ramas `feat/`.
