<template>
  <EditorFixture v-if="editor" />
  <AnswerFeedback v-else-if="settings" />
  <div v-else class="chat-scroll">
    <p>服务器负载高需要怎么定位？</p>
    <p>先区分 CPU 计算占用与 IO 等待，再定位具体进程。</p>
    <div class="answer-toolbar">
      <t-button variant="text"><t-icon name="copy" /></t-button>
      <AnswerFeedbackButtons session-id="session" message-id="answer" />
    </div>
  </div>
</template>
<script setup lang="ts">
import AnswerFeedbackButtons from '@/views/chat/components/AnswerFeedbackButtons.vue'
import AnswerFeedback from '@/views/settings/AnswerFeedback.vue'
import EditorFixture from './editor.vue'
const editor = new URLSearchParams(location.search).has('editor')
const settings = new URLSearchParams(location.search).has('settings')
</script>
<style scoped lang="less">
@import '../../src/components/css/chat-message-shared.less';
.chat-scroll { margin: 16px; padding: 16px; height: 180px; overflow: auto; transform: translateZ(0); border: 1px solid #ddd; }
</style>
