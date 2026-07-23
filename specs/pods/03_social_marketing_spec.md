# 📜 SPEC: AI Pod - Odoo Social Marketing
**ID:** SPEC-POD-03  
**Épica Relacionada:** Épica 3 (HU 3.1)  
**Estado:** DRAFT / READY FOR IMPL  

---

## 1. Objetivo del Pod
Diagnosticar y resolver errores comunes de autenticación, expiración de tokens de acceso y permisos en la publicación desde Odoo Social Marketing hacia Meta (Instagram/Facebook).

---

## 2. Escenarios BDD de Aceptación

### Escenario 1: Troubleshooting de Token Meta Expirado
```gherkin
Given un usuario reportando "Mi publicación en Instagram falló con el error OAuthException code 190"
When el Pod Social Marketing procesa la duda
Then debe identificar que el código 190 corresponde a un Token de Acceso Expirado
And debe proveer la guía paso a paso para reconectar la cuenta en Odoo Social Marketing > Cuentas > Reconectar Meta
```
