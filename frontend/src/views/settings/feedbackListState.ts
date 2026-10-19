export const feedbackStatuses = [
  { value: 'pending', label: '待处理', icon: 'time' },
  { value: 'correcting', label: '纠错中', icon: 'edit' },
  { value: 'reviewed', label: '已阅', icon: 'check-circle' },
  { value: 'resolved', label: '已发布', icon: 'check-circle' },
  { value: 'ignored', label: '已忽略', icon: 'minus-circle' },
  { value: 'withdrawn', label: '已撤回', icon: 'rollback' },
  { value: 'all', label: '全部', icon: 'file' },
]
export function feedbackListState(query: Record<string, unknown>) {
  const text = (key: string) => typeof query[key] === 'string' ? query[key] as string : ''
  const date = (key: string) => /^\d{4}-\d{2}-\d{2}$/.test(text(key)) ? text(key) : ''
  return {
    status: feedbackStatuses.some(s => s.value === text('status')) ? text('status') : 'pending',
    rating: ['useful', 'not_useful'].includes(text('rating')) ? text('rating') : 'all',
    source: ['web', 'wecom'].includes(text('source')) ? text('source') : '',
    reporter: text('reporter'), keyword: text('keyword'), start: date('start'), end: date('end'),
    page: Math.min(100000, Math.max(1, Math.trunc(Number(query.page)) || 1)),
    pageSize: [20, 50, 100].includes(Number(query.pageSize)) ? Number(query.pageSize) : 20,
  }
}
export type FeedbackListState = ReturnType<typeof feedbackListState>
export function feedbackListQuery(state: FeedbackListState) {
  return Object.fromEntries(Object.entries(state).map(([key, value]) => [key, String(value)]))
}
export function feedbackScrollKey(tenant: string | number | null | undefined, query: Record<string, unknown>) {
  return `feedback-list:${tenant}:${JSON.stringify(feedbackListState(query))}`
}
