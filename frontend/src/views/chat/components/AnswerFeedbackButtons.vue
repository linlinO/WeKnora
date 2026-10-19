<template>
  <div class="answer-feedback" role="group" aria-label="回答反馈">
    <button v-for="choice in choices" :key="choice.value" type="button" class="feedback-vote"
      :class="{ selected: rating === choice.value }" :aria-pressed="rating === choice.value"
      :title="rating === choice.value ? `取消${choice.label}` : choice.label"
      :disabled="busy || loading || !messageId" @click.stop="vote(choice.value)">
      <component :is="choice.icon" /><span>{{ choice.label }}</span>
    </button>
  </div>
  <Teleport to="body">
    <t-dialog v-model:visible="detailOpen" attach="body" placement="center" class="feedback-detail-dialog"
      header="补充反馈（可选）" width="min(440px, calc(100vw - 32px))"
      :close-on-overlay-click="!busy" :close-on-esc-keydown="!busy"
      :confirm-btn="{ content: '提交说明', loading: busy }" cancel-btn="暂不补充" @confirm="saveReason">
      <t-checkbox-group v-model="reasons" class="feedback-reasons">
        <t-checkbox v-for="reason in reasonOptions" :key="reason" :value="reason">{{ reason }}</t-checkbox>
      </t-checkbox-group>
      <t-textarea v-model="detail" placeholder="补充说明（选填）" :maxlength="2000" :autosize="{ minRows: 3, maxRows: 6 }" />
    </t-dialog>
  </Teleport>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { MessagePlugin } from 'tdesign-vue-next'
import { ThumbUpIcon, ThumbDownIcon } from 'tdesign-icons-vue-next'
import { getOwnAnswerFeedback, saveAnswerFeedback } from '@/api/answer-feedback'

const props = defineProps<{ sessionId: string; messageId: string }>()
const choices = [
  { value: 'useful' as const, label: '有用', icon: ThumbUpIcon },
  { value: 'not_useful' as const, label: '无用', icon: ThumbDownIcon },
]
const reasonOptions = ['与问题无关', '内容不完整', '内容有错误', '数据分析错误']
const rating = ref('')
const busy = ref(false)
const loading = ref(false)
const detailOpen = ref(false)
const reasons = ref<string[]>([])
const detail = ref('')
watch(() => [props.sessionId, props.messageId], async ([sessionId, messageId], _, onCleanup) => {
  let active = true
  onCleanup(() => { active = false })
  rating.value = ''
  if (!sessionId || !messageId) return
  loading.value = true
  try {
    const response = await getOwnAnswerFeedback(sessionId, messageId)
    if (active) rating.value = response.data?.rating || ''
  } catch {
    // A turn may not have finished persisting yet; submission checks it again.
  } finally {
    if (active) loading.value = false
  }
}, { immediate: true })

async function vote(value: 'useful' | 'not_useful') {
  if (busy.value) return
  busy.value = true
  const next = rating.value === value ? 'cancel' : value
  try {
    const response = await saveAnswerFeedback(props.sessionId, props.messageId, { rating: next })
    rating.value = response.data.rating
    if (next === 'not_useful') { reasons.value = []; detail.value = ''; detailOpen.value = true }
    else MessagePlugin.success(next === 'cancel' ? '已撤回反馈' : '反馈已记录')
  } catch (error: any) {
    MessagePlugin.error(error?.message || '反馈提交失败，请重试')
  } finally { busy.value = false }
}

async function saveReason() {
  if (busy.value) return
  busy.value = true
  try {
    await saveAnswerFeedback(props.sessionId, props.messageId, {
      rating: 'not_useful', reason: [...reasons.value, detail.value.trim()].filter(Boolean).join('；'),
    })
    detailOpen.value = false
    MessagePlugin.success('补充说明已记录')
  } catch (error: any) { MessagePlugin.error(error?.message || '反馈提交失败，请重试') }
  finally { busy.value = false }
}
</script>

<style scoped lang="less">
.answer-feedback { display: inline-flex; align-items: center; gap: 4px; flex: 0 0 auto; }
.feedback-vote {
  display: inline-flex; align-items: center; justify-content: center; gap: 6px;
  box-sizing: border-box; width: 68px; height: 30px; flex: 0 0 68px;
  padding: 0 8px; border: 1px solid transparent; border-radius: 4px;
  font: inherit; font-size: 13px; line-height: 20px; letter-spacing: 0;
  color: var(--td-text-color-secondary); background: transparent; cursor: pointer; white-space: nowrap;
  .t-icon { width: 16px; height: 16px; flex: 0 0 16px; }
  &:hover:not(:disabled) { background: var(--td-bg-color-container-hover); }
  &:focus-visible { outline: 2px solid var(--td-brand-color); outline-offset: 1px; }
  &.selected { color: var(--td-brand-color); background: var(--td-brand-color-light); border-color: var(--td-brand-color-focus); }
  &:disabled { cursor: wait; opacity: .55; }
}
.feedback-reasons { display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 1fr); gap: 12px; margin-bottom: 20px; }
@media (max-width: 360px) { .feedback-reasons { grid-template-columns: 1fr; } }
</style>

<style lang="less">
// TDesign teleports its root without the caller's scoped-style attribute.
.feedback-detail-dialog {
  .t-dialog__position { padding: 16px 0; }
  .t-dialog { max-height: calc(100dvh - 32px); display: flex; flex-direction: column; }
  .t-dialog__body { min-height: 0; overflow-y: auto; }
  .t-dialog__header, .t-dialog__footer { flex-shrink: 0; }
  .t-dialog__footer .t-button { min-width: 88px; white-space: nowrap; }
}
</style>
