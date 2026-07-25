---
name: frontend-ui-architect
description: Guía experta de desarrollo web moderno en React 18, Vite y Vanilla CSS para AI Pods Enterprise.
---

# 🎨 Skill: Frontend UI Architect

Esta habilidad instruye a asistentes de IA para desarrollar aplicaciones web en **React 18 / Vite** en `aipods-frontend-customer` y `aipods-frontend-admin`:

## 1. Trazabilidad Estricta con Especificaciones SDD
Antes de crear o editar componentes React, consultar las especificaciones en `specs/04_customer_portal_growth/`. Garantizar el cumplimiento del paradigma "Servicio como Software", Sandbox interactivo y modales SSO.

## 2. Gate Mandatorio de Auditoría NPM & Seguridad (Post-Code Quality Gate)
Antes de finalizar cualquier tarea de código en React, el asistente DEBE ejecutar automáticamente:
```bash
npm audit
npx eslint "src/**/*.{js,jsx}"
npm run build
```
Si `npm audit` reporta vulnerabilidades en paquetes, ejecutar la remediación y asegurar `found 0 vulnerabilities`.

## 3. Principios de Diseño Visual
- **Aesthetic First:** Usar tokens de color HSL tailoreados, glassmorphism (`backdrop-filter: blur(12px)`), tipografía Inter y micro-animaciones en hover.
- **Vanilla CSS Puro:** Utilizar archivos `.css` estructurados, evitando ad-hoc utilities frágiles.
- **Sanitización & Clean Code:** Evitar `dangerouslySetInnerHTML` o patrones vulnerables a XSS.
