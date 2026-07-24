# 📜 SPEC: Gobernanza con GitHub CLI (`gh`), Convención de Ramas y Estructura Multi-Repo
**ID:** SPEC-CORE-19  
**Épica Relacionada:** Gobernanza de Código, GitHub Branch Protection, CI/CD Gates & Ecosistema Multi-Repo  
**Estado:** PROPOSED / SPEC-DRIVEN  

---

## 1. Visión y Objetivos

Esta especificación establece la gobernanza operacional del código fuente mediante el uso obligatorio de **GitHub CLI (`gh`)** dentro de la Organización **`onlyone-ai-pods`**, la convención estándar de nombres de ramas, y el flujo de **Spec PRs como compuerta previa al código**.

---

## 2. Convención Estándar de Nombres de Ramas (Conventional Branch Naming)

Toda rama creada en cualquiera de los repositorios de la organización `onlyone-ai-pods` DEBE cumplir con el formato:

$$\text{Formato: } \mathtt{<tipo>/<id-o-modulo>-<descripcion-corta-kebab-case>}$$

| Prefijo de Rama | Propósito & Uso en el Proyecto | Ejemplo de Nombre de Rama |
| :--- | :--- | :--- |
| **`spec/`** | **Fase A - Spec PR:** Creación o edición de especificaciones `.spec.md`. | `spec/01-smart-router` |
| **`feat/`** | **Fase B - Code PR:** Nuevas características en Go o React respaldadas por una Spec. | `feat/01-go-smart-router-impl` |
| **`fix/`** | Corrección de bugs o errores de código. | `fix/02-qdrant-timeout` |
| **`refactor/`** | Reestructuración de código sin alterar comportamiento. | `refactor/04-semantic-cache` |
| **`docs/`** | Actualización de documentación general (`README`, `SDD.md`). | `docs/update-onboarding-guide` |
| **`ci/`** | Workflows de GitHub Actions, linters o scripts. | `ci/setup-gosec-linter` |

---

## 3. Uso Obligatorio de GitHub CLI (`gh`) para el Equipo Interno

Para los desarrolladores internos del núcleo, el uso de **GitHub CLI (`gh`)** es obligatorio para gestionar el ciclo de desarrollo desde la terminal:

```bash
# 1. Crear rama siguiendo el estándar
git checkout -b spec/01-smart-router

# 2. Crear Pull Request de Especificación usando gh CLI
gh pr create --title "[SPEC] Add spec 01_smart_router" --body "Spec review" --base main

# 3. Monitorear el estado de GitHub Actions CI
gh pr checks

# 4. Hacer merge de la Spec una vez aprobada
gh pr merge --squash --delete-branch
```

---

## 4. Compuerta Estricta: Spec PR Pre-Code Gate & Branch Protection

* **Prohibido Push Directo:** La rama `main` en GitHub tiene activadas **Branch Protection Rules**. Ningún desarrollador puede hacer `git push origin main` directamente.
* **Secuencia Obligatoria de PRs:**
  1. **Fase A (Spec PR):** Se crea un PR modificando únicamente especificaciones en `/specs/` usando el prefijo `spec/`.
  2. **Fase B (Code PR):** Tras integrarse la Spec a `main`, el desarrollador crea una rama `feat/` para abrir el PR de código Go/React referenciando la Spec aprobada.

---

## 5. Gobernanza Dual: Pods Internos vs Pods Externos de Clientes

```text
+-----------------------------------------------------------------------------------+
| 1. AI PODS INTERNOS (Equipo Core):                                                |
|    - Desarrollados en repositorios de la organización `onlyone-ai-pods`.           |
|    - Gobernados por Spec PRs con `gh` CLI y Branch Protection Rules.             |
|                                                                                   |
| 2. AI PODS EXTERNOS (Clientes / Desarrolladores de Terceros):                     |
|    - Desarrollados en repositorios independientes usando `aipod-cli`.              |
|    - NO tienen permisos de escritura ni acceso a la rama `main`.                  |
|    - Sometidos vía API (/api/v1/admin/plugins/submit) a un Sandbox Aislado.        |
+-----------------------------------------------------------------------------------+
```
