<template>
  <main v-if="auth.hasRole('admin')" ref="pageEl" class="insights-page">
    <header><div><t-button v-if="topicId" variant="text" @click="backTopics">‹ 返回问题排行</t-button><h1>{{ topicId ? topicTitle : '问答统计' }}</h1><p>当前空间：{{ auth.currentTenantName }} · 按使用身份统计，尚未关联的 Web 与企业微信身份分别计数。</p></div><t-button variant="outline" :loading="loading" @click="load">刷新</t-button></header>
    <div v-if="!topicId" class="memory-overview">企业微信记忆：{{ memory.subjects }} 个身份 · {{ memory.items }} 条生效记忆 · {{ memory.pending }} 个待提炼会话 · {{ memory.failed }} 个提炼异常记录<t-button theme="primary" variant="text" @click="router.push('/platform/qa-insights/memory')">员工记忆与诊断</t-button></div>
    <nav v-if="!topicId" class="tabs"><button :class="{active:state.tab==='topics'}" @click="setTab('topics')">高频问题</button><button :class="{active:state.tab==='questions'}" @click="setTab('questions')">问答明细</button></nav>
    <div class="filter-scroll"><form @submit.prevent="search">
      <label class="keyword">用户问题<t-input v-model="draft.keyword" placeholder="搜索问题关键词" clearable /></label>
      <label>来源<t-select v-model="draft.source"><t-option value="" label="全部来源"/><t-option value="wecom" label="企业微信"/><t-option value="web" label="Web"/><t-option value="api" label="API"/></t-select></label>
      <label>会话类型<t-select v-model="draft.chat_type"><t-option value="" label="全部"/><t-option value="group" label="群聊"/><t-option value="direct" label="私聊"/></t-select></label>
      <label class="reporter">提问人<t-input v-model="draft.reporter" placeholder="姓名或 ID" clearable/></label>
      <label class="date">提问时间<t-date-range-picker v-model="dateRange" format="YYYY-MM-DD" value-type="YYYY-MM-DD" :placeholder="['开始日期','结束日期']" :popup-props="{attach:'body'}" :presets="presets" clearable /></label>
      <template v-if="isTopics"><label>显示范围<t-select v-model="draft.frequent"><t-option value="true" label="高频问题"/><t-option value="false" label="全部排行"/></t-select></label><label class="number">至少提问<t-input-number v-model="draft.min_count" :min="1" :max="100000" theme="normal"/></label><label class="number">至少身份<t-input-number v-model="draft.min_people" :min="1" :max="100000" theme="normal"/></label></template>
      <label v-else>归类状态<t-select v-model="draft.classification"><t-option value="" label="全部"/><t-option value="classified" label="已归类"/><t-option value="pending" label="待归类"/><t-option value="unclassified" label="信息不足"/></t-select></label>
      <div class="actions"><t-button type="submit">查询</t-button><t-button variant="outline" @click="reset">重置</t-button></div>
    </form></div>
    <p class="hint">{{ isTopics ? '默认最近 7 天，至少提问 3 次且涉及 2 个身份。近义归类为辅助判断，可进入主题合并或拆分。' : '从本版本上线后记录，不自动回填历史；同一条消息的重复推送不重复计数。' }}</p>
    <t-alert v-if="error" theme="error" :message="error"/>
    <div v-if="!isTopics" class="toolbar"><span>已选择 {{ selected.length }} 条</span><t-button variant="outline" :disabled="selected.length!==2" @click="compare">比较两次回答</t-button><t-button v-if="topicId" variant="text" @click="editMode=editMode?'':'split'">拆分所选问答</t-button><t-button v-if="topicId" variant="text" @click="openMerge">合并到其他主题</t-button></div>
    <section v-if="editMode" class="edit-panel"><h3>{{editMode==='split'?'拆分为新主题':'合并当前主题'}}</h3><p>{{editMode==='split'?'仅移动已勾选的问答，保留原始问答和反馈。':'当前主题的全部问答将归入目标主题，后续相同问法也使用该归类。'}}</p><t-input v-if="editMode==='split'" v-model="newTitle" placeholder="新主题名称，最多 200 字" :maxlength="200"/><t-select v-else v-model="mergeTarget" filterable placeholder="选择目标主题"><t-option v-for="t in mergeOptions" :key="t.id" :value="t.id" :label="t.title"/></t-select><div class="edit-actions"><t-button :loading="saving" @click="saveClassification">确认{{editMode==='split'?'拆分':'合并'}}</t-button><t-button variant="outline" @click="editMode=''">取消</t-button></div></section>
    <div class="table-wrap"><t-table row-key="id" :data="rows" :columns="columns" :loading="loading" hover table-layout="fixed" :selected-row-keys="selected" @select-change="selectRows">
      <template #title="{row}">{{row.title}}</template>
      <template #question="{row}">{{row.question}}</template>
      <template #reporter="{row}">{{row.user_name || row.user_id}}</template>
      <template #source="{row}">{{qaSource(row.source)}}{{row.chat_type==='group'?' · 群聊':row.chat_type==='direct'?' · 私聊':''}}</template>
      <template #classification="{row}">{{({pending:'待归类',classified:'已归类',unclassified:'信息不足'} as Record<string,string>)[row.classification]}}</template>
      <template #created_at="{row}">{{qaTime(row.created_at)}}</template><template #last_asked="{row}">{{qaTime(row.last_asked)}}</template>
      <template #operation="{row}"><t-button variant="text" theme="primary" size="small" @click="openRow(row.id)">查看</t-button></template>
      <template #empty><t-empty :description="isTopics?'当前条件下暂无高频问题，可切换“全部排行”查看':'暂无问答记录'"/></template>
    </t-table></div>
    <footer><span>共 {{total}} 条</span><t-pagination :current="state.page" :page-size="state.page_size" :total="total" :total-content="false" :page-size-options="[20,50,100]" @change="changePage"/></footer>
  </main>
