import {get,put,post} from '@/utils/request'
export interface EmployeeMemory {id:string;name:string;user_id:string;channel_id:string;enabled:boolean;active:number;pending:number;queued:number;failed:number;last_extracted_at?:string}
export interface MemoryAccess {granted:boolean;owner:boolean;members:Array<{user_id:string;name:string;role:string;granted:boolean}>}
export interface ExtractionJob {session_id:string;pending:boolean;failure_count:number;failure_code:string;updated_at:string}
export interface EmployeeMemoryItem {id:string;content:string;kind:string;status:string;origin:string;source_message_id:string;source_session_id:string;updated_at:string;last_used_at?:string;expires_at?:string}
const base='/api/v1/admin/memory'
export const memoryAccess=()=>get(`${base}/access`) as Promise<{data:MemoryAccess}>
export const grantMemoryAccess=(user_id:string,enabled:boolean)=>put(`${base}/access`,{user_id,enabled})
export const employeeMemories=(page:number,keyword:string)=>get(`${base}/subjects?${new URLSearchParams({page:String(page),keyword,page_size:'20'})}`) as Promise<{data:EmployeeMemory[];total:number}>
export const employeeMemory=(id:string)=>get(`${base}/subjects/${encodeURIComponent(id)}`) as Promise<{data:{id:string;enabled:boolean;last_extracted_at?:string;jobs:ExtractionJob[]}}>
export const employeeMemoryContent=(id:string,reason:string,page:number)=>post(`${base}/subjects/${encodeURIComponent(id)}/content`,{reason,page}) as Promise<{data:EmployeeMemoryItem[];total:number}>
export interface MemoryDiagnostic {reason:string;mode:string;used:number;candidates:number;vector_fallback?:string}
export function memoryReason(d?:MemoryDiagnostic|null){
 if(!d?.reason)return '历史回答未记录调用诊断，不能据此推断未调用原因'
 const reasons:Record<string,string>={recalled:'已调用个人记忆',no_relevant_memory:'没有可注入的相关记忆',no_active_preference:'群聊仅使用回答偏好，当前没有生效偏好',workspace_disabled:'空间长期记忆已关闭',agent_disabled:'本次智能体未启用记忆',user_disabled:'员工已暂停个人记忆',no_subject:'该员工尚未建立记忆',subject_load_failed:'记忆读取失败，本次已降级回答',no_principal:'本次没有可用的员工身份',unknown:'记忆条件未满足'}
 const fallbacks:Record<string,string>={embed_failed:'向量查询失败，已使用关键词匹配',no_embedding_model:'未配置记忆向量模型',no_candidates:'没有待匹配的记忆'}
 return `${reasons[d.reason]||d.reason}；范围：${d.mode==='group_preferences'?'群聊偏好':'个人记忆'}${d.vector_fallback ? '；'+(fallbacks[d.vector_fallback]||'向量召回未使用：'+d.vector_fallback) : ''}`
}
