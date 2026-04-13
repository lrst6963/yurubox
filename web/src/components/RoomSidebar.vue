<template>
  <div class="room-sidebar">
    <div class="ip-list">
      <div class="ip-list-title">当前在线成员</div>
      <div v-if="currentRoomUsers.length === 0" class="ip-item" style="color: var(--md-sys-color-outline);">暂无其他用户</div>
      <div
        v-for="user in currentRoomUsers"
        :key="user.id"
        class="ip-user-wrapper"
      >
        <div
          class="ip-item"
          :class="{
            'ip-item-self': user.id === clientId,
            'ip-item-editable': user.id === clientId
          }"
          :tabindex="user.id === clientId ? 0 : -1"
          @click="user.id === clientId && $emit('editDisplayName')"
          @keydown.enter="user.id === clientId && $emit('editDisplayName')"
        >
          <span class="ip-item-address">{{ formatRoomUserLabel(user) }}</span>
          <span class="ip-item-status" :class="getStatusColorClass(user.status)">
            ({{ user.status }})
          </span>
        </div>
      </div>
    </div>

    <div class="controls-container" style="flex-direction: column; align-items: center;">
      <!-- 摄像头设备选择 -->
      <div class="device-select-wrapper" v-if="audioConfig.protocol === 'webrtc' && videoDevices.length > 0">
        <select :value="selectedVideoDeviceId" @change="$emit('update:selectedVideoDeviceId', ($event.target as HTMLSelectElement).value); $emit('changeVideoDevice')" class="device-select" :title="isVideoOn ? '切换摄像头' : '选择摄像头'">
          <option v-for="(device, index) in videoDevices" :key="device.deviceId" :value="device.deviceId">
            {{ device.label || '摄像头 ' + (index + 1) }}
          </option>
        </select>
      </div>

      <div style="display: flex; width: 100%; justify-content: space-between; align-items: center;">
        <!-- 摄像头按钮 -->
        <md-icon-button
          @click="$emit('toggleVideo')"
          class="video-btn"
          :class="{ 'video-active': isVideoOn }"
          :aria-label="isVideoOn ? '关闭摄像头' : '打开摄像头'"
          v-if="audioConfig.protocol === 'webrtc'"
        >
          <span class="material-symbols-outlined">
            {{ isVideoOn ? 'videocam' : 'videocam_off' }}
          </span>
        </md-icon-button>

        <!-- 扬声器静音按钮 -->
        <md-icon-button
          @click="$emit('toggleMute')"
          class="mute-btn"
          :aria-label="isMuted ? '取消静音' : '静音'"
        >
          <span class="material-symbols-outlined">
            {{ isMuted ? 'volume_off' : 'volume_up' }}
          </span>
        </md-icon-button>

        <!-- 麦克风主按钮 -->
        <div class="mic-btn-wrapper">
          <md-filled-icon-button 
            v-if="showCallBtn"
            :disabled="isCallBtnDisabled"
            @click="$emit('toggleCall')"
            class="mic-btn"
            :class="{ 'mic-active': isCalling }"
            :aria-label="callBtnText"
          >
            <span class="material-symbols-outlined">
              {{ isCalling ? 'mic_off' : 'mic' }}
            </span>
          </md-filled-icon-button>

          <md-filled-tonal-icon-button 
            v-if="showRequestTalkBtn"
            :disabled="isRequestTalkBtnDisabled"
            @click="$emit('requestTalk')"
            class="mic-btn"
            :aria-label="requestTalkBtnText"
          >
            <span class="material-symbols-outlined">
              {{ isRequestingTalk ? 'hourglass_empty' : 'waving_hand' }}
            </span>
          </md-filled-tonal-icon-button>
        </div>
        
        <!-- 占位元素保持居中平衡 -->
        <div class="controls-spacer"></div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import type { RoomUser } from '../types'
import type { AudioRuntimeConfig } from '../core/audio'

defineProps<{
  currentRoomUsers: RoomUser[]
  clientId: string
  formatRoomUserLabel: (user: RoomUser) => string
  getStatusColorClass: (status: string) => string
  audioConfig: AudioRuntimeConfig
  videoDevices: MediaDeviceInfo[]
  selectedVideoDeviceId: string
  isVideoOn: boolean
  isMuted: boolean
  showCallBtn: boolean
  isCallBtnDisabled: boolean
  isCalling: boolean
  callBtnText: string
  showRequestTalkBtn: boolean
  isRequestTalkBtnDisabled: boolean
  isRequestingTalk: boolean
  requestTalkBtnText: string
}>()

defineEmits<{
  (e: 'editDisplayName'): void
  (e: 'update:selectedVideoDeviceId', value: string): void
  (e: 'changeVideoDevice'): void
  (e: 'toggleVideo'): void
  (e: 'toggleMute'): void
  (e: 'toggleCall'): void
  (e: 'requestTalk'): void
}>()
</script>