</template>

<script setup lang="ts">
import {computed,nextTick,onMounted,onBeforeUnmount,reactive,ref,watch} from 'vue'
import {onBeforeRouteLeave,useRoute,useRouter} from 'vue-router'
import {MessagePlugin} from 'tdesign-vue-next'
import {useAuthStore} from '@/stores/auth'
import {tableTooltip} from '@/utils/tableTooltip'
import {insightList,insightMemoryStatus,insightTopic,insightReclassify,qaSource,qaTime,type QATopic,type QAQuestion} from '@/api/qa-insights'
import {qaState,qaQuery,qaParent} from './qaInsightsState'
const auth=useAuthStore(),route=useRoute(),router=useRouter()
const state=reactive(qaState(route.query)),draft=reactive({...state}),dateRange=ref<string[]>([state.start,state.end].filter(Boolean))
const topicId=computed(()=>String(route.params.topicId||'')),isTopics=computed(()=>!topicId.value && state.tab==='topics')
const topicTitle=ref('问题详情'),rows=ref<Array<QATopic|QAQuestion>>([]),total=ref(0),loading=ref(false),error=ref(''),pageEl=ref<HTMLElement>(),selected=ref<Array<string|number>>([])
const memory=reactive({subjects:0,items:0,pending:0,failed:0}),editMode=ref(''),newTitle=ref(''),mergeTarget=ref(''),mergeOptions=ref<QATopic[]>([]),saving=ref(false)
const presets={'最近 7 天':[new Date(Date.now()-6*86400000),new Date()],'最近 30 天':[new Date(Date.now()-29*86400000),new Date()]}
const columns=computed(()=>(isTopics.value?[
  {colKey:'title',title:'问题主题',ellipsis:true},{colKey:'questions',title:'提问次数',width:95},{colKey:'people',title:'使用身份数',width:110},{colKey:'useful',title:'有用反馈',width:95},{colKey:'not_useful',title:'无用反馈',width:95},{colKey:'failures',title:'异常回答',width:95},{colKey:'last_asked',title:'最近提问',width:180},{colKey:'operation',title:'操作',width:80},
]:[{colKey:'row-select',type:'multiple' as const,width:45},{colKey:'question',title:'用户问题',ellipsis:true},{colKey:'reporter',title:'提问人',width:145},{colKey:'source',title:'来源',width:135},{colKey:'classification',title:'归类状态',width:100},{colKey:'created_at',title:'提问时间',width:180},{colKey:'operation',title:'操作',width:80}])
  .map(column=>({...column,ellipsisTitle:tableTooltip,...(column.colKey==='operation'
    ? {width:96,align:'center' as const,className:'table-operation'}
    : column.colKey==='row-select'?{}:{ellipsis:tableTooltip})})))
