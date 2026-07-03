<template>
  <div class="flex" :class="isUser ? 'justify-end' : 'justify-start'">
    <div
      class="max-w-[85%] rounded-2xl px-4 py-3 text-sm"
      :class="
        isUser
          ? 'bg-primary-600 text-white'
          : 'bg-gray-100 text-gray-900 dark:bg-dark-700 dark:text-gray-100'
      "
    >
      <!-- 用户上传的图片缩略图 -->
      <div v-if="message.attachments?.length" class="mb-2 flex flex-wrap gap-2">
        <img
          v-for="(src, i) in message.attachments"
          :key="i"
          :src="src"
          class="h-16 w-16 rounded-lg object-cover"
          alt="attachment"
        />
      </div>

      <!-- 推理过程（可折叠） -->
      <div v-if="message.reasoning" class="mb-2">
        <button
          class="text-xs opacity-70 hover:opacity-100"
          type="button"
          @click="reasoningOpen = !reasoningOpen"
        >
          {{ reasoningOpen ? t('playground.hideReasoning') : t('playground.showReasoning') }}
        </button>
        <pre
          v-if="reasoningOpen"
          class="mt-1 whitespace-pre-wrap break-words rounded-lg bg-black/5 p-2 text-xs opacity-80 dark:bg-white/5"
          >{{ message.reasoning }}</pre
        >
      </div>

      <!-- 文本内容 -->
      <pre
        v-if="message.content"
        class="whitespace-pre-wrap break-words font-sans"
        >{{ message.content }}</pre
      >

      <!-- 流式光标 -->
      <span v-if="message.streaming && !message.images?.length" class="animate-pulse">▍</span>

      <!-- 生图结果网格 -->
      <div v-if="message.images?.length" class="mt-1 grid grid-cols-2 gap-2">
        <div
          v-for="(img, i) in message.images"
          :key="i"
          class="group relative overflow-hidden rounded-lg border border-gray-200 dark:border-dark-600"
        >
          <img :src="imageSrc(img)" class="w-full object-cover" alt="generated" />
          <div
            class="absolute inset-x-0 bottom-0 flex justify-end gap-1 bg-black/40 p-1 opacity-0 transition group-hover:opacity-100"
          >
            <button
              class="rounded bg-white/90 px-2 py-0.5 text-xs text-gray-800"
              type="button"
              @click="download(img, i)"
            >
              {{ t('playground.download') }}
            </button>
            <button
              class="rounded bg-white/90 px-2 py-0.5 text-xs text-gray-800"
              type="button"
              @click="$emit('use-as-edit-input', img)"
            >
              {{ t('playground.useAsEditInput') }}
            </button>
          </div>
        </div>
      </div>

      <!-- 错误态 -->
      <p v-if="message.error" class="mt-1 text-xs text-red-500 dark:text-red-400">
        ⚠ {{ message.error }}
      </p>

      <!-- 用量回显 -->
      <p
        v-if="usageText"
        class="mt-2 border-t border-black/10 pt-1 text-[11px] opacity-60 dark:border-white/10"
      >
        {{ usageText }}
      </p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import type { PlaygroundImage } from '@/api/playground'

export interface UiMessage {
  id: number
  role: 'user' | 'assistant'
  kind: 'text' | 'image'
  content: string
  reasoning?: string
  images?: PlaygroundImage[]
  usage?: Record<string, unknown>
  error?: string
  streaming?: boolean
  attachments?: string[]
}

const props = defineProps<{ message: UiMessage }>()
defineEmits<{ (e: 'use-as-edit-input', img: PlaygroundImage): void }>()

const { t } = useI18n()
const reasoningOpen = ref(false)

const isUser = computed(() => props.message.role === 'user')

function imageSrc(img: PlaygroundImage): string {
  if (img.url) return img.url
  if (img.b64) return `data:image/png;base64,${img.b64}`
  return ''
}

function download(img: PlaygroundImage, idx: number) {
  const src = imageSrc(img)
  if (!src) return
  const a = document.createElement('a')
  a.href = src
  a.download = `playground-image-${props.message.id}-${idx}.png`
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
}

const usageText = computed(() => {
  const u = props.message.usage
  if (!u) return ''
  const parts: string[] = []
  const input = (u.input_tokens ?? u.prompt_tokens) as number | undefined
  const output = (u.output_tokens ?? u.completion_tokens) as number | undefined
  if (input != null || output != null) {
    parts.push(
      t('playground.usage.tokens', { input: input ?? 0, output: output ?? 0 })
    )
  }
  if (props.message.images?.length) {
    parts.push(t('playground.generatedImages', { n: props.message.images.length }))
  }
  return parts.join(' · ')
})
</script>
