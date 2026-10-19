<template>
  <div class="feedback-editor">
    <div class="editor-heading">
      <strong>标准答案</strong>
      <t-radio-group v-model="mode" size="small" variant="default-filled" :disabled="disabled">
        <t-radio-button value="rich">富文本</t-radio-button>
        <t-radio-button value="markdown">Markdown</t-radio-button>
      </t-radio-group>
    </div>
    <template v-if="mode === 'rich'">
      <div class="format-toolbar" role="toolbar" aria-label="答案格式">
        <button v-for="tool in tools" :key="tool.label" type="button" :title="tool.label"
          :aria-label="tool.label" :disabled="disabled" @click="tool.run">{{ tool.text }}</button>
        <button type="button" :disabled="disabled" @click="openLink('link')">链接</button>
        <button type="button" :disabled="disabled" @click="openLink('image')">图片</button>
        <template v-if="editor?.isActive('table')">
          <button type="button" :disabled="disabled" @click="editor?.chain().focus().addRowAfter().run()">加行</button>
          <button type="button" :disabled="disabled" @click="editor?.chain().focus().addColumnAfter().run()">加列</button>
          <button type="button" :disabled="disabled" @click="editor?.chain().focus().deleteTable().run()">删除表格</button>
        </template>
      </div>
      <EditorContent :editor="editor" class="editor-content" />
    </template>
    <textarea v-else class="markdown-source" :value="modelValue" :disabled="disabled" aria-label="标准答案 Markdown"
      @input="emit('update:modelValue', ($event.target as HTMLTextAreaElement).value)" />
    <section v-if="linkOpen" class="link-form" :aria-label="linkType === 'image' ? '插入图片' : '插入链接'">
      <t-input v-model="linkURL" aria-label="链接地址" placeholder="https://…" />
      <p class="editor-hint">使用长期可访问的 HTTP/HTTPS 地址。临时文件地址不会保存到知识库。</p>
      <t-button size="small" :disabled="disabled" @click="insertLink">插入</t-button>
      <t-button size="small" variant="text" @click="linkOpen = false">取消</t-button>
    </section>
  </div>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { useEditor, EditorContent } from '@tiptap/vue-3'
import StarterKit from '@tiptap/starter-kit'
import { Markdown } from '@tiptap/markdown'
import Image from '@tiptap/extension-image'
import { TableKit } from '@tiptap/extension-table'
import DOMPurify from 'dompurify'
import { MessagePlugin } from 'tdesign-vue-next'

