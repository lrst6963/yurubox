<template>
  <div class="header-flex">
    <h1>Web 通话</h1>
    <div style="display: flex; align-items: center; gap: 12px;">
      <span class="badge">{{ userCount }} 人在线</span>
      <md-icon-button @click="showSettings = true" aria-label="设置" :class="{ 'active': showSettings }">
        <span class="material-symbols-outlined">settings</span>
      </md-icon-button>
      <md-icon-button @click="$emit('toggleLogs')" aria-label="查看日志" :class="{ 'active': showLogs }">
        <span class="material-symbols-outlined">receipt_long</span>
      </md-icon-button>
      <md-icon-button @click="$emit('leaveRoom')" aria-label="退出频道">
        <span class="material-symbols-outlined">logout</span>
      </md-icon-button>
    </div>
  </div>

  <!-- 设置弹窗 -->
  <md-dialog :open="showSettings" @close="showSettings = false">
    <div slot="headline">频道设置</div>
    <div slot="content" class="settings-content">
      <!-- 麦克风选择 -->
      <div class="setting-item" v-if="audioDevices.length > 0">
        <md-filled-select label="麦克风" :value="selectedAudioDeviceId" @change="$emit('update:selectedAudioDeviceId', ($event.target as HTMLSelectElement).value); $emit('changeAudioDevice')">
          <md-select-option v-for="(device, index) in audioDevices" :key="device.deviceId" :value="device.deviceId">
            <div slot="headline">{{ device.label || '麦克风 ' + (index + 1) }}</div>
          </md-select-option>
        </md-filled-select>
        <div class="volume-bar-container">
          <div class="volume-bar" :style="{ width: Math.min(localVolume * 100, 100) + '%' }"></div>
        </div>
      </div>

      <!-- 扬声器选择 -->
      <div class="setting-item" v-if="audioOutputDevices.length > 0">
        <md-filled-select label="扬声器" :value="selectedAudioOutputDeviceId" @change="$emit('update:selectedAudioOutputDeviceId', ($event.target as HTMLSelectElement).value); $emit('changeAudioOutputDevice')">
          <md-select-option v-for="(device, index) in audioOutputDevices" :key="device.deviceId" :value="device.deviceId">
            <div slot="headline">{{ device.label || '扬声器 ' + (index + 1) }}</div>
          </md-select-option>
        </md-filled-select>
        <div class="volume-bar-container">
          <div class="volume-bar output-bar" :style="{ width: Math.min(remoteVolume * 100, 100) + '%' }"></div>
        </div>
      </div>

      <!-- 摄像头选择 -->
      <div class="setting-item" v-if="videoDevices.length > 0">
        <md-filled-select label="摄像头" :value="selectedVideoDeviceId" @change="$emit('update:selectedVideoDeviceId', ($event.target as HTMLSelectElement).value); $emit('changeVideoDevice')">
          <md-select-option v-for="(device, index) in videoDevices" :key="device.deviceId" :value="device.deviceId">
            <div slot="headline">{{ device.label || '摄像头 ' + (index + 1) }}</div>
          </md-select-option>
        </md-filled-select>
      </div>

      <div style="display: flex; align-items: center; justify-content: space-between; margin-bottom: 8px; margin-top: 16px; gap: 24px;">
        <span>麦克风降噪</span>
        <md-switch
          :selected="noiseSuppression"
          @change="$emit('updateNoiseSuppression', ($event.target as any).selected)"
        ></md-switch>
      </div>
      <div style="font-size: 12px; color: var(--md-sys-color-on-surface-variant);">
        开启降噪可以过滤背景噪音，但可能会降低音质。如果你在安静的环境下，建议关闭降噪以获得更好的音质。
      </div>
    </div>
    <div slot="actions">
      <md-text-button @click="showSettings = false">关闭</md-text-button>
    </div>
  </md-dialog>
</template>

<style scoped>
.settings-content {
  display: flex;
  flex-direction: column;
  gap: 16px;
  min-width: 300px;
}
.setting-item {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.setting-item md-filled-select {
  width: 100%;
}
.volume-bar-container {
  height: 4px;
  background-color: var(--md-sys-color-surface-variant);
  border-radius: 2px;
  overflow: hidden;
  width: 100%;
}
.volume-bar {
  height: 100%;
  background-color: var(--md-sys-color-primary);
  transition: width 0.1s ease-out;
}
.volume-bar.output-bar {
  background-color: var(--md-sys-color-secondary);
}
</style>

<script setup lang="ts">
import { ref } from 'vue'

defineProps<{
  userCount: number
  showLogs: boolean
  noiseSuppression: boolean
  videoDevices: MediaDeviceInfo[]
  audioDevices: MediaDeviceInfo[]
  audioOutputDevices: MediaDeviceInfo[]
  selectedVideoDeviceId: string
  selectedAudioDeviceId: string
  selectedAudioOutputDeviceId: string
  localVolume: number
  remoteVolume: number
}>()
defineEmits<{
  (e: 'toggleLogs'): void
  (e: 'leaveRoom'): void
  (e: 'updateNoiseSuppression', value: boolean): void
  (e: 'update:selectedVideoDeviceId', value: string): void
  (e: 'update:selectedAudioDeviceId', value: string): void
  (e: 'update:selectedAudioOutputDeviceId', value: string): void
  (e: 'changeVideoDevice'): void
  (e: 'changeAudioDevice'): void
  (e: 'changeAudioOutputDevice'): void
}>()

const showSettings = ref(false)
</script>
