<template>
  <div class="room-sidebar">
    <!-- 移动端底部工具栏 -->
    <div class="mobile-toolbar">
      <!-- 用户列表抽屉 -->
      <Transition name="drawer">
        <div v-if="mobileDrawerOpen" class="mobile-drawer-backdrop" @click="mobileDrawerOpen = false">
          <div class="mobile-drawer" @click.stop>
            <div class="mobile-drawer-header">
              <span>在线成员</span>
              <md-icon-button @click="mobileDrawerOpen = false" aria-label="关闭">
                <span class="material-symbols-outlined">close</span>
              </md-icon-button>
            </div>
            <div class="mobile-drawer-body">
              <!-- 移动端摄像头选择 -->
              <div class="device-select-wrapper" v-if="audioConfig.protocol === 'webrtc' && videoDevices.length > 0">
                <select :value="selectedVideoDeviceId" @change="$emit('update:selectedVideoDeviceId', ($event.target as HTMLSelectElement).value); $emit('changeVideoDevice')" class="device-select" :title="isVideoOn ? '切换摄像头' : '选择摄像头'">
                  <option v-for="(device, index) in videoDevices" :key="device.deviceId" :value="device.deviceId">
                    {{ device.label || '摄像头 ' + (index + 1) }}
                  </option>
                </select>
              </div>

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
              @contextmenu.prevent="$emit('openUserMenu', $event, user)"
              @touchstart="$emit('startUserLongPress', $event, user)"
              @touchend="$emit('cancelUserLongPress')"
              @touchmove="$emit('cancelUserLongPress')"
              @touchcancel="$emit('cancelUserLongPress')"
            >
                  <span class="ip-item-address">
                    <img v-if="user.avatar" :src="user.avatar" class="user-avatar-small" alt="" />
                    <span class="voice-indicator" v-show="userVolumes[user.id] > 0.05">
                      <span class="voice-bar"></span><span class="voice-bar"></span><span class="voice-bar"></span>
                    </span>
                    {{ formatRoomUserLabel(user) }}
                  </span>
                  <span class="ip-item-status" :class="getStatusColorClass(user.status)">
                    ({{ user.status }})
                  </span>
                </div>
              </div>
            </div>
          </div>
        </div>
      </Transition>

      <!-- 底部操作栏 -->
      <div class="mobile-bar">
        <!-- 用户列表按钮 -->
        <md-icon-button @click="mobileDrawerOpen = !mobileDrawerOpen" aria-label="成员列表">
          <span class="material-symbols-outlined">group</span>
        </md-icon-button>

        <!-- 摄像头按钮 -->
        <md-icon-button
          @click="$emit('toggleVideo')"
          class="video-btn"
          :class="{ 'video-active': isVideoOn }"
          :aria-label="isVideoOn ? '关闭摄像头' : '打开摄像头'"
          v-if="audioConfig.protocol === 'webrtc'"
          :disabled="isLocalMediaMuted"
        >
          <span class="material-symbols-outlined">
            {{ isVideoOn ? 'videocam' : 'videocam_off' }}
          </span>
        </md-icon-button>

        <!-- 扬声器静音 -->
        <md-icon-button
          @click="$emit('toggleMute')"
          class="mute-btn"
          :aria-label="isMuted ? '取消静音' : '静音'"
        >
          <span class="material-symbols-outlined">
            {{ isMuted ? 'volume_off' : 'volume_up' }}
          </span>
        </md-icon-button>

        <!-- 麦克风按钮 -->
        <md-filled-icon-button
          v-if="showCallBtn"
          :disabled="isCallBtnDisabled"
          @click="$emit('toggleCall')"
          class="mobile-mic-btn"
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
          class="mobile-mic-btn"
          :aria-label="requestTalkBtnText"
        >
          <span class="material-symbols-outlined">
            {{ isRequestingTalk ? 'hourglass_empty' : 'waving_hand' }}
          </span>
        </md-filled-tonal-icon-button>
      </div>
    </div>

    <!-- 桌面端原始侧边栏布局 -->
    <div class="desktop-sidebar">
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
            @contextmenu.prevent="$emit('openUserMenu', $event, user)"
            @touchstart="$emit('startUserLongPress', $event, user)"
            @touchend="$emit('cancelUserLongPress')"
            @touchmove="$emit('cancelUserLongPress')"
            @touchcancel="$emit('cancelUserLongPress')"
          >
            <span class="ip-item-address">
              <img v-if="user.avatar" :src="user.avatar" class="user-avatar-small" alt="" />
              <span class="voice-indicator" v-show="userVolumes[user.id] > 0.05">
                <span class="voice-bar"></span><span class="voice-bar"></span><span class="voice-bar"></span>
              </span>
              {{ formatRoomUserLabel(user) }}
            </span>
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
            :disabled="isLocalMediaMuted"
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
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import type { RoomUser } from '../types'
import type { AudioRuntimeConfig } from '../core/audio'

