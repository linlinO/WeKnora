import { createApp, h } from 'vue'
import { createPinia } from 'pinia'
import { createRouter, createWebHashHistory, RouterView } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import i18n from '@/i18n'
import TDesign from 'tdesign-vue-next'
import 'tdesign-vue-next/dist/tdesign.css'
import '@/assets/theme/theme.css'
import '@/assets/theme/tdesign-overrides.less'
import { installTDesignIconOfflineGuard } from '@/utils/tdesign-icon-offline'
import Fixture from './fixture.vue'
import AnswerFeedback from '@/views/settings/AnswerFeedback.vue'
import AnswerFeedbackDetail from '@/views/settings/AnswerFeedbackDetail.vue'
import QAInsights from '@/views/settings/QAInsights.vue'
import QAInsightDetail from '@/views/settings/QAInsightDetail.vue'
import EmployeeMemory from '@/views/settings/EmployeeMemory.vue'

installTDesignIconOfflineGuard()
const pinia = createPinia()
const router = createRouter({ history: createWebHashHistory(), routes: [
  { path: '/', component: Fixture },
  { path: '/platform/answer-feedback', name: 'answerFeedback', component: AnswerFeedback },
  { path: '/platform/answer-feedback/:id', name: 'answerFeedbackDetail', component: AnswerFeedbackDetail },
  { path: '/platform/qa-insights', component: QAInsights },
  { path: '/platform/qa-insights/memory', component: EmployeeMemory },
  { path: '/platform/qa-insights/memory/:id', component: EmployeeMemory },
  { path: '/platform/qa-insights/topics/:topicId', component: QAInsights },
  { path: '/platform/qa-insights/questions/:id', component: QAInsightDetail },
] })
const app = createApp({ render: () => h(RouterView) }).use(pinia).use(router).use(TDesign).use(i18n)
const auth = useAuthStore(pinia)
auth.$patch({ user: { id: 'test-admin', username: '测试管理员' } as any, tenant: { id: 42, name: '验收空间' } as any,
  memberships: [{ tenant_id: 42, role: 'admin' }] })
router.isReady().then(async () => {
  if (new URLSearchParams(location.search).has('workbench') && !router.currentRoute.value.name) await router.replace({ name: 'answerFeedback' })
  app.mount('#app')
})
