<template>
  <main v-if="auth.hasRole('admin')" class="feedback-detail-page">
    <header class="page-header">
      <div><t-button variant="text" @click="backToList"><template #icon><t-icon name="chevron-left" /></template>返回列表</t-button><h1>反馈查看与处理</h1></div>
      <span class="muted">当前空间：{{ auth.currentTenantName }}</span>
    </header>
    <t-alert v-if="error" theme="error" :message="error" />
    <t-loading :loading="detailLoading" class="detail-loading"><template v-if="selected">
          <div class="detail-meta"><t-tag :theme="selected.rating === 'useful' ? 'success' : 'warning'" variant="light">{{ selected.source === 'admin_review' ? '主动审核' : ratingLabel(selected.rating) }}</t-tag>
            <t-tag variant="light">{{ statusLabel(selected.status) }}</t-tag><span>{{ selected.source === 'admin_review' ? '管理员审核' : selected.source === 'wecom' ? '企业微信' : 'Web' }} · {{ formatTime(selected.created_at) }}</span></div>
          <div class="reporter-row">反馈人：<strong>{{ selected.user_name || '未备注姓名' }}</strong>
            <t-button v-if="selected.source === 'wecom'" variant="text" size="small" @click="editReporter">备注姓名</t-button><span class="muted">{{ selected.user_id.replace(/^wecom:/, '') }}</span></div>
          <div v-if="reporterOpen" class="inline-form"><t-input v-model="reporterName" placeholder="姓名" :maxlength="100" /><t-button :loading="busy" @click="saveReporter">保存姓名</t-button><t-button variant="text" @click="reporterOpen = false">取消</t-button></div>
          <div class="detail-columns"><div class="original-column"><section class="detail-section"><h3>用户问题</h3><p class="preserve">{{ selected.question }}</p></section>
          <section class="detail-section"><h3>用户补充</h3><p class="preserve">{{ selected.reason || '未填写补充说明' }}</p></section>
          <details class="original-answer" open><summary>原回答</summary><div class="answer-markdown" v-html="originalHTML" /></details>
          <details v-if="references.length" class="original-answer"><summary>引用知识（{{ references.length }}）</summary>
            <section v-for="(item, index) in references" :key="index" class="reference-item"><strong>{{ item.knowledge_title || item.knowledge_id || '知识片段' }}</strong><p class="preserve">{{ item.content }}</p></section></details>
          </div><div class="processing-column"><section v-if="selected.rating === 'useful' && !correctionEnabled && ['pending', 'reviewed'].includes(selected.status)" class="useful-panel">
            <h3>这是一条有用反馈</h3><p>可标记已阅；如审核后发现答案有误，可转入纠错，用户原始评价会保留。</p>
            <div class="actions"><t-button v-if="selected.status === 'pending'" :loading="busy" @click="review('acknowledge')">标记已阅</t-button>
              <t-button variant="outline" :disabled="busy" @click="review('correct')">转入纠错处理</t-button></div></section>
          <section v-if="correctionEnabled" class="correction-panel">
            <h3>知识纠错</h3>
            <div class="review-fields"><label>目标知识库<t-select v-model="kbID" :disabled="!!proposal || busy" placeholder="选择知识库" aria-label="目标知识库">
              <t-option v-for="kb in knowledgeBases" :key="kb.id" :value="kb.id" :label="kb.name" /></t-select></label>
              <label>知识标题<t-input v-model="title" :disabled="busy" placeholder="留空使用问题作为标题" /></label></div>
            <div v-if="!proposal" class="actions"><t-button :loading="busy" @click="createDraft">创建纠错草稿</t-button></div>
            <template v-else>
              <p class="muted">草稿状态：{{ proposalStatusLabel(proposal.status) }}。请核实业务事实后发布。</p>
              <FeedbackAnswerEditor :key="proposal.id" v-model="answer" :disabled="busy" />
              <p v-if="dirty" class="changed-notice">答案有未保存的修改{{ regression ? '，原测试结果已过期，请重新测试' : '' }}。</p>
              <t-alert v-if="proposal.conflict_summary" theme="warning" :message="proposal.conflict_summary" />
              <t-checkbox v-if="proposal.conflict_status === 'confirmed'" v-model="confirmConflict" :disabled="busy">已核对冲突，确认以本次答案替代旧纠错版本</t-checkbox>
              <KnowledgeReplayPanel :proposal="proposal" :answer="answer" :disabled="busy" @update="updateReplay"/>
              <div class="actions"><t-button variant="outline" :loading="busy" @click="saveDraft">保存草稿</t-button>
                <t-button :disabled="busy || !testCurrent || (proposal.conflict_status === 'confirmed' && !confirmConflict)" @click="publish">发布新版本</t-button></div>
            </template>
            <t-button variant="text" :disabled="busy" @click="review('ignore')">忽略该反馈</t-button>
          </section>
          <section v-if="selected.status === 'resolved'" class="detail-section"><h3>发布记录</h3><p>纠错知识：{{ selected.correction_knowledge_id }}</p>
            <details v-if="proposal?.candidate_answer" class="original-answer" open><summary>已发布的标准答案</summary><div class="answer-markdown" v-html="publishedHTML" /></details>
            <p v-if="proposal?.supersedes_knowledge_id">替代旧版本：{{ proposal.supersedes_knowledge_id }}</p><p v-else>新增纠错知识；原始上传文档保持不变。</p>
            <KnowledgeReplayPanel v-if="proposal?.status==='published'" :proposal="proposal" :answer="proposal.candidate_answer" live :disabled="busy"/>
            <t-button v-if="proposal?.supersedes_knowledge_id && proposal.status === 'published'" variant="outline" theme="warning" :disabled="busy" @click="rollbackOpen = true">回滚此版本</t-button></section>
          <p v-if="proposal?.status === 'rejected'" class="muted">该纠错版本已回滚，旧版本已恢复。</p>
          <section v-if="['ignored', 'withdrawn'].includes(selected.status)" class="detail-section"><h3>处理记录</h3><p>{{ selected.status === 'ignored' ? '该反馈已忽略，可返回列表查看其他待处理反馈。' : '用户已撤回该反馈，原始问题与回答保留供查阅。' }}</p></section>
          <div v-if="rollbackOpen" class="inline-confirm"><p>回滚将停用本次纠错知识并恢复旧版本。</p><div class="actions"><t-button theme="warning" :loading="busy" @click="rollback">确认回滚</t-button><t-button variant="outline" @click="rollbackOpen = false">取消</t-button></div></div>
          </div></div>

    </template></t-loading>
    <footer class="detail-footer"><span class="muted">{{ dirty ? '有未保存的修改' : '返回列表将保留筛选条件、页码和滚动位置' }}</span><t-button variant="outline" :disabled="busy" @click="backToList">返回列表</t-button></footer>
    <section v-if="leaveOpen" ref="leaveElement" class="leave-confirm" role="alert">
      <strong>有未保存的答案</strong><p>请选择保存后返回，或放弃本次修改。</p><div class="actions"><t-button :loading="busy" @click="saveAndLeave">保存并返回</t-button><t-button variant="outline" @click="decideLeave(true)">放弃修改并返回</t-button><t-button variant="text" @click="decideLeave(false)">继续编辑</t-button></div>
    </section>
  </main>
