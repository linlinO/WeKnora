<template>
 <main v-if="auth.hasRole('admin')" class="employee-memory">
  <header><div><t-button theme="primary" variant="text" @click="back">‹ {{id?'返回员工列表':'返回问答统计'}}</t-button><h1>{{id?'员工记忆详情':'员工记忆'}}</h1><p>当前空间：{{auth.currentTenantName}} · 企业微信身份独立保存，正文查看需要授权并留存审计。</p></div><t-button variant="outline" :loading="loading" @click="load">刷新</t-button></header>
  <t-alert v-if="error" theme="error" :message="error" />
  <template v-if="!id">
   <details v-if="access.owner" class="access"><summary>记忆正文查看授权（空间所有者管理）</summary><p>仅授予需要处理记忆问题的管理员。每次查看仍需填写原因，操作记录进入审计日志。</p><div v-for="member in access.members" :key="member.user_id" class="member"><span>{{member.name || member.user_id}}</span><span>{{member.role==='owner'?'所有者':'管理员'}}</span><t-button theme="primary" variant="text" :loading="grantBusy===member.user_id" @click="grant(member.user_id,!member.granted)">{{member.granted?'撤销授权':'授权查看正文'}}</t-button></div></details>
   <form @submit.prevent="search"><label>员工<t-input v-model="draft" placeholder="姓名或企业微信 ID" clearable/></label><t-button type="submit">查询</t-button><t-button variant="outline" @click="reset">重置</t-button></form>
   <t-table :data="rows" :columns="columns" row-key="id" table-layout="fixed" :loading="loading" hover>
    <template #name="{row}">{{row.name || row.user_id}}</template><template #enabled="{row}"><t-tag :theme="row.enabled?'success':'default'" variant="light">{{row.enabled?'已开启':'已暂停'}}</t-tag></template><template #last_extracted_at="{row}">{{qaTime(row.last_extracted_at)}}</template><template #operation="{row}"><t-button theme="primary" variant="text" size="small" @click="open(row.id)">查看</t-button></template>
   </t-table><footer><span>共 {{total}} 人</span><t-pagination :current="page" :total="total" :page-size="20" :show-page-size="false" :total-content="false" @current-change="changePage"/></footer>
  </template>
  <template v-else-if="detail">
   <section><h3>运行状态</h3><p>个人记忆：{{detail.enabled?'已开启':'已暂停'}} · 最近提炼：{{qaTime(detail.last_extracted_at || '')}}</p><p>空间和智能体开关也会影响实际调用。逐次调用原因请在“问答统计 → 问答明细 → 查看”中核对。</p>
    <t-table :data="detail.jobs" :columns="jobColumns" row-key="session_id" table-layout="fixed"><template #pending="{row}">{{row.pending?'待提炼':'已处理'}}</template><template #updated_at="{row}">{{qaTime(row.updated_at)}}</template><template #failure_code="{row}">{{row.failure_code || '—'}}</template></t-table><p>最多展示最近 50 个提炼会话。异常代码用于排查任务，不代表所有问答均失败。</p>
   </section>
   <section><h3>个人记忆正文</h3><p v-if="!access.granted">你尚未获得记忆正文查看授权，请由本空间所有者在员工列表页授权。</p><template v-else><form @submit.prevent="readContent(1)"><label class="reason">查看原因<t-input v-model="reason" placeholder="例如：排查员工反馈的记忆未生效问题" :maxlength="200"/></label><t-button type="submit" :loading="contentLoading" :disabled="reason.trim().length<4">查看并记录审计</t-button></form><p>管理员在此只读查看；确认、删除和暂停仍由员工私聊机器人操作。</p></template>
    <template v-if="revealed"><article v-for="item in items" :key="item.id" class="memory-item"><div><t-tag>{{status(item.status)}}</t-tag> {{kind(item.kind)}} · {{item.origin==='explicit'?'员工明确要求记住':item.origin==='extracted'?'自动提炼':'手动记录'}}</div><p class="content">{{item.content}}</p><small>更新时间：{{qaTime(item.updated_at)}} · 最近使用：{{qaTime(item.last_used_at || '')}} · 到期时间：{{qaTime(item.expires_at || '')}}</small><details><summary>来源记录</summary><p>会话：{{item.source_session_id || '无'}}<br>消息：{{item.source_message_id || '无'}}</p></details></article><t-empty v-if="!items.length" description="暂无记忆条目"/><footer><span>共 {{itemTotal}} 条</span><t-pagination :current="contentPage" :total="itemTotal" :page-size="20" :show-page-size="false" :total-content="false" @current-change="readContent"/></footer></template>
   </section>
  </template>
 </main>
