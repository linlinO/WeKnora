<template>
  <main style="max-width:900px;margin:24px auto;padding:16px">
    <h1>富文本编辑器验收（独立测试数据）</h1>
    <FeedbackAnswerEditor v-model="answer" />
    <h2>保存内容预览</h2><pre data-testid="serialized-answer" style="white-space:pre-wrap">{{ answer }}</pre>
  </main>
</template>
<script setup lang="ts">
import { ref } from 'vue'
import FeedbackAnswerEditor from '@/components/FeedbackAnswerEditor.vue'
const answer = ref('## 检查步骤\n\n1. 检查服务状态。\n2. 查看错误日志。\n\n| 检查项 | 说明 |\n| --- | --- |\n| 状态 | 是否运行 |\n\n```bash\nsystemctl status example\n```')
</script>
