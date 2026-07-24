# 📜 SPEC: AI Pod - GitHub API & Odoo.sh DevOps Integrator
**ID:** SPEC-POD-05  
**Épica Relacionada:** Épica 11 (Automatización de Repositorios GitHub & Despliegues Odoo.sh)  
**Estado:** PROPOSED / SPEC-DRIVEN  

---

## 1. Visión General

Este AI Pod especializado se otorga **por defecto a todos los clientes** de la plataforma. Su objetivo es automatizar la interacción con la **API de GitHub** y la plataforma PaaS **Odoo.sh**, permitiendo crear repositorios de código para módulos personalizados, gestionar Pull Requests y coordinar despliegues automatizados.

---

## 2. Herramientas del Pod (Tools Schema)

### 2.1 Tool `crear_repositorio_modulo_github`
Permite crear un nuevo repositorio en la cuenta/organización de GitHub del cliente para alojar un módulo de Odoo:

```json
{
  "name": "crear_repositorio_modulo_github",
  "description": "Crea un repositorio GitHub en la cuenta del cliente para un nuevo módulo.",
  "parameters": {
    "type": "object",
    "properties": {
      "github_org": { "type": "string" },
      "repo_name": { "type": "string" },
      "is_private": { "type": "boolean", "default": true },
      "dry_run": {
        "type": "boolean",
        "default": true,
        "description": "MANDATORIO: Simula la creación del repositorio sin invocar la API de GitHub."
      }
    },
    "required": ["github_org", "repo_name", "dry_run"]
  }
}
```

### 2.2 Tool `vincular_despliegue_odoo_sh`
Vincula una rama Git del cliente con un entorno de Staging o Producción en Odoo.sh:

```json
{
  "name": "vincular_despliegue_odoo_sh",
  "description": "Despliega o vincula una rama Git en el entorno de Odoo.sh del cliente.",
  "parameters": {
    "type": "object",
    "properties": {
      "project_id": { "type": "string" },
      "branch_name": { "type": "string" },
      "environment_type": { "type": "string", "enum": ["staging", "production"] },
      "dry_run": { "type": "boolean", "default": true }
    },
    "required": ["project_id", "branch_name", "environment_type", "dry_run"]
  }
}
```

---

## 3. Estructuración Automática de Módulos Odoo

Cuando un cliente solicita crear un módulo (ej. *"Crea un módulo de Odoo para agregar el campo CUIT a los contactos"*), el Pod genera la estructura estándar:

```text
custom_module_name/
├── __manifest__.py
├── __init__.py
├── models/
│   ├── __init__.py
│   └── res_partner.py
├── views/
│   └── res_partner_views.xml
└── security/
    └── ir.model.access.csv
```

---

## 4. Escenario BDD de Integración GitHub & Odoo.sh

```gherkin
Given un cliente solicitando al Pod DevOps "Crea el repositorio github.com/empresa/odoo-custom-cuit y despliegalo en Odoo.sh staging"
When el AI Pod procesa la solicitud
Then el Pod DEBE invocar la herramienta crear_repositorio_modulo_github con dry_run=true
And mostrar la vista previa del repositorio y los archivos __manifest__.py que serán creados
And solicitar la confirmación explícita del usuario mediante el token "approval_token"
And tras la aprobación humana, invocar la API de GitHub y vincular la rama con Odoo.sh
```
