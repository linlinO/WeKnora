import { get, post } from '@/utils/request'

export interface AnswerFeedback {
  id: string
  rating: 'useful' | 'not_useful' | ''
  status: 'pending' | 'correcting' | 'reviewed' | 'resolved' | 'ignored' | 'withdrawn'
  source: 'web' | 'wecom' | 'admin_review'
  user_id: string
  user_name?: string
  channel_id?: string
  question: string
  answer: string
  reason?: string
  knowledge_base_id?: string
  correction_knowledge_id?: string
  created_at: string
  review_note?: string
  reviewer_id?: string
}

export function listAnswerFeedback(filters: Record<string, string | number> = {}) {
  const params = new URLSearchParams(Object.entries(filters).map(([key, value]) => [key, String(value)]))
  return get(`/api/v1/admin/answer-feedback?${params}`) as Promise<{ data: AnswerFeedback[]; total: number; status_counts: Record<string, number> }>
}

export function getAnswerFeedback(id: string) {
  return get(`/api/v1/admin/answer-feedback/${encodeURIComponent(id)}`) as Promise<{
    data: AnswerFeedback; proposal: KnowledgeEvolutionProposal | null; regression: KnowledgeRegressionRun | null
    references: Array<{ knowledge_id?: string; knowledge_title?: string; content?: string }>
  }>
}

export function saveEvolutionDraft(id: string, answer: string) {
  return post(`/api/v1/admin/knowledge-evolution/${encodeURIComponent(id)}/draft`, { answer }) as Promise<{ data: KnowledgeEvolutionProposal }>
}

export function getOwnAnswerFeedback(sessionId: string, messageId: string) {
  return get(`/api/v1/answer-feedback/${encodeURIComponent(sessionId)}/${encodeURIComponent(messageId)}`) as Promise<{ data: AnswerFeedback | null }>
}

export function setFeedbackReporterName(id: string, name: string) {
  return post(`/api/v1/admin/answer-feedback/${encodeURIComponent(id)}/reporter-name`, { name })
}

export function saveAnswerFeedback(sessionId: string, messageId: string, payload: { rating: 'useful' | 'not_useful' | 'cancel'; reason?: string }) {
  return post(`/api/v1/answer-feedback/${encodeURIComponent(sessionId)}/${encodeURIComponent(messageId)}`, payload) as Promise<{ data: AnswerFeedback }>
}

export function reviewAnswerFeedback(
  id: string,
  payload: {
    action: 'publish' | 'ignore' | 'acknowledge' | 'correct'
    answer?: string
    title?: string
    knowledge_base_id?: string
    similar_questions?: string[]
    note?: string
  },
) {
  return post(`/api/v1/admin/answer-feedback/${id}/review`, payload) as Promise<{ success: boolean; data: AnswerFeedback }>
}

export interface KnowledgeEvolutionProposal {
  id: string
  knowledge_base_id: string
  feedback_id: string
  question: string
  original_answer: string
  candidate_answer: string
  status: string
  conflict_status: string
  conflict_summary?: string
  supersedes_knowledge_id?: string
  correction_knowledge_id?: string
  regression_run_id?: string
  auto_generated?: boolean
  created_at: string
  updated_at: string
}

export interface KnowledgeRegressionRun {
	 kind?: 'legacy_text' | 'preview' | 'published'
	 agent_id?: string
	 model_id?: string
	 input?: {answer:string;questions:string[];required:string[];forbidden:string[];expected_behavior?:'answer'|'clarification';context?:string}
	 started_at?:string
  id: string
  proposal_id: string
  status: 'pending' | 'passed' | 'failed'
  total: number
  passed: number
  critical_total: number
  critical_passed: number
  failure_summary?: string
}

export interface KnowledgeReplayResult {id:string;run_id:string;question:string;answer:string;expected_answer:string;status:string;failure_reason:string;evaluation:string;duration_ms:number;references:Array<{knowledge_id:string;knowledge_title:string;content:string}>|null}
export const knowledgeReplayRuns=(id:string)=>get(`/api/v1/admin/knowledge-evolution/${encodeURIComponent(id)}/runs`) as Promise<{data:KnowledgeRegressionRun[];results:KnowledgeReplayResult[];proposal:KnowledgeEvolutionProposal}>
export const startKnowledgeReplay=(id:string,live:boolean,input:{answer:string;agent_id:string;questions:string[];required:string[];forbidden:string[];expected_behavior?:'answer'|'clarification';context?:string})=>post(`/api/v1/admin/knowledge-evolution/${encodeURIComponent(id)}/${live?'verify':'regression'}`,input) as Promise<{data:KnowledgeRegressionRun;proposal:KnowledgeEvolutionProposal}>

export function createEvolutionDraft(feedbackId: string, knowledgeBaseId: string) {
  return post(`/api/v1/admin/knowledge-evolution/feedback/${encodeURIComponent(feedbackId)}/draft`, {
    knowledge_base_id: knowledgeBaseId,
  }) as Promise<{ success: boolean; data: KnowledgeEvolutionProposal }>
}

export function runEvolutionRegression(proposalId: string, answer?: string) {
  return post(`/api/v1/admin/knowledge-evolution/${encodeURIComponent(proposalId)}/regression`, { answer }) as Promise<{
    success: boolean
    data: KnowledgeRegressionRun
    proposal: KnowledgeEvolutionProposal
  }>
}

export function publishEvolutionProposal(proposalId: string, payload: {
  answer?: string
  title?: string
  note?: string
  force?: boolean
  confirm_conflict?: boolean
}) {
  return post(`/api/v1/admin/knowledge-evolution/${encodeURIComponent(proposalId)}/publish`, payload) as Promise<{
    success: boolean
    data: KnowledgeEvolutionProposal
    knowledge_id: string
  }>
}

export function rollbackEvolutionProposal(proposalId: string, note = '') {
  return post(`/api/v1/admin/knowledge-evolution/${encodeURIComponent(proposalId)}/rollback`, { note })
}
