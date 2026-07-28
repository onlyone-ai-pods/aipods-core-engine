const { chromium } = require('/tmp/pw/node_modules/playwright');

async function consultarPuntosDeVenta(options = {}) {
  const {
    cuit = '20262534538',
    clave = 'Matias34**',
    headless = true
  } = options;

  console.log(`[RPA ARCA] Iniciando consulta de Puntos de Venta para CUIT: ${cuit}...`);
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

    // 2. Buscar el servicio "Administración de Puntos de Venta y Domicilios"
    console.log('[RPA ARCA] Buscando servicio "Administración de Puntos de Venta y Domicilios"...');
    const pagePromise = context.waitForEvent('page', { timeout: 10000 }).catch(() => null);

    const searchInput = page.locator('input[placeholder*="Buscador"], #buscadorInput, input[type="search"]').first();
    await searchInput.fill('Puntos de Venta');
    await page.waitForTimeout(1000);

    const searchResult = page.locator('a.dropdown-item:has-text("Puntos de Venta"), li:has-text("Puntos de Venta") a').first();
    if (await searchResult.isVisible().catch(() => false)) {
      await searchResult.click();
    } else {
      console.log('[RPA ARCA] Buscando con término alternativo "Regímenes de Facturación"...');
      await searchInput.fill('Regímenes de Facturación');
      await page.waitForTimeout(1000);
      const altResult = page.locator('a.dropdown-item:has-text("Puntos de Venta"), li:has-text("Regímenes") a').first();
      await altResult.click();
    }

    let servicePage = (await pagePromise) || page;
    await servicePage.waitForLoadState('networkidle').catch(() => {});
    await servicePage.waitForTimeout(3000);

    console.log(`[RPA ARCA] Servicio abierto. URL: ${servicePage.url()}`);
    console.log(`[RPA ARCA] Título: ${await servicePage.title()}`);

    // Extraer texto inicial
    const initialText = await servicePage.evaluate(() => document.body ? document.body.innerText : '');
    console.log('[RPA ARCA] Snippet servicio:\n', initialText.substring(0, 600).replace(/\n+/g, ' '));

    // Si hay botón de Selección de Empresa / Contribuyente
    const btnEmpresa = servicePage.locator('input[type="button"][value*="LLANOS"], input[value*="Ingresar"], button:has-text("Seleccionar")').first();
    if (await btnEmpresa.isVisible().catch(() => false)) {
      console.log('[RPA ARCA] Seleccionando empresa/contribuyente...');
      await btnEmpresa.click();
      await servicePage.waitForTimeout(2000);
    }

    // Buscar la opción "A/B/M de Puntos de Venta"
    const abmPvOption = servicePage.locator('a:has-text("Puntos de Venta"), button:has-text("Puntos de Venta"), input[value*="Puntos de Venta"]').first();
    if (await abmPvOption.isVisible().catch(() => false)) {
      console.log('[RPA ARCA] Ingresando a A/B/M de Puntos de Venta...');
      await abmPvOption.click();
      await servicePage.waitForTimeout(3000);
    }

    // Extraer tabla de Puntos de Venta
    const pvData = await servicePage.evaluate(() => {
      const rows = Array.from(document.querySelectorAll('table tbody tr')).map(tr => {
        return Array.from(tr.querySelectorAll('td, th')).map(td => td.innerText.trim());
      });
      return {
        textSnippet: document.body ? document.body.innerText.substring(0, 800).replace(/\n+/g, ' ') : '',
        rowCount: rows.length,
        rows
      };
    });

    console.log('[RPA ARCA] Puntos de Venta encontrados:');
    console.log('- Cantidad de filas:', pvData.rowCount);
    console.log('- Texto snippet:', pvData.textSnippet);
    if (pvData.rows.length > 0) {
      console.log('- Detalle de Puntos de Venta:', JSON.stringify(pvData.rows, null, 2));
    }

    const screenshotPath = '/tmp/puntos_de_venta_result.png';
    await servicePage.screenshot({ path: screenshotPath, fullPage: true });
    console.log(`[RPA ARCA] Screenshot guardado en: ${screenshotPath}`);

    return {
      status: 'SUCCESS',
      pvCount: pvData.rowCount,
      pvList: pvData.rows,
      screenshot: screenshotPath
    };
  } catch (err) {
    console.error('[RPA ARCA] Error:', err.message);
    return { status: 'ERROR', error: err.message };
  } finally {
    await browser.close();
    console.log('[RPA ARCA] Navegador cerrado.');
  }
}

if (require.main === module) {
  consultarPuntosDeVenta();
}

module.exports = { consultarPuntosDeVenta };
