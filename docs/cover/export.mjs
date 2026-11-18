// Renders the cover at 2x with a transparent page (so the stage's rounded
// corners stay clear) and encodes it to ../../assets/gitsy-cover.webp.
// Needs Google Chrome (or CHROME_PATH) and ffmpeg with libwebp.
import { execFileSync } from 'node:child_process'
import { mkdtempSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { dirname, join, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { chromium } from 'playwright-core'
import { createServer } from 'vite'

const here = dirname(fileURLToPath(import.meta.url))
const out = resolve(here, '../../assets/gitsy-cover.webp')
const work = mkdtempSync(join(tmpdir(), 'gitsy-cover-'))
const png = join(work, 'cover.png')

const server = await createServer({ root: here, configFile: join(here, 'vite.config.ts'), server: { port: 0 }, logLevel: 'error' })
await server.listen()
const url = server.resolvedUrls?.local[0]
const path = process.env.CHROME_PATH
const browser = await chromium.launch(path ? { executablePath: path } : { channel: 'chrome' })
try {
  if (!url) throw new Error('vite did not report a local URL')
  const page = await browser.newPage({ viewport: { width: 1100, height: 700 }, deviceScaleFactor: 2 })
  await page.goto(url)
  const stage = page.locator('.ss-stage').first()
  await stage.waitFor()
  await page.evaluate(() => document.fonts.ready)
  await stage.screenshot({ path: png, omitBackground: true, animations: 'disabled' })
  execFileSync('ffmpeg', ['-y', '-loglevel', 'error', '-i', png, '-c:v', 'libwebp', '-quality', '92', '-compression_level', '6', out])
  console.log(`wrote ${out}`)
} finally {
  await browser.close()
  await server.close()
  rmSync(work, { recursive: true, force: true })
}
