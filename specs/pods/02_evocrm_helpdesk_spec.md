# 📜 SPEC: AI Pod - EvoCRM & Helpdesk
**ID:** SPEC-POD-02  
**Épica Relacionada:** Épica 2 (HU 2.1, HU 2.2)  
**Estado:** DRAFT / READY FOR IMPL  

---

## 1. Objetivo del Pod
Asistir a integradores de sistemas y agentes de soporte en la integración entre Odoo Helpdesk y EvoCRM (WhatsApp Omnicanal).

---

## 2. Validador de Webhooks JSON (Tool Specification)

El Pod debe contar con capacidad para validar la estructura del payload que envía EvoCRM al webhook de Odoo Helpdesk:

### Estructura JSON Esperada de EvoCRM:
```json
{
  "event": "messages.upsert",
  "instance": "string",
  "data": {
    "key": {
      "remoteJid": "string",
      "fromMe": "boolean",
      "id": "string"
    },
    "message": {
      "conversation": "string"
    }
  }
}
```

---

## 3. Escenarios BDD de Aceptación

### Escenario 1: Asistencia en Configuración de Webhook EvoCRM
```gherkin
Given un integrador consultando sobre la URL de webhook en Odoo Helpdesk
When interactúa con el Pod EvoCRM
Then el Pod debe proveer la URL base de integración `https://mi-odoo.com/evocrm/webhook`
And proveer el JSON de prueba para validar la conexión desde el panel de EvoCRM
```

### Escenario 2: Capacitación sobre Envíos Masivos
```gherkin
Given un agente de soporte preguntando "¿Cómo creo un envío masivo de WhatsApp a un grupo?"
When el Pod atiende la pregunta
Then debe indicar paso a paso la navegación en el módulo de EvoCRM
And advertir sobre las políticas de anti-spam y rate limits de WhatsApp API
```
