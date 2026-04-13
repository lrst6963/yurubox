<template>
  <div class="video-grid" v-if="hasAnyVideo">
    <div
      v-for="user in usersWithVideo"
      :key="'video_grid_' + user.id"
      class="video-grid-item"
    >
      <div :id="'video_container_' + user.id" class="user-video-container"></div>
      <div class="video-user-label">{{ formatRoomUserLabel(user) }}</div>
      <button class="video-fullscreen-btn" @click="toggleFullscreen('video_container_' + user.id)" title="全屏">
        <span class="material-symbols-outlined">fullscreen</span>
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import type { RoomUser } from '../types'

defineProps<{
  hasAnyVideo: boolean
  usersWithVideo: RoomUser[]
  formatRoomUserLabel: (user: RoomUser) => string
}>()

const toggleFullscreen = (containerId: string) => {
  const container = document.getElementById(containerId)
  if (!container) return

  if (!document.fullscreenElement) {
    container.requestFullscreen().catch(err => {
      console.warn(`Error attempting to enable fullscreen: ${err.message}`)
    })
  } else {
    document.exitFullscreen()
  }
}
</script>