let request=0
const positionKey=()=>`qa-insights:${auth.effectiveTenantId}:${route.path}:${JSON.stringify(qaQuery(state))}`
function persist(){if(pageEl.value)sessionStorage.setItem(positionKey(),String(pageEl.value.scrollTop))}
async function load(){const id=++request;loading.value=true;error.value='';try{const result=await insightList<QATopic|QAQuestion>(isTopics.value?'topics':'questions',{...qaQuery(state),...(topicId.value?{topic:topicId.value}:{})});if(id!==request)return;rows.value=result.data||[];total.value=result.total;await nextTick();if(pageEl.value)pageEl.value.scrollTop=Number(sessionStorage.getItem(positionKey())||0)}catch(e:any){if(id===request)error.value=e.message||'读取失败'}finally{if(id===request)loading.value=false}}
const parentQuery=()=>typeof route.query.parent==='string'?{parent:route.query.parent}:{}
function navigate(s:typeof state){persist();return router.push({path:route.path,query:{...qaQuery(s),...parentQuery()}})}
function search(){return navigate({...draft,start:String(dateRange.value?.[0]||''),end:String(dateRange.value?.[1]||''),page:1})}
function reset(){return navigate({...qaState({}),tab:state.tab})}
function setTab(tab:string){return navigate({...state,tab,page:1})}
function changePage(p:{current:number;pageSize:number}){return navigate({...state,page:p.current,page_size:p.pageSize})}
function selectRows(keys:Array<string|number>){selected.value=keys}
function detailQuery(){return {...qaQuery(state),...parentQuery(),fromTopic:topicId.value}}
async function openRow(id:string){persist();await router.push({path:isTopics.value?`/platform/qa-insights/topics/${id}`:`/platform/qa-insights/questions/${id}`,query:isTopics.value?{...qaQuery(state),page:'1',parent:JSON.stringify(qaQuery(state))}:detailQuery()})}
function compare(){persist();return router.push({path:`/platform/qa-insights/questions/${selected.value[0]}`,query:{...detailQuery(),compare:String(selected.value[1])}})}
function backTopics(){persist();return router.push({path:'/platform/qa-insights',query:qaParent(route.query)})}
async function openMerge(){editMode.value='merge';try{const result=await insightList<QATopic>('topics',{frequent:'false',page_size:100,start:'2000-01-01'});mergeOptions.value=(result.data||[]).filter(t=>t.id!==topicId.value)}catch(e:any){MessagePlugin.error(e.message||'读取主题失败')}}
async function saveClassification(){if(editMode.value==='split'&&(!selected.value.length||!newTitle.value.trim())){MessagePlugin.warning('请选择问答并填写主题');return}if(editMode.value==='merge'&&!mergeTarget.value){MessagePlugin.warning('请选择目标主题');return}saving.value=true;try{const result=await insightReclassify(topicId.value,{action:editMode.value,title:newTitle.value,target_id:mergeTarget.value,question_ids:selected.value.map(String)});editMode.value='';selected.value=[];await router.push({path:`/platform/qa-insights/topics/${result.topic_id}`,query:{...qaQuery(state),page:'1'}})}catch(e:any){MessagePlugin.error(e.message||'保存失败')}finally{saving.value=false}}
async function init(){if(topicId.value){try{topicTitle.value=(await insightTopic(topicId.value)).data.title}catch{topicTitle.value='问题主题'}}await load()}
onBeforeRouteLeave(()=>persist());onBeforeUnmount(()=>{request++})
watch(()=>route.fullPath,()=>{Object.assign(state,qaState(route.query));Object.assign(draft,state);dateRange.value=[state.start,state.end].filter(Boolean);selected.value=[];void init()})
async function loadMemory(){const tenant=auth.effectiveTenantId;try{const result=await insightMemoryStatus();if(tenant===auth.effectiveTenantId)Object.assign(memory,result)}catch{}}
watch(()=>auth.effectiveTenantId,()=>{++request;rows.value=[];selected.value=[];Object.assign(memory,{subjects:0,items:0,pending:0,failed:0});router.replace('/platform/qa-insights');void loadMemory();void init()})
onMounted(()=>{void init();void loadMemory()})
</script>

