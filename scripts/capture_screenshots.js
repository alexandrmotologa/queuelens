const { spawn } = require('child_process');
const path = require('path');
const puppeteer = require('B:/workgit/tlslens/web/node_modules/puppeteer');

async function wait(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

async function run() {
  const binaryPath = path.resolve(__dirname, '../queuelens.exe');
  const port = '9195';

  console.log('Starting QueueLens demo server on port', port);
  const server = spawn(binaryPath, ['demo', '--port', port], {
    stdio: 'ignore',
    detached: false,
  });

  try {
    // Wait for server to bind & synthetic queue data to generate
    await wait(3000);

    const browser = await puppeteer.launch({
      headless: true,
      args: ['--no-sandbox', '--disable-setuid-sandbox'],
    });

    const page = await browser.newPage();
    const url = `http://localhost:${port}`;

    // 1. Desktop captures (1440x900)
    await page.setViewport({ width: 1440, height: 900, deviceScaleFactor: 2 });
    await page.goto(url, { waitUntil: 'networkidle2' });
    await wait(1500);

    const outDir = path.resolve(__dirname, '../docs/images');

    // Tab 1: Overview
    console.log('Capturing Desktop Tab 1 (Overview)...');
    await page.screenshot({ path: path.join(outDir, 'queuelens-1.png') });

    // Tab 2: Live Stream
    console.log('Capturing Desktop Tab 2 (Live Stream)...');
    await page.keyboard.press('2');
    await wait(1000);
    await page.screenshot({ path: path.join(outDir, 'queuelens-2.png') });

    // Tab 3: DLQ Studio
    console.log('Capturing Desktop Tab 3 (DLQ Studio)...');
    await page.keyboard.press('3');
    await wait(1000);
    await page.screenshot({ path: path.join(outDir, 'queuelens-3.png') });

    // Tab 4: Stalls
    console.log('Capturing Desktop Tab 4 (Stalls)...');
    await page.keyboard.press('4');
    await wait(1000);
    await page.screenshot({ path: path.join(outDir, 'queuelens-4.png') });

    // 2. Mobile Responsive Capture (iPhone 14 viewport: 390x844)
    console.log('Capturing Mobile Viewport (390x844)...');
    await page.setViewport({ width: 390, height: 844, deviceScaleFactor: 2, isMobile: true, hasTouch: true });
    await page.keyboard.press('1'); // back to overview
    await wait(1000);
    await page.screenshot({ path: path.join(outDir, 'queuelens-mobile.png') });

    await browser.close();
    console.log('✓ All responsive screenshots captured successfully.');
  } finally {
    server.kill();
  }
}

run().catch((err) => {
  console.error('Error during screenshot capture:', err);
  process.exit(1);
});
