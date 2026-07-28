const { chromium } = require('/tmp/pw/node_modules/playwright');

async function consultarMisRetenciones(options = {}) {
  const {
    cuit = '20262534538',
    clave = 'Matias34**',
    fechaDesde = '01/01/2026',
    fechaHasta = '28/07/2026',
    headless = true
  } = options;

  console.log(`[RPA ARCA] Consulta Mis Retenciones (Mirequa) ${fechaDesde} a ${fechaHasta}...`);
  const browser = await chromium.launch({
    headless: headless,
    args: ['--no-sandbox', '--disable-dev-shm-usage', '--disable-gpu']
  });

  const context = await browser.newContext({
    userAgent: 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36',
    viewport: { width: 1280, height: 900 }
  });

  const page = await context.newPage();

  try {
    // 1. Login
    await page.goto('https://auth.afip.gob.ar/contribuyente_/login.xhtml', { waitUntil: 'networkidle', timeout: 30000 });
    await page.locator('id=F1:username').fill(cuit);
    await page.locator('id=F1:btnSiguiente').click();
    await page.waitForTimeout(1500);

    await page.locator('id=F1:password').fill(clave);
    await page.locator('id=F1:btnIngresar').click();
    await page.waitForTimeout(4000);

    // 2. Abrir servicio "Mis Retenciones"
    console.log('[RPA ARCA] Buscando servicio "Mis Retenciones"...');
    const pagePromise = context.waitForEvent('page', { timeout: 10000 }).catch(() => null);

    const searchInput = page.locator('input[placeholder*="Buscador"], #buscadorInput, input[type="search"]').first();
    await searchInput.fill('Mis Retenciones');
    await page.waitForTimeout(1000);

    const searchResult = page.locator('a.dropdown-item:has-text("Mis Retenciones"), li:has-text("Mis Retenciones") a').first();
    await searchResult.click();

    let servicePage = (await pagePromise) || page;
    await servicePage.waitForLoadState('networkidle').catch(() => {});
    await servicePage.waitForTimeout(3000);

    console.log(`[RPA ARCA] Servicio Mirequa abierto. URL: ${servicePage.url()}`);

    // 3. Seleccionar Impuesto si hay desplegable
    const dropdownImpuesto = servicePage.locator('div:has-text("Seleccioná tu impuesto retenido"), button:has-text("Seleccioná tu impuesto")').first();
    if (await dropdownImpuesto.isVisible().catch(() => false)) {
      console.log('[RPA ARCA] Desplegando menú de impuestos...');
      await dropdownImpuesto.click().catch(() => {});
      await servicePage.waitForTimeout(1000);
      const firstOption = servicePage.locator('li[role="option"], div.ng-option, option').first();
      if (await firstOption.isVisible().catch(() => false)) {
        await firstOption.click().catch(() => {});
      }
    }

    // 4. Llenar fechas desde y hasta
    console.log('[RPA ARCA] Ingresando fechas...');
    const inputDesde = servicePage.locator('#datePickerFechasRetencionesDesde__input, input[name*="fechaDesde"], input[placeholder*="Desde"]').first();
    const inputHasta = servicePage.locator('#datePickerFechasRetencionesHasta__input, input[name*="fechaHasta"], input[placeholder*="Hasta"]').first();

    if (await inputDesde.isVisible({ timeout: 3000 }).catch(() => false)) {
      await inputDesde.fill(fechaDesde);
    }
    if (await inputHasta.isVisible({ timeout: 3000 }).catch(() => false)) {
      await inputHasta.fill(fechaHasta);
    }

    // 5. Presionar Consultar
    console.log('[RPA ARCA] Presionando botón Consultar...');
    const btnConsultar = servicePage.locator('button:has-text("Consultar"), input[value="Consultar"]').first();
    if (await btnConsultar.isVisible().catch(() => false)) {
      await btnConsultar.click();
      await servicePage.waitForTimeout(4000);
    }

    // Extraer resultados
    const resultData = await servicePage.evaluate(() => {
      const text = document.body ? document.body.innerText : '';
      const rows = Array.from(document.querySelectorAll('table tbody tr')).map(tr => {
        return Array.from(tr.querySelectorAll('td, th')).map(td => td.innerText.trim());
      });
      return {
        snippet: text.substring(0, 800).replace(/\n+/g, ' '),
        rows
      };
    });

    console.log('[RPA ARCA] Resultado Mis Retenciones:');
    console.log('- Total filas:', resultData.rows.length);
    console.log('- Snippet:', resultData.snippet);

    const screenshotPath = '/tmp/mis_retenciones_result.png';
    await servicePage.screenshot({ path: screenshotPath, fullPage: true });
    console.log(`[RPA ARCA] Evidencia guardada en: ${screenshotPath}`);

    return {
      status: 'SUCCESS',
      rowCount: resultData.rows.length,
      rows: resultData.rows,
      screenshot: screenshotPath
    };
  } catch (err) {
    console.error('[RPA ARCA] Error:', err.message);
    return { status: 'ERROR', error: err.message };
  } finally {
    await browser.close();
    console.log('[RPA ARCA] Sesión finalizada.');
  }
}

if (require.main === module) {
  consultarMisRetenciones();
}

module.exports = { consultarMisRetenciones };