<style scoped>
.table-wrap :deep(th),.table-wrap :deep(td){white-space:nowrap;vertical-align:middle}.table-wrap :deep(.table-operation .t-table__cell-inner){justify-content:center}
.insights-page{height:100%;flex:1;min-width:0;overflow:auto;padding:28px 32px;box-sizing:border-box;background:var(--td-bg-color-container)}header{display:flex;justify-content:space-between;align-items:start;gap:24px}h1{margin:0 0 10px;font-size:24px}p,.hint{color:var(--td-text-color-secondary);font-size:13px;line-height:1.7}.memory-overview{margin:18px 0;padding:14px 18px;border-radius:8px;background:var(--td-bg-color-secondarycontainer);font-size:13px;line-height:1.8}.memory-overview span{margin-left:18px;color:var(--td-text-color-secondary)}.tabs{display:flex;gap:24px;border-bottom:1px solid var(--td-component-border);margin:20px 0}.tabs button{border:0;border-bottom:2px solid transparent;background:transparent;padding:13px 4px;cursor:pointer;font-size:14px;color:var(--td-text-color-secondary)}.tabs .active{border-color:var(--td-brand-color);color:var(--td-brand-color)}.filter-scroll{overflow-x:auto;border-radius:10px;background:var(--td-bg-color-secondarycontainer)}form{display:flex;flex-wrap:nowrap;align-items:end;gap:14px;padding:16px;min-width:1280px}label{display:flex;flex-direction:column;gap:7px;font-size:12px;flex:0 0 112px;min-width:0}.keyword{flex:1 0 170px}.reporter{flex-basis:130px}.date{flex-basis:275px}.number{flex-basis:95px}label :deep(.t-input-number),label :deep(.t-select__wrap),label :deep(.t-date-range-picker){width:100%;min-width:0}.actions,.edit-actions{display:flex;gap:10px;white-space:nowrap}.toolbar{display:flex;align-items:center;gap:12px;margin:18px 0;font-size:13px}.edit-panel{padding:20px;border:1px solid var(--td-component-border);border-radius:10px;margin-bottom:18px;max-width:700px}.edit-panel h3{margin:0}.edit-actions{margin-top:14px}.table-wrap{border:1px solid var(--td-component-border);border-radius:8px;overflow:auto}.table-wrap :deep(.t-table){min-width:960px}.ellipsis{display:block;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}.table-wrap :deep(tbody tr:nth-child(even)){background:var(--td-bg-color-secondarycontainer)}footer{display:flex;justify-content:flex-end;align-items:center;gap:20px;margin-top:18px;font-size:13px}footer :deep(.t-pagination){width:auto}
</style>