const props = defineProps<{ modelValue: string; disabled?: boolean }>()
const emit = defineEmits<{ 'update:modelValue': [value: string] }>()
const mode = ref('rich')
const linkOpen = ref(false)
const linkURL = ref('')
const linkType = ref<'link' | 'image'>('link')
const editor = useEditor({
  extensions: [StarterKit.configure({ link: { openOnClick: false } }), Markdown, Image, TableKit],
  content: props.modelValue,
  contentType: 'markdown',
  editable: !props.disabled,
  editorProps: {
    attributes: { role: 'textbox', 'aria-label': '标准答案富文本', 'aria-multiline': 'true' },
    transformPastedHTML: html => DOMPurify.sanitize(html),
    handlePaste: (_view, event) => {
      if (event.clipboardData?.files.length) {
        MessagePlugin.warning('请使用“图片”按钮插入可长期访问的图片链接')
        return true
      }
      return false
    },
    handleDrop: (_view, event) => Boolean(event.dataTransfer?.files.length),
  },
  onUpdate: ({ editor: current }) => emit('update:modelValue', current.getMarkdown()),
})
watch(() => props.modelValue, value => {
  if (editor.value && value !== editor.value.getMarkdown()) {
    editor.value.commands.setContent(value, { contentType: 'markdown', emitUpdate: false })
  }
})
// Locking controls during a request must not serialize or modify the draft.
watch(() => props.disabled, value => editor.value?.setEditable(!value, false))
const tools = [
  { label: '撤销', text: '↶', run: () => editor.value?.chain().focus().undo().run() },
  { label: '重做', text: '↷', run: () => editor.value?.chain().focus().redo().run() },
  { label: '二级标题', text: 'H2', run: () => editor.value?.chain().focus().toggleHeading({ level: 2 }).run() },
  { label: '加粗', text: 'B', run: () => editor.value?.chain().focus().toggleBold().run() },
  { label: '斜体', text: 'I', run: () => editor.value?.chain().focus().toggleItalic().run() },
  { label: '无序列表', text: '• 列表', run: () => editor.value?.chain().focus().toggleBulletList().run() },
  { label: '有序列表', text: '1. 列表', run: () => editor.value?.chain().focus().toggleOrderedList().run() },
  { label: '代码块', text: '</>', run: () => editor.value?.chain().focus().toggleCodeBlock().run() },
  { label: '引用', text: '❞', run: () => editor.value?.chain().focus().toggleBlockquote().run() },
  { label: '插入表格', text: '表格', run: () => editor.value?.chain().focus().insertTable({ rows: 3, cols: 2, withHeaderRow: true }).run() },
]
function openLink(type: 'link' | 'image') {
  linkType.value = type
  linkURL.value = ''
  linkOpen.value = true
}
function insertLink() {
  const value = linkURL.value.trim()
  try {
    if (!['http:', 'https:'].includes(new URL(value).protocol)) throw new Error()
  } catch { MessagePlugin.warning('请输入有效的 HTTP/HTTPS 地址'); return }
  if (linkType.value === 'image') editor.value?.chain().focus().setImage({ src: value, alt: '知识说明图片' }).run()
  else if (editor.value?.state.selection.empty) editor.value.chain().focus().insertContent({ type: 'text', text: value, marks: [{ type: 'link', attrs: { href: value } }] }).run()
  else editor.value?.chain().focus().setLink({ href: value }).run()
  linkOpen.value = false
}
</script>

<style scoped lang="less">
.feedback-editor { border: 1px solid var(--td-component-border); border-radius: 10px; overflow: hidden; background: var(--td-bg-color-container); }
.editor-heading { padding: 12px; display: flex; justify-content: space-between; align-items: center; gap: 12px; }
.format-toolbar { display: flex; flex-wrap: wrap; padding: 8px; gap: 4px; background: var(--td-bg-color-secondarycontainer); border-block: 1px solid var(--td-component-border); }
.format-toolbar button { min-width: 30px; min-height: 30px; border: 0; border-radius: 4px; background: transparent; color: var(--td-text-color-primary); cursor: pointer; }
.format-toolbar button:hover { background: var(--td-bg-color-container-hover); }
.format-toolbar button:disabled { opacity: .4; cursor: default; }
.editor-content :deep(.tiptap) { padding: 16px; min-height: 250px; max-height: 520px; overflow: auto; outline: none; line-height: 1.7; overflow-wrap: anywhere; }
.editor-content :deep(p) { margin: 8px 0; }
.editor-content :deep(pre) { background: var(--td-bg-color-secondarycontainer); padding: 12px; white-space: pre-wrap; font-family: monospace; }
.editor-content :deep(table) { border-collapse: collapse; width: 100%; table-layout: fixed; }
.editor-content :deep(th), .editor-content :deep(td) { border: 1px solid var(--td-component-border); padding: 8px; position: relative; }
.editor-content :deep(th) { background: var(--td-bg-color-secondarycontainer); }
.editor-content :deep(img) { max-width: 100%; height: auto; }
.editor-content :deep(blockquote) { border-left: 3px solid var(--td-brand-color); margin: 12px 0; padding-left: 12px; }
.markdown-source { width: 100%; box-sizing: border-box; min-height: 300px; padding: 16px; resize: vertical; border: none; border-top: 1px solid var(--td-component-border); background: inherit; color: inherit; font: 14px/1.7 monospace; }
.editor-hint { font-size: 12px; color: var(--td-text-color-secondary); }
.link-form { padding: 14px; border-top: 1px solid var(--td-component-border); }
</style>
