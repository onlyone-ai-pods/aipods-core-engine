# 📜 SPEC: AI Pod - Cadena de Suministros (SCM: WMS / MRP / Compras)
**ID:** SPEC-POD-04  
**Épica Relacionada:** Épica 4 (HU 4.1, HU 4.2, HU 4.3)  
**Estado:** DRAFT / READY FOR IMPL  

---

## 1. Objetivo del Pod
Consultoría avanzada en configuración logistica de Odoo: WMS (reglas Push/Pull), MRP (estructuras de listas de materiales BoM) y Compras (Landed Costs y costo promedio/estándar).

---

## 2. Escenarios BDD de Aceptación

### Escenario 1: WMS - Configuración de Reglas Push/Pull
```gherkin
Given un usuario con un flujo logístico de 3 pasos (Recepción -> Control de Calidad -> Stock)
When solicita ayuda al Pod SCM
Then el Pod debe guiarlo para activar "Rutas de Varios Pasos" en Ajustes de Inventario
And especificar la configuración exacta de las reglas Push/Pull y las ubicaciones virtuales correspondientes
```

### Escenario 2: MRP - Recomendación de Lista de Materiales (BoM)
```gherkin
Given un ingeniero de producción consultando sobre la fabricación de un kit promocional que no requiere orden de trabajo
When consulta al Pod MRP
Then el Pod debe recomendar el tipo de Lista de Materiales "Kit" (Phantom BoM)
And explicar la diferencia contable contra "Fabricar este producto"
```

### Escenario 3: Compras - Configuración de Landed Costs
```gherkin
Given un gerente de compras que necesita imputar 500 USD de flete marítimo sobre una recepción de 1000 unidades
When consulta sobre el procedimiento en Odoo
Then el Pod debe explicar cómo habilitar Landed Costs en Ajustes de Compras
And detallar el mecanismo de prorrateo (por Cantidad, por Valor o por Peso)
```