</template>
<script setup lang="ts">
import { computed, nextTick, onMounted, onBeforeUnmount, ref, watch } from 'vue'
import { onBeforeRouteLeave, useRoute, useRouter } from 'vue-router'
import { MessagePlugin } from 'tdesign-vue-next'
import { marked } from 'marked'
import { sanitizeHTML } from '@/utils/security'
import { useAuthStore } from '@/stores/auth'
import { useUIStore } from '@/stores/ui'
import FeedbackAnswerEditor from '@/components/FeedbackAnswerEditor.vue'
import KnowledgeReplayPanel from '@/components/KnowledgeReplayPanel.vue'
import { listKnowledgeBases } from '@/api/knowledge-base'
import { getAnswerFeedback, reviewAnswerFeedback, setFeedbackReporterName, createEvolutionDraft,
  saveEvolutionDraft, publishEvolutionProposal, rollbackEvolutionProposal,
  type AnswerFeedback, type KnowledgeEvolutionProposal, type KnowledgeRegressionRun } from '@/api/answer-feedback'
import { feedbackStatuses as statusOptions } from './feedbackListState'
const auth = useAuthStore(), route = useRoute(), router = useRouter()
const detailLoading = ref(false), busy = ref(false), error = ref('')
const selected = ref<AnswerFeedback | null>(null), proposal = ref<KnowledgeEvolutionProposal | null>(null)
const regression = ref<KnowledgeRegressionRun | null>(null)
const references = ref<Array<{ knowledge_id?: string; knowledge_title?: string; content?: string }>>([])
const knowledgeBases = ref<Array<{ id: string; name: string }>>([])
const answer = ref(''), savedAnswer = ref(''), testedAnswer = ref(''), kbID = ref(''), title = ref('')
const confirmConflict = ref(false), reporterOpen = ref(false), reporterName = ref(''), rollbackOpen = ref(false)
const leaveOpen = ref(false), leaveElement = ref<HTMLElement>()
let detailRequest = 0, leaveResolve: ((allowed: boolean) => void) | null = null
const dirty = computed(() => answer.value.trim() !== savedAnswer.value.trim())
const testCurrent = computed(() => regression.value?.kind === 'preview' && regression.value?.input?.expected_behavior !== 'clarification' && regression.value?.status === 'passed' && answer.value.trim() === testedAnswer.value.trim())
function updateReplay(result:{proposal:KnowledgeEvolutionProposal;run:KnowledgeRegressionRun}){proposal.value=result.proposal;regression.value=result.run;testedAnswer.value=result.run.input?.answer||'';savedAnswer.value=result.proposal.candidate_answer}
const correctionEnabled = computed(() => !!selected.value && ['pending', 'correcting'].includes(selected.value.status) &&
  (selected.value.rating === 'not_useful' || selected.value.status === 'correcting') && !['published', 'publishing', 'rejected'].includes(proposal.value?.status || ''))
