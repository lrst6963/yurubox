<template>
  <div v-show="showLogs" class="room-logs-popup">
    <div class="logs-header">
      <span>运行日志</span>
      <md-icon-button @click="$emit('close')" aria-label="关闭日志" class="close-logs-btn">
        <span class="material-symbols-outlined">close</span>
      </md-icon-button>
    </div>
    <div class="logs-container" ref="logsContainerRef">
      <div v-for="(log, index) in logs" :key="index" class="log-entry">
        {{ log }}
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch, nextTick } from 'vue'

const props = defineProps<{
  showLogs: boolean
  logs: string[]
}>()

defineEmits<{
  (e: 'close'): void
}>()

const logsContainerRef = ref<HTMLElement | null>(null)

watch(() => props.logs.length, () => {
  if (props.showLogs) {
    nextTick(() => {
      if (logsContainerRef.value) {
        logsContainerRef.value.scrollTop = logsContainerRef.value.scrollHeight
      }
    })
  }
})

watch(() => props.showLogs, (newVal) => {
  if (newVal) {
    nextTick(() => {
      if (logsContainerRef.value) {
        logsContainerRef.value.scrollTop = logsContainerRef.value.scrollHeight
      }
    })
  }
})
</script>
