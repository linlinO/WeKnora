import assert from 'node:assert/strict'
import test from 'node:test'
import { feedbackListState, feedbackListQuery, feedbackScrollKey } from './feedbackListState'

test('list state survives detail navigation and reload without losing page or filters', () => {
  const state = { status: 'all', rating: 'not_useful', source: 'wecom', reporter: '方绍羽', keyword: '安装', start: '2026-09-01', end: '2026-10-10', page: 3, pageSize: 50 }
  assert.deepEqual(feedbackListState(feedbackListQuery(state)), state)
  assert.notEqual(feedbackScrollKey(42, feedbackListQuery(state)), feedbackScrollKey(43, feedbackListQuery(state)))
  assert.notEqual(feedbackScrollKey(42, feedbackListQuery(state)), feedbackScrollKey(42, feedbackListQuery({ ...state, page: 2 })))
})
test('untrusted route values cannot produce invalid filters or pagination', () => {
  const got = feedbackListState({ status: 'bad', rating: ['useful'], page: '-4', pageSize: '50000', start: 'garbage', source: 'email' })
  assert.deepEqual(got, { status: 'pending', rating: 'all', source: '', reporter: '', keyword: '', start: '', end: '', page: 1, pageSize: 20 })
})
