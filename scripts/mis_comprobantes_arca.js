const { chromium } = require('/tmp/pw/node_modules/playwright');

async function consultarComprobantes(options = {}) {
  const {
    cuit = '20262534538',
    clave = 'Matias34**',
    tipo = 'Emitidos', // 'Emitidos' o 'Recibidos'
    fechaDesde = '01/06/2026', // Formato DD/MM/YYYY para el datepicker de AFIP
    fechaHasta = '28/07/2026',
    headless = true
  } = options;

  console.log(`[RPA ARCA] Consulta de Comprobantes ${tipo} (${fechaDesde} a ${fechaHasta})`);
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

    // 2. Abrir servicio "Mis Comprobantes"
    const pagePromise = context.waitForEvent('page', { timeout: 10000 }).catch(() => null);
    const searchInput = page.locator('input[placeholder*="Buscador"], #buscadorInput, input[type="search"]').first();
    await searchInput.fill('Mis Comprobantes');
    await page.waitForTimeout(1000);
    const searchResult = page.locator('a.dropdown-item:has-text("Mis Comprobantes"), li:has-text("Mis Comprobantes") a').first();
    await searchResult.click();

    let servicePage = (await pagePromise) || page;
    await servicePage.waitForLoadState('networkidle').catch(() => {});
    await servicePage.waitForTimeout(2000);

    console.log(`[RPA ARCA] Portal Mis Comprobantes cargado. URL: ${servicePage.url()}`);

    // 3. Click en Emitidos o Recibidos
    if (tipo === 'Emitidos') {
      await servicePage.locator('#btnEmitidos').click();
    } else {
      await servicePage.locator('#btnRecibidos').click();
    }

    await servicePage.waitForTimeout(2500);
    console.log(`[RPA ARCA] Pantalla ${tipo} abierta. URL: ${servicePage.url()}`);

    // 4. Completar Rango de Fechas
    const datePicker = servicePage.locator('#fechaEmision, input[name="fechaEmision"]').first();
    if (await datePicker.isVisible().catch(() => false)) {
      console.log('[RPA ARCA] Abriendo selector de fechas...');
      await datePicker.click();
      await servicePage.waitForTimeout(500);

      // Si existe rango personalizado en daterangepicker
      const customRangeOption = servicePage.locator('li[data-range-key="Rango Personalizado"]').first();
      if (await customRangeOption.isVisible().catch(() => false)) {
        await customRangeOption.click();
      }

      // Llenar desde y hasta
      const inputStart = servicePage.locator('input[name="daterangepicker_start"]').first();
      const inputEnd = servicePage.locator('input[name="daterangepicker_end"]').first();
      if (await inputStart.isVisible().catch(() => false)) {
        await inputStart.fill(fechaDesde);
        await inputEnd.fill(fechaHasta);
        const applyBtn = servicePage.locator('button.applyBtn, button:has-text("Aplicar")').first();
        if (await applyBtn.isVisible().catch(() => false)) {
          await applyBtn.click();
        }
      }
    }

    // 5. Presionar Buscar
    console.log('[RPA ARCA] Presionando botón Buscar...');
    const searchBtn = servicePage.locator('button:has-text("Buscar"), input[type="submit"][value="Buscar"]').first();
    await searchBtn.click();

    await servicePage.waitForTimeout(4000);

    // 6. Extraer resultados
    const resultSummary = await servicePage.evaluate(() => {
      const infoBox = document.querySelector('.alert, .info, #detalles, table');
      const text = document.body ? document.body.innerText : '';
      
      // Buscar tablas de comprobantes
      const rows = Array.from(document.querySelectorAll('table tbody tr')).map(tr => {
        return Array.from(tr.querySelectorAll('td')).map(td => td.innerText.trim());
      });

      return {
        textSnippet: text.substring(0, 1000).replace(/\n+/g, ' '),
        rowCount: rows.length,
        sampleRows: rows.slice(0, 5)
      };
    });

    console.log('[RPA ARCA] Resumen de la búsqueda:');
    console.log('- Filas encontradas:', resultSummary.rowCount);
    console.log('- Texto snippet:', resultSummary.textSnippet);
    if (resultSummary.sampleRows.length > 0) {
      console.log('- Muestra de comprobantes:', JSON.stringify(resultSummary.sampleRows, null, 2));
    }

    // Screenshot final
    const screenshotPath = `/tmp/comprobantes_${tipo.toLowerCase()}_result.png`;
    await servicePage.screenshot({ path: screenshotPath, fullPage: true });
    console.log(`[RPA ARCA] Evidencia guardada en: ${screenshotPath}`);

    return {
      status: 'SUCCESS',
      tipo,
      rowCount: resultSummary.rowCount,
      sampleRows: resultSummary.sampleRows,
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
  consultarComprobantes({ tipo: 'Emitidos' });
}

module.exports = { consultarComprobantes };
