<template>
  <div class="header-flex">
    <h1 class="room-title">Yurubox</h1>
    <div style="display: flex; align-items: center; gap: 12px; margin-left: auto;">
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
    <div slot="headline">设置</div>
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

      <!-- QQ头像同步 -->
      <div class="setting-item" style="margin-top: 8px;">
        <md-outlined-text-field
          label="同步QQ头像"
          type="number"
          supporting-text="输入QQ号以显示头像，不填则不使用"
          :value="localQQNumber"
          @input="localQQNumber = ($event.target as HTMLInputElement).value"
          @change="$emit('update:qqNumber', localQQNumber)"
        ></md-outlined-text-field>
      </div>

      <!-- 主题模式设置 -->
      <div class="setting-item" style="margin-top: 8px;">
        <md-filled-select label="主题模式" :value="theme" @change="$emit('update:theme', ($event.target as HTMLSelectElement).value)">
          <md-select-option value="system">
            <div slot="headline">跟随系统</div>
          </md-select-option>
          <md-select-option value="light">
            <div slot="headline">浅色模式</div>
          </md-select-option>
          <md-select-option value="dark">
            <div slot="headline">深色模式</div>
          </md-select-option>
        </md-filled-select>
      </div>

      <!-- 主题颜色设置 -->
      <div class="setting-item" style="margin-top: 8px;">
        <md-filled-select label="主题颜色" :value="colorTheme" @change="$emit('update:colorTheme', ($event.target as HTMLSelectElement).value)">
          <md-select-option value="default">
            <div slot="headline">默认 (翡翠绿)</div>
          </md-select-option>
          <md-select-option value="wechat">
            <div slot="headline">微信绿</div>
          </md-select-option>
          <md-select-option value="qq">
            <div slot="headline">QQ蓝</div>
          </md-select-option>
          <md-select-option value="netease">
            <div slot="headline">网易红</div>
          </md-select-option>
          <md-select-option value="weibo">
            <div slot="headline">微博橙</div>
          </md-select-option>
          <md-select-option value="bilibili">
            <div slot="headline">B站粉</div>
          </md-select-option>
          <md-select-option value="purple">
            <div slot="headline">紫罗兰</div>
          </md-select-option>
          <md-select-option value="cyan">
            <div slot="headline">青黛蓝</div>
          </md-select-option>
        </md-filled-select>
      </div>

      <div style="display: flex; align-items: center; justify-content: space-between; margin-bottom: 8px; margin-top: 8px; gap: 24px;">
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
@media (max-width: 600px) {
  .room-title {
    display: none;
  }
}

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
import { ref, watch } from 'vue'

const props = defineProps<{
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
  qqNumber: string
  theme: string
  colorTheme: string
  isLocalMediaMuted?: boolean
}>()

const localQQNumber = ref(props.qqNumber)
watch(() => props.qqNumber, (newVal) => {
  localQQNumber.value = newVal
})
defineEmits<{
  (e: 'toggleLogs'): void
  (e: 'leaveRoom'): void
  (e: 'updateNoiseSuppression', value: boolean): void
  (e: 'update:qqNumber', value: string): void
  (e: 'update:theme', value: string): void
  (e: 'update:colorTheme', value: string): void
  (e: 'update:selectedVideoDeviceId', value: string): void
  (e: 'update:selectedAudioDeviceId', value: string): void
  (e: 'update:selectedAudioOutputDeviceId', value: string): void
  (e: 'changeVideoDevice'): void
  (e: 'changeAudioDevice'): void
  (e: 'changeAudioOutputDevice'): void
}>()

const showSettings = ref(false)
</script>