const mobileDrawerOpen = ref(false)

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
  userVolumes: Record<string, number>
  isLocalMediaMuted: boolean
}>()

defineEmits<{
  (e: 'editDisplayName'): void
  (e: 'update:selectedVideoDeviceId', value: string): void
  (e: 'changeVideoDevice'): void
  (e: 'toggleVideo'): void
  (e: 'toggleMute'): void
  (e: 'toggleCall'): void
  (e: 'requestTalk'): void
  (e: 'openUserMenu', event: MouseEvent | TouchEvent, user: RoomUser): void
  (e: 'startUserLongPress', event: TouchEvent, user: RoomUser): void
  (e: 'cancelUserLongPress'): void
}>()
</script>

<style scoped>
/* 桌面端默认显示传统侧边栏 */
.mobile-toolbar {
  display: none;
}

.desktop-sidebar {
  display: contents;
}

/* 移动端布局 */
@media (max-width: 800px) {
  .desktop-sidebar {
    display: none;
  }

  .mobile-toolbar {
    display: block;
  }

  /* 底部操作栏 */
  .mobile-bar {
    display: flex;
    align-items: center;
    justify-content: space-evenly;
    gap: 4px;
    padding: 8px 12px;
    padding-bottom: max(8px, env(safe-area-inset-bottom));
    background: var(--md-sys-color-surface);
    border-top: 1px solid var(--md-sys-color-outline-variant);
  }

  .mobile-mic-btn {
    --md-icon-button-state-layer-width: 48px;
    --md-icon-button-state-layer-height: 48px;
    --md-icon-button-icon-size: 24px;
    width: 48px;
    height: 48px;
    transition: all 0.2s cubic-bezier(0.2, 0, 0, 1);
  }

  .mobile-mic-btn.mic-active {
    --md-sys-color-primary: #b3261e;
    --md-sys-color-on-primary: #ffffff;
    box-shadow: 0 0 16px rgba(179, 38, 30, 0.4);
    border-radius: 50%;
  }

  /* 抽屉面板 */
  .mobile-drawer-backdrop {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.4);
    z-index: 200;
    display: flex;
    align-items: flex-end;
  }

  .mobile-drawer {
    width: 100%;
    max-height: 60vh;
    background: var(--md-sys-color-surface);
    border-top-left-radius: 20px;
    border-top-right-radius: 20px;
    display: flex;
    flex-direction: column;
    overflow: hidden;
    box-shadow: 0 -4px 16px rgba(0, 0, 0, 0.15);
  }

  .mobile-drawer-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 16px 20px 8px;
    font-size: 16px;
    font-weight: 500;
    color: var(--md-sys-color-on-surface);
    flex-shrink: 0;
  }

  .mobile-drawer-body {
    flex: 1;
    overflow-y: auto;
    padding: 4px 16px 16px;
    -webkit-overflow-scrolling: touch;
  }

  /* 抽屉动画 */
  .drawer-enter-active,
  .drawer-leave-active {
    transition: opacity 0.25s ease;
  }
  .drawer-enter-active .mobile-drawer,
  .drawer-leave-active .mobile-drawer {
    transition: transform 0.3s cubic-bezier(0.2, 0, 0, 1);
  }
  .drawer-enter-from,
  .drawer-leave-to {
    opacity: 0;
  }
  .drawer-enter-from .mobile-drawer,
  .drawer-leave-to .mobile-drawer {
    transform: translateY(100%);
  }
}

/* 语音发声动画 */
.voice-indicator {
  display: inline-flex;
  align-items: center;
  gap: 2px;
  height: 14px;
  margin-right: 6px;
  vertical-align: middle;
}
.voice-bar {
  width: 3px;
  background-color: var(--md-sys-color-primary);
  border-radius: 2px;
  animation: voice-bounce 0.5s infinite alternate ease-in-out;
}
.voice-bar:nth-child(1) { height: 50%; animation-delay: 0s; }
.voice-bar:nth-child(2) { height: 100%; animation-delay: 0.15s; }
.voice-bar:nth-child(3) { height: 75%; animation-delay: 0.3s; }

@keyframes voice-bounce {
  from { transform: scaleY(0.3); }
  to { transform: scaleY(1); }
}
</style>
