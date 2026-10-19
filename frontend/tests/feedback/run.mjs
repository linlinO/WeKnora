// Run with Node >= 22; PLAYWRIGHT_MODULE may point at a bundled Playwright.
import assert from 'node:assert/strict'
import { createRequire } from 'node:module'
import { mkdir } from 'node:fs/promises'
import { resolve } from 'node:path'
import { createServer } from 'vite'

const require = createRequire(import.meta.url)
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright')
const artifacts = resolve(process.env.FEEDBACK_TEST_ARTIFACTS || 'test-results/feedback')
await mkdir(artifacts, { recursive: true })
const server = await createServer({ server: { host: '127.0.0.1', port: 0, open: false } })
let browser
try {
  await server.listen()
  const origin = `http://127.0.0.1:${server.httpServer.address().port}`
  browser = await chromium.launch({ headless: true, channel: process.env.PLAYWRIGHT_CHANNEL || undefined })
  const page = await browser.newPage()
  const errors = []
  page.on('pageerror', error => errors.push(error.message))
  let vote = null, name = ''
  const writes = []
  await page.route('**/api/v1/**', async route => {
    const request = route.request(), url = new URL(request.url())
    let data
    if (url.pathname.includes('/reporter-name')) {
      name = request.postDataJSON().name
      data = { success: true, data: { user_name: name } }
    } else if (url.pathname.includes('/admin/answer-feedback')) {
      const records = [
        { id: 'im', source: 'wecom', user_id: 'wecom:opaque-id', channel_id: 'channel', user_name: name, status: 'pending', rating: 'not_useful', question: '服务器负载高？', answer: '旧回答', reason: '内容有错误', created_at: '2026-09-23T02:05:40Z' },
        { id: 'web', source: 'web', user_id: 'uuid', user_name: '测试用户', status: 'pending', rating: 'useful', question: '怎么安装？', answer: '安装步骤', created_at: '2026-09-23T02:05:05Z' },
      ]
      const id = url.pathname.split('/admin/answer-feedback/')[1]
      const filtered = records.filter(row => !url.searchParams.get('rating') || url.searchParams.get('rating') === 'all' || row.rating === url.searchParams.get('rating'))
      data = id ? { data: records.find(row => row.id === id), proposal: null, regression: null, references: [] } : { data: filtered, total: filtered.length }
    } else if (url.pathname.includes('/answer-feedback/')) {
      if (request.method() === 'POST') {
        const payload = request.postDataJSON()
        writes.push(payload)
        vote = { ...payload, rating: payload.rating === 'cancel' ? '' : payload.rating }
      }
      data = { data: vote }
    } else if (url.pathname.includes('/knowledge-bases')) {
      data = { data: [{ id: 'kb', name: 'XBROTHER' }] }
    } else { data = { data: [] } }
    await route.fulfill({ json: data })
  })
  for (const [width, height] of [[1280, 800], [390, 844], [320, 640], [844, 360]]) {
    vote = null
    await page.setViewportSize({ width, height })
    await page.goto(`${origin}/tests/feedback/index.html`)
    await page.getByRole('button', { name: '无用', exact: true }).click()
    const dialog = page.locator('.t-dialog:visible')
    await dialog.waitFor()
    await page.getByText('内容有错误', { exact: true }).click()
    await page.getByPlaceholder('补充说明（选填）').fill('请按新安装步骤修订')
    await page.screenshot({ path: resolve(artifacts, `dialog-${width}x${height}.png`), fullPage: true })
    const bounds = await dialog.boundingBox()
    if (bounds.y < 0 || bounds.height > height) console.error(await dialog.evaluate(el => ({
      maxHeight: getComputedStyle(el).maxHeight, display: getComputedStyle(el).display,
      context: el.closest('.t-dialog__ctx').outerHTML.slice(0, 400),
    })))
    assert(bounds.x >= 0 && bounds.y >= 0 && bounds.x + bounds.width <= width + 1 && bounds.y + bounds.height <= height + 1, JSON.stringify(bounds))
    assert.equal(await dialog.evaluate(el => !!el.closest('.answer-toolbar, .chat-scroll')), false)
    for (const label of ['提交说明', '暂不补充']) {
      const button = dialog.getByRole('button', { name: label, exact: true })
      const size = await button.boundingBox()
      assert(size.width >= 88 && size.height >= 32, `${label}: ${JSON.stringify(size)}`)
      assert(size.y + size.height <= height, 'footer clipped')
      assert.equal(await button.evaluate(el => el.scrollWidth <= el.clientWidth + 1), true, 'button text overflow')
    }
    await dialog.getByRole('button', { name: '提交说明', exact: true }).click()
    await dialog.waitFor({ state: 'hidden' })
    assert.equal(vote.reason, '内容有错误；请按新安装步骤修订')
    await page.reload()
    const selected = page.locator('.feedback-vote[title="取消无用"]')
    await selected.waitFor()
    assert.equal(await selected.getAttribute('aria-pressed'), 'true')
    await selected.click()
    await page.waitForFunction(() => document.querySelectorAll('.feedback-vote[aria-pressed="true"]').length === 0)
    assert.equal(vote.rating, '')
  }
  for (const [width, height] of [[1280, 900], [390, 844]]) {
    await page.setViewportSize({ width, height })
    await page.goto(`${origin}/tests/feedback/index.html?settings`)
    await page.getByRole('row').filter({ hasText: '服务器负载高？' }).getByRole('button', { name: '查看', exact: true }).click()
    await page.getByRole('button', { name: '备注姓名' }).click()
    const dialog = page.locator('.t-dialog:visible')
    await dialog.getByPlaceholder('姓名', { exact: true }).fill('测试同事')
    await dialog.getByRole('button', { name: '保存', exact: true }).click()
    await dialog.waitFor({ state: 'hidden' })
    await page.locator('.reporter-row').getByText('测试同事', { exact: true }).waitFor()
    assert.equal(name, '测试同事')
    await page.reload()
    await page.getByText('测试同事', { exact: true }).waitFor()
    assert.equal(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), true)
    await page.screenshot({ path: resolve(artifacts, `review-${width}x${height}.png`), fullPage: true })
  }
  assert.equal(errors.length, 0, errors.join('\n'))
  console.log(`Feedback UI checks passed, ${writes.length} vote submissions; screenshots: ${artifacts}`)
} finally {
  if (browser) await browser.close()
  await server.close()
}
