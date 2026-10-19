<template>
 <section class="replay-panel">
  <h3>{{live?'发布后真实检索复验':'发布前问答预演'}}</h3>
  <p>{{live?'使用已发布知识的真实索引，检查是否检索到新版本、是否仍引用旧版本及回答是否符合审核答案。':'先执行真实知识检索，再临时加入当前纠错草稿生成回答。草稿不写入知识库；是否能从索引检索到新版本，需要发布后复验。'}}</p>
  <p>使用所选智能体的模型和提示词，仅检索目标知识库；关闭个人记忆、历史对话和联网。测试调用会消耗模型额度，不发送企业微信消息。</p>
  <label>测试智能体<t-select v-model="agentID" :disabled="running"><t-option v-for="a in agents" :key="a.id" :value="a.id" :label="a.name"/></t-select></label>
  <label>预期行为<t-select v-model="expectedBehavior" :disabled="running"><t-option value="answer" label="信息充分：应回答并引用知识"/><t-option value="clarification" label="信息不足：应追问，不生成处理方案"/></t-select></label>
  <p v-if="expectedBehavior==='clarification'">追问测试检查是否请求补充关键信息，不要求引用草稿。通过后仍需补全问题并完成回答预演，才能发布纠错。</p>
  <label>已确认的场景补充（可选）<t-textarea v-model="scenario" :disabled="running" :maxlength="4000" :autosize="{minRows:2,maxRows:5}" placeholder="只填写用户已经确认的产品、报错和适用条件。不要把正确答案填在这里。"/></label>
  <details><summary>补充测试问题和检查条件</summary><label>相同问题的其他问法（每行一个，最多4个；自动包含原问题）<t-textarea v-model="questions" :disabled="running" :autosize="{minRows:2,maxRows:5}"/></label><label>必须包含的短语（每行一个，可留空）<t-textarea v-model="required" :disabled="running" :autosize="{minRows:2,maxRows:4}"/></label><label>禁止出现的旧结论或命令（每行一个，可留空）<t-textarea v-model="forbidden" :disabled="running" :autosize="{minRows:2,maxRows:4}"/></label></details>
  <t-button theme="primary" variant="outline" :loading="running" :disabled="disabled || !answer.trim() || !agentID" @click="start">{{live?'运行真实检索复验':'运行问答预演'}}</t-button>
  <t-alert v-if="error" theme="error" :message="error"/>
  <p v-if="running">测试进行中，可离开此页，返回后继续查看结果。每题分别进行回答生成和事实评估。</p>
  <article v-for="run in visibleRuns" :key="run.id" class="run">
   <div class="run-heading"><t-tag :theme="run.status==='passed'?'success':run.status==='pending'?'primary':'warning'">{{run.status==='passed'?'通过':run.status==='pending'?'运行中':'未通过'}}</t-tag><strong>{{run.kind==='published'?'真实检索复验':run.kind==='preview'?'候选问答预演':'历史文本检查'}}</strong><span>{{run.passed}} / {{run.total}} 项</span></div>
   <p>{{qaTime(run.started_at||'')}} · 模型：{{modelNames[run.model_id||'']||run.model_id||'未记录'}}</p><p v-if="run.failure_summary" class="failure">{{run.failure_summary}}</p>
   <p>预期行为：{{run.input?.expected_behavior==='clarification'?'追问（不作为发布依据）':'回答并引用知识'}}</p>
   <p v-if="run.input?.context">当次场景补充：{{run.input.context}}</p>
   <details v-for="result in results.filter(r=>r.run_id===run.id)" :key="result.id"><summary>{{result.status==='passed'?'✓':'!'}} {{result.question}} · {{Math.round(result.duration_ms/1000)}} 秒</summary><p v-if="result.failure_reason" class="failure">{{result.failure_reason}}</p><p>模型评估：{{result.evaluation || '未完成'}}</p><h4>实际回答</h4><div class="answer" v-html="render(result.answer)"/><details><summary>当次引用资料（{{result.references?.length||0}}）</summary><div v-for="(r,i) in result.references||[]" :key="i"><strong>{{r.knowledge_title || r.knowledge_id}}</strong><p class="source">{{r.content}}</p></div></details></details>
  </article>
  <p class="note">通过表示本轮测试满足检查条件及模型评估，仍需审核人员核实业务事实。</p>
 </section>
