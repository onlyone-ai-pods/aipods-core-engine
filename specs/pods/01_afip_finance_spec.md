# 📜 SPEC: AI Pod - AFIP / ARCA & Finanzas
**ID:** SPEC-POD-01  
**Épica Relacionada:** Épica 1 (HU 1.1, HU 1.2)  
**Estado:** DRAFT / READY FOR IMPL  

---

## 1. Objetivo del Pod
Proveer asistencia técnica interactiva para la configuración de facturación electrónica en Odoo (Localización Argentina - AFIP/ARCA) y generar resúmenes ejecutivos de balances financieros.

---

## 2. Herramienta: Generador de Comandos OpenSSL

### 2.1 Prompt / Tool Specs
El Pod debe solicitar:
1. **CUIT** (11 dígitos sin guiones)
2. **Sistema Operativo** (`Linux`, `Windows`, `macOS`)

### 2.2 Salida Esperada Exacta
Para CUIT `20334445559` y SO `Linux`:
```bash
# 1. Generar la clave privada
openssl genrsa -out privada.key 2048

# 2. Generar la solicitud de certificado (CSR)
openssl req -new -key privada.key -subj "/C=AR/O=Empresa/CN=OdooAFIP/serialNumber=CUIT 20334445559" -out pedido.csr
```

---

## 3. Escenarios BDD de Aceptación

### Escenario 1: Solicitud interactiva de generación de CSR
```gherkin
Given un usuario pidiendo "Quiero generar mi certificado para AFIP en Odoo"
When el Pod AFIP atiende la consulta
Then debe responder pidiendo confirmación de su CUIT y el Sistema Operativo que utiliza
And una vez recibidos los datos, debe generar el bloque de código OpenSSL exacto
And debe listar los pasos para subir el archivo .csr al portal de ARCA/AFIP en "Administración de Certificados Digitales"
```

### Escenario 2: Asistencia en Análisis de Balance
```gherkin
Given un balance financiero exportado de Odoo en formato CSV o PDF
When el usuario sube el archivo al Pod AFIP & Finanzas
Then el Pod debe extraer los ingresos, egresos, EBITDA y margen operativo
And responder con un resumen estructurado en lenguaje natural indicando 3 puntos críticos
```