const originalHTML = computed(() => sanitizeHTML(marked.parse(selected.value?.answer || '', { async: false }) as string))
const publishedHTML = computed(() => sanitizeHTML(marked.parse(proposal.value?.candidate_answer || '', { async: false }) as string))

async function loadKBs() {
  try { const result: any = await listKnowledgeBases(); knowledgeBases.value = Array.isArray(result?.data) ? result.data : Array.isArray(result) ? result : [] }
  catch { MessagePlugin.warning('知识库列表加载失败，请刷新后重试') }
}
async function openDetail(id: string) {
  const request = ++detailRequest
  selected.value = null; proposal.value = null; detailLoading.value = true
  try {
    const data = await getAnswerFeedback(id)
    if (request !== detailRequest) return
    selected.value = data.data; proposal.value = data.proposal; regression.value = data.regression; references.value = data.references || []
    answer.value = data.proposal?.candidate_answer || ''; savedAnswer.value = answer.value; testedAnswer.value = data.regression ? answer.value : ''
    kbID.value = data.proposal?.knowledge_base_id || data.data.knowledge_base_id || knowledgeBases.value.find(k => k.name === 'XBROTHER')?.id || ''
    title.value = ''; confirmConflict.value = false
  } catch (err: any) { MessagePlugin.error(err?.message || '读取详情失败'); error.value = err?.message || '读取详情失败' }
  finally { if (request === detailRequest) detailLoading.value = false }
}

