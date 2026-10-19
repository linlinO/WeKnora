// Isolated browser fixture. No requests or writes reach a real WeKnora server.
import { createServer } from 'vite'
const rows = Array.from({ length: 47 }, (_, i) => ({ id: `qa-${i + 1}`, source: i % 2 ? 'web' : 'wecom', user_id: `user-${i}`, user_name: `测试用户${i + 1}`, status: 'pending', rating: i % 3 ? 'not_useful' : 'useful', question: `验收问题 ${i + 1}：如何检查服务器运行状态？`, answer: '## 原回答\n\n检查运行状态，并查看日志。', reason: '请补充检查步骤', created_at: '2026-10-10T02:00:00Z' }))
const proposals = new Map(), runs = new Map()
rows[0].question='安装器配置时，用户模式、隔离模式、数据库名、数据库地址、账号、密码、存储引擎、网关类型和是否快切应该如何填写？'.repeat(3)
rows[0].user_name=''
rows[0].user_id='woYSFLDQAAVeryLongUserIdentifierForEllipsisReview'
const server = await createServer({ plugins: [{ name: 'feedback-isolated-api', configureServer(server) {
  server.middlewares.use(async (req, res, next) => {
    if (!req.url?.startsWith('/api/v1/')) return next()
    const url = new URL(req.url, 'http://localhost'), p = url.pathname, q = url.searchParams
    let raw = ''; for await (const chunk of req) raw += chunk
    const body = raw ? JSON.parse(raw) : {}, parts = p.split('/'), id = parts.at(-1)
    let data = { data: [] }
    if (p === '/api/v1/admin/answer-feedback') {
      const base = rows.filter(r => (!q.get('rating') || q.get('rating') === 'all' || r.rating === q.get('rating')) && (!q.get('source') || r.source === q.get('source')) && r.question.includes(q.get('keyword') || '') && r.user_name.includes(q.get('reporter') || '') && (!q.get('from') || r.created_at >= q.get('from')) && (!q.get('to') || r.created_at < q.get('to')))
      const selected = base.filter(r => q.get('status') === 'all' || r.status === q.get('status')), size = Number(q.get('page_size')) || 20, page = Number(q.get('page')) || 1
      data = { data: selected.slice((page - 1) * size, page * size), total: selected.length, status_counts: { all: base.length } }
      for (const r of base) data.status_counts[r.status] = (data.status_counts[r.status] || 0) + 1
    } else if (p.includes('/admin/answer-feedback/')) {
      const row = rows.find(r => r.id === (['review', 'reporter-name'].includes(id) ? parts.at(-2) : id))
      if (id === 'review') row.status = { acknowledge: 'reviewed', correct: 'correcting', ignore: 'ignored' }[body.action]
      if (id === 'reporter-name') row.user_name = body.name
      data = { data: row, proposal: proposals.get(row?.id) || null, regression: runs.get(row?.id) || null, references: [] }
    } else if (p.includes('/knowledge-evolution/')) {
      const rowID = parts.at(-2), row = rows.find(r => r.id === rowID)
      let proposal = proposals.get(rowID)
      if (!proposal) { proposal = { id: rowID, feedback_id: rowID, candidate_answer: '## 标准答案\n\n1. 检查服务运行状态。\n2. 查看错误日志。', status: 'draft', conflict_status: 'none', knowledge_base_id: 'qa-kb' }; proposals.set(rowID, proposal); row.status = 'correcting' }
      if (body.answer) proposal.candidate_answer = body.answer
      if (id === 'regression') { const run = { id: 'run', status: 'passed', total: 1, passed: 1 }; runs.set(rowID, run); data = { data: run, proposal } }
      else { if (id === 'publish') { proposal.status = 'published'; row.status = 'resolved' } data = { data: proposal, knowledge_id: 'qa-knowledge' } }
    } else if (p.includes('knowledge-bases')) data = { data: [{ id: 'qa-kb', name: 'XBROTHER' }] }
    res.setHeader('Content-Type', 'application/json'); res.end(JSON.stringify(data))
  })
} }], server: { host: '127.0.0.1', port: 5188, strictPort: true, open: false } })
await server.listen()
console.log('Isolated feedback fixture: http://127.0.0.1:5188/tests/feedback/index.html?workbench')
