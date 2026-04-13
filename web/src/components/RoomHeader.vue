<template>
  <div class="header-flex">
    <h1>Web 通话</h1>
    <div style="display: flex; align-items: center; gap: 12px;">
      <span class="badge">{{ userCount }} 人在线</span>
      <md-icon-button @click="$emit('toggleLogs')" aria-label="查看日志" :class="{ 'active': showLogs }">
        <span class="material-symbols-outlined">receipt_long</span>
      </md-icon-button>
      <md-icon-button @click="$emit('leaveRoom')" aria-label="退出频道">
        <span class="material-symbols-outlined">logout</span>
      </md-icon-button>
    </div>
  </div>
</template>

<script setup lang="ts">
defineProps<{
  userCount: number
  showLogs: boolean
}>()
defineEmits<{
  (e: 'toggleLogs'): void
  (e: 'leaveRoom'): void
}>()
</script>
