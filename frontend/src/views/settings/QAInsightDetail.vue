<template>
  <main v-if="auth.hasRole('admin')" class="detail-page">
    <header><div><t-button variant="text" @click="back">‹ 返回列表</t-button><h1>{{route.query.compare?'回答对比':'问答详情'}}</h1><p>展示当次实际生成的回答与引用。不同答案的差异不自动认定为错误。</p></div></header>
    <t-alert v-if="error" theme="error" :message="error"/>
    <t-loading :loading="loading"><div class="panels" :class="{compare:panels.length>1}">
      <article v-for="(panel,index) in panels" :key="panel.data.id">
        <section><div class="meta">{{qaSource(panel.data.source)}} · {{panel.data.chat_type==='group'?'群聊':panel.data.chat_type==='direct'?'私聊':'网页 / API'}} · {{qaTime(panel.data.created_at)}}</div><h2>{{panel.data.question}}</h2><p>提问人：{{panel.data.user_name || panel.data.user_id}}</p></section>
        <section v-if="!panel.answers.length"><t-empty description="问题已记录，尚无回答记录"/></section>
        <template v-else>
          <section class="answer-meta"><label>回答记录<t-select v-model="selected[index]"><t-option v-for="(a,i) in panel.answers" :key="a.id" :value="a.id" :label="`第 ${i+1} 次 · ${qaStatus(a.generation_status)}`"/></t-select></label><div v-if="answer(panel,index)"><p>模型：{{answer(panel,index)?.model_name || answer(panel,index)?.model_id || '未记录'}}<br>生成：{{qaStatus(answer(panel,index)!.generation_status)}} · 耗时 {{Math.round((answer(panel,index)?.duration_ms||0)/1000)}} 秒<br>发送：{{qaStatus(answer(panel,index)!.delivery_status)}}<br>使用个人记忆：{{answer(panel,index)?.memory_count||0}} 条（此处不展示私人内容）</p></div></section>
          <section v-if="answer(panel,index)"><h3>记忆调用诊断</h3><p>{{memoryReason(answer(panel,index)?.memory_diagnostic)}}</p><p v-if="answer(panel,index)?.memory_diagnostic">候选记忆 {{answer(panel,index)?.memory_diagnostic?.candidates || 0}} 条 · 本次使用 {{answer(panel,index)?.memory_diagnostic?.used || 0}} 条</p><t-button theme="primary" variant="text" @click="router.push('/platform/qa-insights/memory')">查看员工记忆状态</t-button></section>
          <section v-if="answer(panel,index)"><div class="section-title"><h3>实际回答</h3><t-button :loading="reviewing===answer(panel,index)?.id" :disabled="answer(panel,index)?.generation_status==='generating'" @click="review(answer(panel,index)!.id)">转入纠错</t-button></div><div class="markdown" v-html="render(answer(panel,index)!.answer)"/></section>
          <section><h3>当次引用资料</h3><p v-if="!answer(panel,index)?.references?.length">未记录引用资料</p><details v-for="(r,i) in answer(panel,index)?.references||[]" :key="i"><summary>{{r.knowledge_title||r.knowledge_id||'引用资料'}}</summary><p class="reference">{{r.content||'本条引用没有保存片段内容；不能据此还原文档全文版本。'}}</p></details></section>
          <section><h3>用户反馈与审核</h3><p v-if="!panel.feedback.length">暂无反馈。没有反馈不代表答案正确。</p><div v-for="f in panel.feedback" :key="f.id" class="feedback"><t-tag>{{f.source==='admin_review'?'管理员审核':f.rating==='useful'?'有用':'无用'}}</t-tag><span>{{feedbackStatus(f.status)}}</span><p>{{f.reason||'无补充说明'}}</p><t-button variant="text" @click="router.push(`/platform/answer-feedback/${f.id}`)">查看处理记录</t-button></div></section>
        </template>
      </article>
    </div></t-loading>
  </main>