async function canLeave(): Promise<boolean> {
  if (busy.value) { MessagePlugin.warning('正在处理，请稍候'); return false }
  if (!dirty.value) return true
  if (leaveResolve) return false
  leaveOpen.value = true
  nextTick(() => leaveElement.value?.scrollIntoView({ block: 'nearest' }))
  return new Promise(resolve => { leaveResolve = resolve })
}
function decideLeave(allowed: boolean) {
  leaveOpen.value = false
  if (allowed) answer.value = savedAnswer.value
  const resolve = leaveResolve; leaveResolve = null; resolve?.(allowed)
}
async function saveAndLeave() { await saveDraft(); if (!dirty.value) decideLeave(true) }
function backToList() { return router.push({ name: 'answerFeedback', query: route.query }) }
onBeforeRouteLeave(() => canLeave())
async function perform(action: () => Promise<void>) {
  if (busy.value) return
  busy.value = true
  try { await action() } catch (err: any) { MessagePlugin.error(err?.message || '操作失败') } finally { busy.value = false }
}
async function createDraft() {
  if (!selected.value || !kbID.value) { MessagePlugin.warning('请选择目标知识库'); return }
  await perform(async () => {
    const result = await createEvolutionDraft(selected.value!.id, kbID.value)
    proposal.value = result.data; answer.value = result.data.candidate_answer; savedAnswer.value = answer.value; regression.value = null; testedAnswer.value = ''
    selected.value!.status = 'correcting'
    MessagePlugin.success('草稿已创建，请审核并编辑答案')
  })
}
async function saveDraft() {
  if (!proposal.value) return
  await perform(async () => { const result = await saveEvolutionDraft(proposal.value!.id, answer.value.trim()); proposal.value = result.data; savedAnswer.value = answer.value
    if (!result.data.regression_run_id) regression.value = null
    MessagePlugin.success('草稿已保存') })
}
async function publish() {
  if (!proposal.value || !testCurrent.value) return
  await perform(async () => {
    const result = await publishEvolutionProposal(proposal.value!.id, { answer: answer.value.trim(), title: title.value.trim(), confirm_conflict: confirmConflict.value })
    proposal.value = result.data; savedAnswer.value = answer.value; selected.value!.status = 'resolved'; selected.value!.correction_knowledge_id = result.knowledge_id
    MessagePlugin.success('已提交发布，请等待知识索引完成')
  })
}
async function review(action: 'ignore' | 'acknowledge' | 'correct') {
  if (!selected.value || (action === 'ignore' && !(await canLeave()))) return
  await perform(async () => {
    const result = await reviewAnswerFeedback(selected.value!.id, { action }); selected.value = { ...selected.value!, ...result.data }; savedAnswer.value = answer.value
    MessagePlugin.success(action === 'acknowledge' ? '已标记已阅' : action === 'correct' ? '已转入纠错，原始评价保持不变' : '已忽略')
  })
}
function editReporter() { reporterName.value = selected.value?.user_name || ''; reporterOpen.value = true }
async function saveReporter() { if (!selected.value) return
  await perform(async () => { await setFeedbackReporterName(selected.value!.id, reporterName.value.trim()); selected.value!.user_name = reporterName.value.trim(); reporterOpen.value = false }) }
async function rollback() {
  if (!proposal.value) return
  await perform(async () => { await rollbackEvolutionProposal(proposal.value!.id); proposal.value!.status = 'rejected'; rollbackOpen.value = false; MessagePlugin.success('已恢复旧版本') })
}
function ratingLabel(value: string) { return value === 'useful' ? '有用' : value === 'not_useful' ? '无用' : '已撤回' }
function statusLabel(value: string) { return statusOptions.find(item => item.value === value)?.label || value }
function proposalStatusLabel(value: string) { return ({ draft: '待审核', conflict: '存在冲突', ready: '待发布', testing: '测试中', publishing: '发布中', published: '已发布', rejected: '已回滚', failed: '失败' } as Record<string,string>)[value] || value }
function formatTime(value: string) { return new Date(value).toLocaleString('zh-CN', { hour12: false }) }