</template>
<script setup lang="ts">
import {computed,ref,watch} from 'vue'
import {useRoute,useRouter} from 'vue-router'
import {useAuthStore} from '@/stores/auth'
import {tableTooltip} from '@/utils/tableTooltip'
import {qaTime} from '@/api/qa-insights'
import {memoryAccess,grantMemoryAccess,employeeMemories,employeeMemory,employeeMemoryContent,type EmployeeMemory,type EmployeeMemoryItem,type MemoryAccess,type ExtractionJob} from '@/api/memory-admin'
const route=useRoute(),router=useRouter(),auth=useAuthStore(),id=computed(()=>String(route.params.id||'')),page=computed(()=>Math.max(1,Number(route.query.page)||1))
const draft=ref(''),rows=ref<EmployeeMemory[]>([]),total=ref(0),loading=ref(false),error=ref(''),grantBusy=ref('')
const access=ref<MemoryAccess>({granted:false,owner:false,members:[]}),detail=ref<{enabled:boolean;last_extracted_at?:string;jobs:ExtractionJob[]}|null>(null)
const reason=ref(''),items=ref<EmployeeMemoryItem[]>([]),itemTotal=ref(0),contentPage=ref(1),contentLoading=ref(false),revealed=ref(false)
let request=0,contentRequest=0
const columns=[{colKey:'name',title:'员工',width:240},{colKey:'enabled',title:'个人开关',width:110},{colKey:'active',title:'生效记忆',width:90},{colKey:'pending',title:'待确认',width:90},{colKey:'queued',title:'待提炼会话',width:110},{colKey:'failed',title:'提炼异常',width:100},{colKey:'last_extracted_at',title:'最近提炼',width:175},{colKey:'operation',title:'操作',width:96,align:'center' as const}].map(c=>({...c,ellipsis:tableTooltip,ellipsisTitle:tableTooltip}))
const jobColumns=[{colKey:'session_id',title:'会话',width:300},{colKey:'pending',title:'提炼状态',width:120},{colKey:'failure_count',title:'失败次数',width:100},{colKey:'failure_code',title:'异常原因',width:200},{colKey:'updated_at',title:'更新时间',width:180}].map(c=>({...c,ellipsis:tableTooltip,ellipsisTitle:tableTooltip}))
async function load(){const seq=++request;error.value='';loading.value=true;try{const a=await memoryAccess();if(seq!==request)return;access.value=a.data;if(id.value){const d=await employeeMemory(id.value);if(seq===request)detail.value=d.data}else{const r=await employeeMemories(page.value,String(route.query.keyword||''));if(seq===request){rows.value=r.data||[];total.value=r.total}}}catch(e:any){if(seq===request)error.value=e?.message||'读取失败'}finally{if(seq===request)loading.value=false}}
async function grant(user:string,enabled:boolean){grantBusy.value=user;try{await grantMemoryAccess(user,enabled);await load()}catch(e:any){error.value=e?.message||'授权失败'}finally{grantBusy.value=''}}
async function readContent(p:number){const seq=++contentRequest,tenant=auth.effectiveTenantId,subject=id.value;contentLoading.value=true;error.value='';try{const r=await employeeMemoryContent(subject,reason.value.trim(),p);if(seq!==contentRequest||tenant!==auth.effectiveTenantId||subject!==id.value)return;items.value=r.data||[];itemTotal.value=r.total;contentPage.value=p;revealed.value=true}catch(e:any){if(seq===contentRequest){items.value=[];revealed.value=false;error.value=e?.message||'读取失败'}}finally{if(seq===contentRequest)contentLoading.value=false}}
function search(){router.replace({path:'/platform/qa-insights/memory',query:{keyword:draft.value,page:'1'}})}
function reset(){draft.value='';search()}
function changePage(p:number){router.replace({path:route.path,query:{...route.query,page:String(p)}})}
function open(subject:string){router.push({path:`/platform/qa-insights/memory/${subject}`,query:{...route.query}})}
function back(){router.push(id.value?{path:'/platform/qa-insights/memory',query:{...route.query}}:'/platform/qa-insights')}
function status(s:string){return ({active:'生效中',pending:'待确认',superseded:'已替代',archived:'已归档'} as Record<string,string>)[s]||s}
function kind(s:string){return ({preference:'回答偏好',fact:'个人事实',profile:'个人信息',task:'任务',interest:'关注方向'} as Record<string,string>)[s]||s}
watch(()=>[route.fullPath,auth.effectiveTenantId],()=>{++contentRequest;contentLoading.value=false;reason.value='';items.value=[];revealed.value=false;detail.value=null;rows.value=[];access.value={granted:false,owner:false,members:[]};draft.value=String(route.query.keyword||'');void load()},{immediate:true})
</script>
<style scoped>
.employee-memory{padding:28px 32px;overflow:auto;height:100%;box-sizing:border-box;background:var(--td-bg-color-container)}header{display:flex;align-items:start;justify-content:space-between}h1{margin:12px 0;font-size:24px}p,small{color:var(--td-text-color-secondary);line-height:1.8}form{display:flex;gap:12px;align-items:end;padding:16px;background:var(--td-bg-color-secondarycontainer);border-radius:8px;margin:20px 0}label{display:flex;flex-direction:column;gap:8px;width:300px}.reason{width:min(560px,75%)}section,.access{padding:20px;border:1px solid var(--td-component-border);border-radius:8px;margin:20px 0}summary{cursor:pointer}.member{display:flex;align-items:center;gap:20px;padding:8px}.member>span:first-child{min-width:160px}footer{display:flex;align-items:center;justify-content:flex-end;gap:16px;margin-top:20px}footer :deep(.t-pagination){width:auto}.memory-item{padding:18px 0;border-top:1px solid var(--td-component-border)}.content{white-space:pre-wrap;overflow-wrap:anywhere;color:var(--td-text-color-primary)}:deep(.t-table td),:deep(.t-table th){white-space:nowrap;vertical-align:middle}:deep(.t-table__cell-inner){min-width:0}:deep(.t-table tr:nth-child(even)){background:var(--td-bg-color-secondarycontainer)}
</style>
