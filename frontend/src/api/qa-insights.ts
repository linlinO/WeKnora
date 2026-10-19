import { get, post } from '@/utils/request'

export interface QAQuestion {
  id: string; question: string; user_id: string; user_name: string; source: string
  channel_id: string; chat_type: string; topic_id: string; classification: string; created_at: string
}
export interface QATopic {
  id: string; title: string; questions: number; people: number; not_useful: number; useful: number; failures: number; last_asked: string
}
export interface QAAnswer {
  id: string; answer: string; generation_status: string; delivery_status: string; model_name: string; model_id: string
  agent_id: string; memory_count: number; duration_ms: number; created_at: string
  memory_diagnostic?: import('./memory-admin').MemoryDiagnostic | null
  references: Array<{ knowledge_title?: string; knowledge_id?: string; content?: string; chunk_id?: string }> | null
}
export interface QADetail {
  data: QAQuestion; answers: QAAnswer[]
  feedback: Array<{ id: string; rating: string; source: string; status: string; reason: string; message_id: string }>
}
export function insightList<T>(kind: 'questions' | 'topics', filters: Record<string, string | number>) {
  const params = new URLSearchParams(Object.entries(filters).map(([k,v])=>[k,String(v)]))
  return get(`/api/v1/admin/qa-insights/${kind}?${params}`) as Promise<{data:T[]; total:number}>
}
export const insightDetail = (id: string) => get(`/api/v1/admin/qa-insights/questions/${encodeURIComponent(id)}`) as Promise<QADetail>
export const insightTopic = (id: string) => get(`/api/v1/admin/qa-insights/topics/${encodeURIComponent(id)}`) as Promise<{data:{id:string;title:string}}>
export const insightReview = (id: string) => post(`/api/v1/admin/qa-insights/answers/${encodeURIComponent(id)}/review`,{}) as Promise<{feedback_id:string}>
export const insightReclassify = (id: string, data: { action:string;target_id?:string;title?:string;question_ids?:string[] }) => post(`/api/v1/admin/qa-insights/topics/${encodeURIComponent(id)}/reclassify`,data) as Promise<{topic_id:string}>
export const insightMemoryStatus = () => get('/api/v1/admin/qa-insights/memory-status') as Promise<{subjects:number;items:number;pending:number;failed:number}>
export const qaSource = (s:string) => ({wecom:'企业微信',web:'Web',api:'API',admin_review:'管理员审核'}[s] || s || '未知')
export const qaStatus = (s:string) => ({generating:'生成中',completed:'已生成',clarification:'等待补充信息',handoff:'建议人工核对',no_match:'无适用知识',unavailable:'信息检查未完成',failed:'生成失败',interrupted:'已中断',truncated:'输出截断',submitted_unconfirmed:'已提交 · 平台未确认',unknown:'未知'}[s] || s)
export const qaTime = (s:string) => s ? new Date(s).toLocaleString('zh-CN',{hour12:false}) : '—'
