<template>
  <main v-if="auth.hasRole('admin')" ref="pageElement" class="answer-feedback-page">
    <header class="page-header">
      <div><h1>回答反馈</h1><p>查看用户评价，审核纠错并更新知识。当前空间：{{ auth.currentTenantName }}</p></div>
      <t-button variant="outline" :loading="loading" @click="load"><template #icon><t-icon name="refresh" /></template>刷新</t-button>
    </header>
    <nav class="status-tabs" aria-label="处理状态">
      <button v-for="tab in feedbackStatuses" :key="tab.value" type="button" :class="{ active: applied.status === tab.value }" :aria-pressed="applied.status === tab.value" @click="selectStatus(tab.value)">
        <t-icon :name="tab.icon" />{{ tab.label }}<span>({{ counts[tab.value] || 0 }})</span>
      </button>
    </nav>
    <div ref="filterElement" class="filter-scroll">
      <form class="feedback-filters" aria-label="反馈筛选" @submit.prevent="search">
        <label class="question-filter">用户问题<t-input v-model="draft.keyword" placeholder="搜索问题关键词" clearable aria-label="用户问题关键词" /></label>
        <label>反馈类型<t-select v-model="draft.rating" aria-label="反馈类型"><t-option value="all" label="全部评价" /><t-option value="not_useful" label="无用" /><t-option value="useful" label="有用" /></t-select></label>
        <label>来源<t-select v-model="draft.source" aria-label="来源"><t-option value="" label="全部来源" /><t-option value="web" label="Web" /><t-option value="wecom" label="企业微信" /></t-select></label>
        <label class="reporter-filter">反馈人<t-input v-model="draft.reporter" placeholder="姓名或 ID" clearable aria-label="反馈人" /></label>
        <label class="date-filter">反馈时间<t-date-range-picker v-model="dateRange" :placeholder="['开始日期', '结束日期']" clearable format="YYYY-MM-DD" value-type="YYYY-MM-DD" :presets="datePresets" :popup-props="{ attach: 'body' }" /></label>
        <div class="filter-actions"><t-button type="submit">查询</t-button><t-button variant="outline" @click="resetFilters">重置</t-button></div>
      </form>
    </div>
    <t-alert v-if="error" theme="error" :message="error" />
    <div ref="tableElement" class="table-wrap">
      <t-table row-key="id" :data="rows" :columns="columns" :loading="loading" hover :bordered="false" table-layout="fixed">
        <template #created_at="{ row }">{{ formatTime(row.created_at) }}</template>
        <template #reporter="{ row }">{{ row.user_name || row.user_id || '未备注姓名' }}</template>
        <template #rating="{ row }"><t-tag :theme="row.rating === 'useful' ? 'success' : 'warning'" variant="light">{{ row.source === 'admin_review' ? '主动审核' : row.rating === 'useful' ? '有用' : row.rating === 'not_useful' ? '无用' : '已撤回' }}</t-tag></template>
        <template #question="{ row }">{{ row.question }}</template>
        <template #source="{ row }">{{ row.source === 'admin_review' ? '管理员审核' : row.source === 'wecom' ? '企业微信' : 'Web' }}</template>
        <template #status="{ row }"><t-tag variant="light">{{ feedbackStatuses.find(item => item.value === row.status)?.label || row.status }}</t-tag></template>
        <template #operation="{ row }"><t-button variant="text" theme="primary" size="small" @click="openDetail(row.id)">{{ ['pending', 'correcting'].includes(row.status) ? '处理' : '查看' }}</t-button></template>
        <template #empty><t-empty description="当前筛选条件下暂无反馈" /></template>
      </t-table>
    </div>
    <footer class="pagination-row"><span>共 {{ total }} 条</span><t-pagination :current="applied.page" :page-size="applied.pageSize" :total="total" :total-content="false" :page-size-options="[20, 50, 100]" @change="changePage" /></footer>
  </main>
</template>