</template>
<script setup lang="ts">
import {computed,ref,watch,onBeforeUnmount,onMounted} from 'vue'
import {listModels} from '@/api/model'
import {marked} from 'marked'
import {sanitizeHTML} from '@/utils/security'
import {listAgents} from '@/api/agent'
import {qaTime} from '@/api/qa-insights'
import {knowledgeReplayRuns,startKnowledgeReplay,type KnowledgeEvolutionProposal,type KnowledgeRegressionRun,type KnowledgeReplayResult} from '@/api/answer-feedback'
const props=defineProps<{proposal:KnowledgeEvolutionProposal;answer:string;disabled?:boolean;live?:boolean}>()
const emit=defineEmits<{update:[{proposal:KnowledgeEvolutionProposal;run:KnowledgeRegressionRun}]}>()
const agents=ref<Array<{id:string;name:string}>>([]),agentID=ref('builtin-quick-answer'),questions=ref(''),required=ref(''),forbidden=ref(''),error=ref(''),starting=ref(false),runs=ref<KnowledgeRegressionRun[]>([]),results=ref<KnowledgeReplayResult[]>([])
const modelNames=ref<Record<string,string>>({})
const expectedBehavior=ref<'answer'|'clarification'>('answer')
const scenario=ref('')
onMounted(async()=>{try{const models=await listModels('KnowledgeQA');if(!disposed)modelNames.value=Object.fromEntries(models.filter(m=>m.id).map(m=>[m.id!,m.display_name||m.name]))}catch{/* Historical IDs remain visible if a model is no longer accessible. */}})
const running=computed(()=>starting.value||runs.value.some(r=>r.status==='pending')),visibleRuns=computed(()=>runs.value.filter(r=>props.live?r.kind==='published':r.kind!=='published').slice(0,3))
let timer:ReturnType<typeof setTimeout>|undefined,request=0,disposed=false
const lines=(s:string)=>s.split('\n').map(v=>v.trim()).filter(Boolean)
const render=(s:string)=>sanitizeHTML(marked.parse(s||'未生成回答',{async:false}) as string)
async function load(){const seq=++request;try{const r=await knowledgeReplayRuns(props.proposal.id);if(disposed||seq!==request)return;runs.value=r.data||[];results.value=r.results||[];const latest=r.data?.find(v=>v.id===r.proposal.regression_run_id);if(latest)emit('update',{proposal:r.proposal,run:latest});if(r.data?.some(v=>v.status==='pending'))timer=setTimeout(load,2500)}catch(e:any){if(!disposed&&seq===request)error.value=e?.message||'读取测试结果失败'}}
async function start(){error.value='';starting.value=true;try{const r=await startKnowledgeReplay(props.proposal.id,!!props.live,{answer:props.answer,agent_id:agentID.value,questions:lines(questions.value),required:lines(required.value),forbidden:lines(forbidden.value),expected_behavior:expectedBehavior.value,context:scenario.value});if(disposed)return;if(!props.live)emit('update',{proposal:r.proposal,run:r.data});if(timer)clearTimeout(timer);await load()}catch(e:any){error.value=e?.message||'启动测试失败'}finally{starting.value=false}}
watch(()=>props.proposal.id,async()=>{if(timer)clearTimeout(timer);runs.value=[];results.value=[];error.value='';await load();const latest=runs.value.find(r=>r.kind==='preview');if(latest?.input){questions.value=latest.input.questions.filter(q=>q!==props.proposal.question).join('\n');required.value=(latest.input.required||[]).join('\n');forbidden.value=(latest.input.forbidden||[]).join('\n')}try{const r=await listAgents();if(!disposed)agents.value=(r.data||[]).filter(a=>a.config?.agent_mode==='quick-answer').map(a=>({id:a.id,name:a.id==='builtin-quick-answer'?'快速问答':a.name}))}catch{error.value='智能体列表读取失败，请刷新'}},{immediate:true})
onBeforeUnmount(()=>{disposed=true;++request;if(timer)clearTimeout(timer)})
</script>
<style scoped>
.replay-panel{margin:20px 0;padding:18px;border:1px solid var(--td-component-border);border-radius:8px}.replay-panel p{line-height:1.7;color:var(--td-text-color-secondary);font-size:13px}label{display:flex;flex-direction:column;gap:8px;margin:14px 0}summary{cursor:pointer;line-height:1.8;padding:8px 0}details{margin:12px 0}.run{margin-top:18px;padding-top:18px;border-top:1px solid var(--td-component-border)}.run-heading{display:flex;align-items:center;gap:10px}.failure,.source{white-space:pre-wrap;overflow-wrap:anywhere}.failure{color:var(--td-error-color)!important}.answer{overflow-wrap:anywhere;line-height:1.8}.answer :deep(pre){white-space:pre-wrap;background:var(--td-bg-color-secondarycontainer);padding:12px}.answer :deep(img){max-width:100%}.note{font-size:12px}
</style>