watch(() => auth.effectiveTenantId, () => { ++detailRequest; answer.value = savedAnswer.value; selected.value = null; router.replace('/platform/answer-feedback') })
watch(() => route.params.id, async id => { if (id) { await loadKBs(); await openDetail(String(id)) } })
onBeforeUnmount(() => { ++detailRequest; leaveResolve?.(false) })
onMounted(async () => { useUIStore().closeSettings(); await loadKBs(); await openDetail(String(route.params.id)) })
</script>
<style scoped lang="less">
.feedback-detail-page { flex: 1; min-width: 0; height: 100%; box-sizing: border-box; padding: 28px 32px; overflow: auto; background: var(--td-bg-color-container); }
.page-header { display: flex; align-items: center; justify-content: space-between; gap: 16px; margin-bottom: 24px; }
.page-header h1 { margin: 10px 0 0; font-size: 24px; }
.muted { font-size: 13px; color: var(--td-text-color-secondary); }
.detail-columns { display: grid; grid-template-columns: minmax(0, 2fr) minmax(0, 3fr); gap: 24px; align-items: start; margin-top: 20px; }
.original-column, .processing-column { min-width: 0; padding: 20px; border: 1px solid var(--td-component-border); border-radius: 8px; }
.detail-footer { display: flex; justify-content: space-between; align-items: center; gap: 16px; border-top: 1px solid var(--td-component-border); margin-top: 24px; padding-top: 18px; }
.actions { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; }
.inline-form { display: flex; align-items: center; gap: 10px; max-width: 520px; padding: 12px 0; }
.inline-confirm, .leave-confirm { background: var(--td-bg-color-secondarycontainer); border: 1px solid var(--td-component-border); border-radius: 8px; padding: 16px; margin-top: 16px; }
.leave-confirm { position: sticky; bottom: 0; z-index: 2; }
.detail-loading { display: block; min-height: 160px; }
.detail-heading { display: flex; align-items: center; justify-content: space-between; width: 100%; }
.detail-meta, .reporter-row { display: flex; flex-wrap: wrap; gap: 8px; align-items: center; margin-bottom: 12px; overflow-wrap: anywhere; }
.detail-meta { color: var(--td-text-color-secondary); font-size: 13px; }
.detail-section, .original-answer { margin: 20px 0; }
.detail-section h3, .correction-panel h3 { font-size: 15px; margin: 0 0 12px; }
.preserve { white-space: pre-wrap; overflow-wrap: anywhere; line-height: 1.7; }
.original-answer { border: 1px solid var(--td-component-border); border-radius: 8px; padding: 14px; }
.original-answer summary { cursor: pointer; font-weight: 600; }
.answer-markdown { line-height: 1.7; overflow-wrap: anywhere; }
.answer-markdown :deep(img) { max-width: 100%; }
.answer-markdown :deep(pre) { white-space: pre-wrap; padding: 10px; background: var(--td-bg-color-secondarycontainer); }
.answer-markdown :deep(table) { border-collapse: collapse; width: 100%; }
.answer-markdown :deep(th), .answer-markdown :deep(td) { border: 1px solid var(--td-component-border); padding: 6px; }
.reference-item { border-top: 1px solid var(--td-component-border); padding-top: 12px; margin-top: 12px; }
.useful-panel { padding: 18px; border-radius: 8px; background: var(--td-success-color-1); }
.correction-panel { display: grid; gap: 14px; }
.review-fields { display: grid; grid-template-columns: 1fr 1fr; gap: 14px; }
.review-fields label { display: grid; gap: 6px; }
.changed-notice { color: var(--td-warning-color); font-size: 13px; margin: 0; }

@media (max-width: 1050px) { .detail-columns { grid-template-columns: 1fr; } }
@media (max-width: 800px) { .feedback-detail-page { padding: 18px; } .review-fields { grid-template-columns: 1fr; } }
</style>