<script setup lang="ts">
import { nextTick, onMounted, onBeforeUnmount, reactive, ref, watch } from 'vue'
import { onBeforeRouteLeave, useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { useUIStore } from '@/stores/ui'
import { tableTooltip } from '@/utils/tableTooltip'
import { listAnswerFeedback, type AnswerFeedback } from '@/api/answer-feedback'
import { feedbackStatuses, feedbackListState, feedbackListQuery, feedbackScrollKey, type FeedbackListState } from './feedbackListState'
const auth = useAuthStore(), route = useRoute(), router = useRouter()
const applied = reactive(feedbackListState(route.query)), draft = reactive({ ...applied })
const dateRange = ref<string[]>([applied.start, applied.end].filter(Boolean))
const rows = ref<AnswerFeedback[]>([]), counts = ref<Record<string, number>>({}), total = ref(0), loading = ref(false), error = ref('')
const pageElement = ref<HTMLElement>(), filterElement = ref<HTMLElement>(), tableElement = ref<HTMLElement>()
const columns = [{ colKey: 'question', title: '用户问题', minWidth: 260 }, { colKey: 'rating', title: '评价', width: 85 },
  { colKey: 'reporter', title: '反馈人', width: 120 }, { colKey: 'source', title: '来源', width: 100 },
  { colKey: 'created_at', title: '反馈时间', width: 170 }, { colKey: 'status', title: '处理状态', width: 100 }, { colKey: 'operation', title: '操作', width: 96 }]
  .map(column => ({ ...column, ellipsisTitle: tableTooltip, ...(column.colKey === 'operation'
    ? { align: 'center' as const, className: 'table-operation' }
    : { ellipsis: tableTooltip }) }))
function recentDays(days: number) { const end = new Date(), start = new Date(); start.setDate(start.getDate() - days + 1); return [start, end] }
const datePresets = { '最近 7 天': () => recentDays(7), '最近 30 天': () => recentDays(30) }
let requestID = 0, restoring = false
function persistPosition() {
  if (restoring) return
  try { sessionStorage.setItem(feedbackScrollKey(auth.effectiveTenantId, feedbackListQuery(applied)), JSON.stringify({
    y: pageElement.value?.scrollTop || 0, x: tableElement.value?.querySelector('.t-table__content')?.scrollLeft || 0,
    filterX: filterElement.value?.scrollLeft || 0, draft: { ...draft }, dates: dateRange.value,
  })) } catch { /* Browser storage can be disabled. */ }
}
async function restorePosition() {
  try {
    const saved = JSON.parse(sessionStorage.getItem(feedbackScrollKey(auth.effectiveTenantId, feedbackListQuery(applied))) || 'null')
    if (!saved) return
    restoring = true
    if (saved.draft) Object.assign(draft, feedbackListState(saved.draft))
    if (Array.isArray(saved.dates)) dateRange.value = saved.dates
    await nextTick()
    if (pageElement.value) pageElement.value.scrollTop = Number(saved.y) || 0
    if (filterElement.value) filterElement.value.scrollLeft = Number(saved.filterX) || 0
    const table = tableElement.value?.querySelector('.t-table__content'); if (table) table.scrollLeft = Number(saved.x) || 0
  } catch { /* Ignore obsolete snapshots. */ } finally { restoring = false }
}
async function load() {
  if (!auth.hasRole('admin')) return
  const id = ++requestID; loading.value = true; error.value = ''
  try {
    const from = applied.start ? new Date(applied.start + 'T00:00:00') : null, to = applied.end ? new Date(applied.end + 'T00:00:00') : null
    if (from && to && from > to) throw new Error('开始日期不能晚于结束日期')
    if (to) to.setDate(to.getDate() + 1)
    const response = await listAnswerFeedback({ ...applied, page_size: applied.pageSize, from: from?.toISOString() || '', to: to?.toISOString() || '' })
    if (id !== requestID) return
    rows.value = response.data || []; total.value = response.total; counts.value = response.status_counts || {}
    await restorePosition()
  } catch (err: any) { if (id === requestID) error.value = err?.message || '读取反馈失败' }
  finally { if (id === requestID) loading.value = false }
}
async function navigateList(state: FeedbackListState) {
  persistPosition()
  const query = feedbackListQuery(state)
  try { sessionStorage.removeItem(feedbackScrollKey(auth.effectiveTenantId, query)) } catch {}
  if (JSON.stringify(state) === JSON.stringify({ ...applied })) { Object.assign(draft, state); dateRange.value = [state.start, state.end].filter(Boolean); await load(); return }
  await router.replace({ name: 'answerFeedback', query })
}
function search() { return navigateList({ ...applied, ...draft, status: applied.status, start: dateRange.value[0] || '', end: dateRange.value[1] || '', page: 1 }) }
function resetFilters() { return navigateList({ ...feedbackListState({}), status: applied.status }) }
function selectStatus(status: string) { return navigateList({ ...applied, status, page: 1 }) }
function changePage(info: { current: number; pageSize: number }) { return navigateList({ ...applied, page: info.pageSize !== applied.pageSize ? 1 : info.current, pageSize: info.pageSize }) }
async function openDetail(id: string) { persistPosition(); await router.push({ name: 'answerFeedbackDetail', params: { id }, query: feedbackListQuery(applied) }) }
function formatTime(value: string) { return new Date(value).toLocaleString('zh-CN', { hour12: false }) }
watch(() => route.query, async query => { Object.assign(applied, feedbackListState(query)); Object.assign(draft, applied); dateRange.value = [applied.start, applied.end].filter(Boolean); await load() })
watch(() => auth.effectiveTenantId, async () => { ++requestID; rows.value = []; counts.value = {}; if (!auth.hasRole('admin')) { await router.replace('/platform/knowledge-bases'); return } await navigateList(feedbackListState({})) })
onBeforeRouteLeave(() => { persistPosition() })
onBeforeUnmount(() => { ++requestID })
onMounted(async () => { useUIStore().closeSettings(); await load() })
</script>

<style scoped lang="less">
.answer-feedback-page { flex: 1; min-width: 0; height: 100%; box-sizing: border-box; padding: 28px 32px; overflow: auto; background: var(--td-bg-color-container); }
.page-header { display: flex; align-items: center; justify-content: space-between; gap: 16px; margin-bottom: 20px; }
.page-header h1 { font-size: 24px; margin: 0 0 8px; }
.page-header p { font-size: 13px; color: var(--td-text-color-secondary); margin: 0; }
.status-tabs { display: flex; flex-wrap: nowrap; gap: 26px; overflow-x: auto; border-bottom: 1px solid var(--td-component-border); margin-bottom: 20px; }
.status-tabs button { display: inline-flex; align-items: center; gap: 6px; flex: 0 0 auto; padding: 12px 0; border: 0; border-bottom: 2px solid transparent; background: none; color: var(--td-text-color-secondary); cursor: pointer; font-size: 14px; }
.status-tabs button.active { color: var(--td-brand-color); border-bottom-color: var(--td-brand-color); }
.filter-scroll { overflow-x: auto; margin-bottom: 20px; background: var(--td-bg-color-secondarycontainer); border-radius: 8px; }
.feedback-filters { display: flex; flex-wrap: nowrap; gap: 16px; align-items: flex-end; padding: 16px; min-width: 1160px; box-sizing: border-box; }
.feedback-filters label { flex: 0 0 112px; min-width: 0; display: flex; flex-direction: column; gap: 7px; font-size: 12px; color: var(--td-text-color-secondary); }
.feedback-filters label :deep(.t-select__wrap), .feedback-filters label :deep(.t-input__wrap), .feedback-filters label :deep(.t-date-range-picker) { width: 100%; min-width: 0; }
.feedback-filters .question-filter { flex: 1 0 190px; }
.feedback-filters .reporter-filter { flex-basis: 138px; }
.feedback-filters .date-filter { flex-basis: 280px; }
.filter-actions { display: flex; flex: 0 0 auto; gap: 10px; }
.table-wrap { min-width: 0; margin-top: 16px; }
.table-wrap :deep(.t-table) { min-width: 0; }
.table-wrap :deep(table) { min-width: 940px; }
.table-wrap :deep(th), .table-wrap :deep(td) { white-space: nowrap; vertical-align: middle; }
.table-wrap :deep(.table-operation .t-table__cell-inner) { justify-content: center; }
.pagination-row { display: flex; justify-content: flex-end; align-items: center; flex-wrap: nowrap; gap: 16px; margin-top: 20px; white-space: nowrap; font-size: 13px; color: var(--td-text-color-secondary); }
.pagination-row :deep(.t-pagination) { width: auto; flex: 0 1 auto; }
@media (max-width: 800px) { .answer-feedback-page { padding: 18px; } }
</style>