</template>
<script setup lang="ts">
import {onMounted,ref,watch} from 'vue'
import {useRoute,useRouter} from 'vue-router'
import {MessagePlugin} from 'tdesign-vue-next'
import {marked} from 'marked'
import DOMPurify from 'dompurify'
import {useAuthStore} from '@/stores/auth'
import {memoryReason} from '@/api/memory-admin'
import {insightDetail,insightReview,qaSource,qaStatus,qaTime,type QADetail} from '@/api/qa-insights'
import {qaQuery,qaState} from './qaInsightsState'
const auth=useAuthStore(),route=useRoute(),router=useRouter(),panels=ref<QADetail[]>([]),selected=ref<string[]>([]),loading=ref(false),error=ref(''),reviewing=ref('')
let request=0
function answer(panel:QADetail,index:number){return panel.answers.find(a=>a.id===selected.value[index]) || panel.answers.at(-1)}
function render(s:string){return DOMPurify.sanitize(marked.parse(s||'暂无回答正文',{async:false}) as string)}
function feedbackStatus(s:string){return ({pending:'待处理',correcting:'纠错中',reviewed:'已阅',resolved:'已发布',ignored:'已忽略',withdrawn:'已撤回'} as Record<string,string>)[s]||s}
function back(){const topic=typeof route.query.fromTopic==='string'?route.query.fromTopic:'';return router.push({path:topic?`/platform/qa-insights/topics/${topic}`:'/platform/qa-insights',query:{...qaQuery(qaState(route.query)),...(typeof route.query.parent==='string'?{parent:route.query.parent}:{})}})}
async function load(){const id=++request;loading.value=true;error.value='';try{const ids=[String(route.params.id)];if(typeof route.query.compare==='string'&&route.query.compare!==ids[0])ids.push(route.query.compare);const result=await Promise.all(ids.map(insightDetail));if(id!==request)return;panels.value=result;selected.value=result.map(p=>p.answers.at(-1)?.id||'')}catch(e:any){if(id===request)error.value=e.message||'读取问答失败'}finally{if(id===request)loading.value=false}}
async function review(id:string){reviewing.value=id;try{const result=await insightReview(id);await router.push(`/platform/answer-feedback/${result.feedback_id}`)}catch(e:any){MessagePlugin.error(e.message||'创建纠错记录失败')}finally{reviewing.value=''}}
watch(()=>route.fullPath,load);watch(()=>auth.effectiveTenantId,()=>{request++;panels.value=[];router.replace('/platform/qa-insights')});onMounted(load)
</script>
<style scoped>
.detail-page{flex:1;min-width:0;height:100%;overflow:auto;box-sizing:border-box;padding:24px 32px;background:var(--td-bg-color-container)}h1{font-size:24px;margin:8px 0}header p,.meta,p{color:var(--td-text-color-secondary);font-size:13px;line-height:1.8}.panels{display:grid;grid-template-columns:minmax(0,1fr);gap:22px;max-width:1280px;margin:22px auto}.panels.compare{grid-template-columns:repeat(2,minmax(0,1fr));max-width:none}article{border:1px solid var(--td-component-border);border-radius:12px;overflow:hidden}section{padding:22px;border-bottom:1px solid var(--td-component-border)}section:last-child{border:0}h2{font-size:18px;line-height:1.7;white-space:pre-wrap;overflow-wrap:anywhere}h3{font-size:15px;margin:0 0 14px}.section-title{display:flex;justify-content:space-between;gap:12px;align-items:center;margin-bottom:16px}.section-title h3{margin:0}.answer-meta{background:var(--td-bg-color-secondarycontainer)}label{display:flex;gap:12px;align-items:center;font-size:13px}label :deep(.t-select__wrap){max-width:280px}.markdown{font-size:14px;line-height:1.9;overflow-wrap:anywhere}.markdown :deep(pre){background:var(--td-bg-color-secondarycontainer);padding:16px;border-radius:8px;overflow:auto}.markdown :deep(img){max-width:100%}.markdown :deep(table){display:block;overflow:auto;border-collapse:collapse}.markdown :deep(td),.markdown :deep(th){padding:8px;border:1px solid var(--td-component-border)}details{padding:12px 0}summary{cursor:pointer;font-size:14px}.reference{white-space:pre-wrap;max-height:350px;overflow:auto}.feedback{padding:12px 0}.feedback span{margin-left:10px;font-size:12px}@media(max-width:1000px){.panels.compare{grid-template-columns:1fr}}
</style>
