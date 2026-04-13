<template>
  <div class="app-wrapper" :class="{ 'in-room': isInRoom }">
    <LoginView
      v-if="!isInRoom"
      :roomKey="roomKey"
      @update:roomKey="roomKey = $event"
      :isConnecting="isConnecting"
      @join="joinRoom"
    />

    <div v-else class="card-container room-container">
      <RoomHeader
        :userCount="userCount"
        :showLogs="showLogs"
        :noiseSuppression="audioConfig.noiseSuppression ?? false"
        :videoDevices="videoDevices"
        :audioDevices="audioDevices"
        :audioOutputDevices="audioOutputDevices"
        :selectedVideoDeviceId="selectedVideoDeviceId"
        :selectedAudioDeviceId="selectedAudioDeviceId"
        :selectedAudioOutputDeviceId="selectedAudioOutputDeviceId"
        :localVolume="localVolume"
        :remoteVolume="remoteVolume"
        :qqNumber="qqNumber"
        :theme="theme"
        :colorTheme="colorTheme"
        @update:colorTheme="colorTheme = $event"
        @update:theme="theme = $event"
        @update:qqNumber="updateQQNumber"
        @update:selectedVideoDeviceId="selectedVideoDeviceId = $event"
        @update:selectedAudioDeviceId="selectedAudioDeviceId = $event"
        @update:selectedAudioOutputDeviceId="selectedAudioOutputDeviceId = $event"
        @changeVideoDevice="changeVideoDevice"
        @changeAudioDevice="changeAudioDevice"
        @changeAudioOutputDevice="changeAudioOutputDevice"
        @updateNoiseSuppression="toggleNoiseSuppression"
        @toggleLogs="toggleLogs"
        @leaveRoom="leaveRoom"
      />
      
      <div class="room-content">
        <RoomSidebar
          :currentRoomUsers="currentRoomUsers"
          :clientId="clientId"
          :formatRoomUserLabel="formatRoomUserLabel"
          :getStatusColorClass="getStatusColorClass"
          :audioConfig="audioConfig"
          :videoDevices="videoDevices"
          :selectedVideoDeviceId="selectedVideoDeviceId"
          :isVideoOn="isVideoOn"
          :isMuted="isMuted"
          :showCallBtn="showCallBtn"
          :isCallBtnDisabled="isCallBtnDisabled"
          :isCalling="isCalling"
          :callBtnText="callBtnText"
          :showRequestTalkBtn="showRequestTalkBtn"
          :isRequestTalkBtnDisabled="isRequestTalkBtnDisabled"
          :isRequestingTalk="isRequestingTalk"
          :requestTalkBtnText="requestTalkBtnText"
          :isLocalMediaMuted="isLocalMediaMuted"
          @editDisplayName="editDisplayName"
          @update:selectedVideoDeviceId="selectedVideoDeviceId = $event"
          :userVolumes="userVolumes"
          @changeVideoDevice="changeVideoDevice"
          @toggleVideo="toggleVideo"
          @toggleMute="toggleMute"
          @toggleCall="toggleCall"
          @requestTalk="requestTalk"
          @openUserMenu="openUserMenu"
          @startUserLongPress="startUserLongPress"
          @cancelUserLongPress="cancelUserLongPress"
        />

        <!-- 用户管理菜单 -->
        <div v-if="userMenu.visible" class="chat-menu-mask" @click="closeUserMenu">
          <div class="chat-message-menu" :style="{ left: `${userMenu.x}px`, top: `${userMenu.y}px` }" @click.stop>
            <button class="chat-menu-item" type="button" @click="kickUser(userMenu.user)">踢出</button>
            <button class="chat-menu-item" type="button" @click="muteUser(userMenu.user, 10000)">禁言10秒</button>
            <button class="chat-menu-item" type="button" @click="muteUser(userMenu.user, 30000)">禁言30秒</button>
            <button class="chat-menu-item" type="button" @click="muteUser(userMenu.user, 60000)">禁言1分钟</button>
            <button class="chat-menu-item" type="button" @click="customMuteUser(userMenu.user)">自定义禁言</button>
            <button class="chat-menu-item" type="button" @click="customMuteMediaUser(userMenu.user)">自定义禁音(语音/视频)</button>
            <button class="chat-menu-item" type="button" @click="unmuteAllUser(userMenu.user)">解除禁言禁音</button>
            <button class="chat-menu-item" type="button" @click="changeUserName(userMenu.user)">修改名称</button>
          </div>
        </div>

        <div class="room-chat">
          <RoomVideoGrid
            :hasAnyVideo="hasAnyVideo"
            :usersWithVideo="usersWithVideo"
            :formatRoomUserLabel="formatRoomUserLabel"
            :userVolumes="userVolumes"
            :clientId="clientId"
          />
          
          <RoomChat
            :clientId="clientId"
            :chatMessages="chatMessages"
            :chatInput="chatInput"
            :isLocalTextMuted="isLocalTextMuted"
            @update:chatInput="chatInput = $event"
            :pendingImages="pendingImages"
            :messageMenu="messageMenu"
            :isImageLikeMessage="isImageLikeMessage"
            :getSenderDisplayName="getSenderDisplayName"
            :formatFileSize="formatFileSize"
            :getImageGridClass="getImageGridClass"
            :removePendingImage="removePendingImage"
            :uploadFile="uploadFile"
            :selectImages="selectImages"
            :sendTextMessage="sendTextMessage"
            :handlePaste="handlePaste"
            :openMessageMenu="openMessageMenu"
            :startMessageLongPress="startMessageLongPress"
            :cancelMessageLongPress="cancelMessageLongPress"
            :closeMessageMenu="closeMessageMenu"
            :canRevokeMessage="canRevokeMessage"
            :revokeMessage="revokeMessage"
            :canCopyMessage="canCopyMessage"
            :copyMessage="copyMessage"
            :pasteFromClipboard="pasteFromClipboard"
            ref="roomChatRef"
          />
        </div>

        <RoomLogs
          :showLogs="showLogs"
          :logs="logs"
          @close="toggleLogs"
        />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, computed, watch } from 'vue'
import { getKey, hashPassword } from './utils/crypto'
import { createClientId, isSocketOpen, buildWebSocketUrl } from './utils/helpers'
import { AudioEngine } from './core/audio'
import { handleWebRTCSignal } from './core/connection'
import type { RoomUser } from './types'

import LoginView from './components/LoginView.vue'
import RoomHeader from './components/RoomHeader.vue'
import RoomSidebar from './components/RoomSidebar.vue'
import RoomVideoGrid from './components/RoomVideoGrid.vue'
import RoomChat from './components/RoomChat.vue'
import RoomLogs from './components/RoomLogs.vue'

import { useChat } from './composables/useChat'
import { useMediaControl } from './composables/useMediaControl'
import { useLogs } from './composables/useLogs'

// --- 状态定义 ---
const roomKey = ref('')
const isInRoom = ref(false)
const isConnecting = ref(false)

// --- 用户管理菜单 ---
const userMenu = ref({
  visible: false,
  x: 0,
  y: 0,
  user: null as RoomUser | null
})
let userLongPressTimer: number | null = null

const openUserMenu = (event: MouseEvent | TouchEvent, user: RoomUser) => {
  const currentUser = currentRoomUsers.value.find(u => u.id === clientId)
  if (!currentUser || !currentUser.isAdmin) return
  if (user.id === clientId) return

  event.preventDefault()
  
  let x = 0
  let y = 0
  if (event instanceof MouseEvent) {
    x = event.clientX
    y = event.clientY
  } else if (window.TouchEvent && event instanceof TouchEvent) {
    x = event.touches[0].clientX
    y = event.touches[0].clientY
  }
  
  // 确保菜单不会超出屏幕边界
  const menuWidth = 180 // 菜单宽度，与 CSS 中对应
  const menuHeight = 360 // 预估菜单高度 (增加高度以适应更多选项)
  const padding = 10 // 屏幕边缘留白
  
  if (x + menuWidth + padding > window.innerWidth) {
    x = window.innerWidth - menuWidth - padding
  }
  if (y + menuHeight + padding > window.innerHeight) {
    y = window.innerHeight - menuHeight - padding
    // 如果修正后 y 仍然小于 padding，说明屏幕太矮，强制固定在顶部
    if (y < padding) {
      y = padding
    }
  }
  
  userMenu.value = {
    visible: true,
    x,
    y,
    user
  }
}

const closeUserMenu = () => {
  userMenu.value.visible = false
}

const startUserLongPress = (event: TouchEvent, user: RoomUser) => {
  const currentUser = currentRoomUsers.value.find(u => u.id === clientId)
  if (!currentUser || !currentUser.isAdmin) return
  if (user.id === clientId) return

  if (userLongPressTimer) clearTimeout(userLongPressTimer)
  userLongPressTimer = window.setTimeout(() => {
    openUserMenu(event, user)
  }, 500)
}

const cancelUserLongPress = () => {
  if (userLongPressTimer) {
    clearTimeout(userLongPressTimer)
    userLongPressTimer = null
  }
}

const kickUser = (user: RoomUser | null) => {
  if (!user || !controlWs || !isSocketOpen(controlWs)) return
  controlWs.send(JSON.stringify({ type: 'admin_action', action: 'kick', targetID: user.id }))
  closeUserMenu()
}

const muteUser = (user: RoomUser | null, duration: number) => {
  if (!user || !controlWs || !isSocketOpen(controlWs)) return
  controlWs.send(JSON.stringify({ type: 'admin_action', action: 'mute', targetID: user.id, duration }))
  closeUserMenu()
}

const customMuteUser = (user: RoomUser | null) => {
  if (!user) return
  const input = prompt('请输入禁言时长(秒):', '60')
  if (input !== null) {
    const duration = parseInt(input, 10)
    if (!isNaN(duration) && duration > 0) {
      muteUser(user, duration * 1000)
    }
  }
  closeUserMenu()
}

const customMuteMediaUser = (user: RoomUser | null) => {
  if (!user) return
  const input = prompt('请输入禁音时长(秒):', '60')
  if (input !== null) {
    const duration = parseInt(input, 10)
    if (!isNaN(duration) && duration > 0) {
      if (controlWs && isSocketOpen(controlWs)) {
        controlWs.send(JSON.stringify({ type: 'admin_action', action: 'mute_media', targetID: user.id, duration: duration * 1000 }))
      }
    }
  }
  closeUserMenu()
}

const unmuteAllUser = (user: RoomUser | null) => {
  if (!user || !controlWs || !isSocketOpen(controlWs)) return
  controlWs.send(JSON.stringify({ type: 'admin_action', action: 'unmute_all', targetID: user.id }))
  closeUserMenu()
}

const changeUserName = (user: RoomUser | null) => {
  if (!user || !controlWs || !isSocketOpen(controlWs)) return
  const newName = prompt(`请输入 ${user.name || user.ip} 的新名称:`, user.name || '')
  if (newName !== null && newName.trim() !== '') {
    controlWs.send(JSON.stringify({ type: 'admin_action', action: 'change_name', targetID: user.id, newName: newName.trim() }))
  }
  closeUserMenu()
}

const { logs, showLogs, toggleLogs, logMsg, clearLogs } = useLogs()

// 房间状态
const currentRoomUsers = ref<RoomUser[]>([])
const userCount = computed(() => currentRoomUsers.value.length)

// 主题设置
const theme = ref(localStorage.getItem('phonecall_theme') || 'system')
watch(theme, (newTheme) => {
  localStorage.setItem('phonecall_theme', newTheme)
  if (newTheme === 'dark') {
    document.documentElement.classList.add('dark')
    document.documentElement.classList.remove('light')
  } else if (newTheme === 'light') {
    document.documentElement.classList.add('light')
    document.documentElement.classList.remove('dark')
  } else {
    document.documentElement.classList.remove('dark', 'light')
  }
}, { immediate: true })

// 颜色主题设置
const colorTheme = ref(localStorage.getItem('phonecall_colorTheme') || 'default')
watch(colorTheme, (newColor, oldColor) => {
  localStorage.setItem('phonecall_colorTheme', newColor)
  if (oldColor && oldColor !== 'default') {
    document.documentElement.classList.remove(`color-${oldColor}`)
  }
  if (newColor !== 'default') {
    document.documentElement.classList.add(`color-${newColor}`)
  }
}, { immediate: true })

// --- 核心变量 ---
let cryptoKey: CryptoKey | null = null
let controlWs: WebSocket | null = null
let mediaWs: WebSocket | null = null
let clientId = ''
let currentRoomId = ''
let isCleaningUp = false
const displayName = ref(normalizeDisplayName(localStorage.getItem('phonecall_displayName') || ''))
const qqNumber = ref(localStorage.getItem('phonecall_qqNumber') || '')

const getAvatarUrl = (qq: string) => qq.trim() ? `http://q2.qlogo.cn/headimg_dl?dst_uin=${qq.trim()}&spec=5` : ''

const isLocalMediaMuted = computed(() => {
  const user = currentRoomUsers.value.find(u => u.id === clientId)
  return !!user?.mediaMuted
})

const isLocalTextMuted = computed(() => {
  const user = currentRoomUsers.value.find(u => u.id === clientId)
  return !!user?.textMuted
})

const {
  usersWithVideo,
  hasAnyVideo,
  isCalling,
  isMuted,
  isVideoOn,
  videoDevices,
  audioDevices,
  audioOutputDevices,
  selectedVideoDeviceId,
  selectedAudioDeviceId,
  selectedAudioOutputDeviceId,
  mediaChannelReady,
  isRequestingTalk,
  showCallBtn,
  showRequestTalkBtn,
  isCallBtnDisabled,
  callBtnText,
  isRequestTalkBtnDisabled,
  requestTalkBtnText,
  userVolumes,
  localVolume,
  remoteVolume,
  
  audioEngine,
  setAudioConfig,
  getAudioConfig,
  bindAudioUnlockEvents,
  loadMediaDevices,
  getPeerConnection,
  toggleCall,
  toggleVideo,
  changeVideoDevice,
  changeAudioDevice,
  changeAudioOutputDevice,
  toggleMute,
  requestTalk,
  syncUsersWithVideoFromRoomInfo,
  handleMediaMessage,
  cleanupMedia,
  updatePeerConnectionsOnRoomInfo,
  reportStatus
} = useMediaControl(
  () => clientId,
  () => currentRoomUsers.value,
  () => controlWs,
  () => mediaWs,
  () => cryptoKey,
  logMsg
)

const audioConfig = computed(() => getAudioConfig())

const {
  chatMessages,
  chatInput,
  pendingImages,
  messageMenu,
  formatFileSize,
  getSenderDisplayName,
  fetchChatHistory,
  isImageLikeMessage,
  getImageGridClass,
  replaceChatMessage,
  clearPendingImages,
  removePendingImage,
  sendTextMessage,
  uploadFile,
  selectImages,
  handlePaste,
  canRevokeMessage,
  canCopyMessage,
  closeMessageMenu,
  openMessageMenu,
  cancelMessageLongPress,
  startMessageLongPress,
  copyMessage,
  pasteFromClipboard,
  revokeMessage
} = useChat(
  () => currentRoomId,
  () => clientId,
  () => controlWs,
  () => {
    const user = currentRoomUsers.value.find(u => u.id === clientId)
    return user ? (user.isAdmin === true) : false
  },
  logMsg
)

const roomChatRef = ref<InstanceType<typeof RoomChat> | null>(null)

function normalizeDisplayName(name: string) {
  const normalized = Array.from(name.trim()).slice(0, 20).join('')
  return normalized || '未命名'
}

const toggleNoiseSuppression = async (value: boolean) => {
  setAudioConfig({ noiseSuppression: value })
  localStorage.setItem('phonecall_noiseSuppression', value.toString())
  
  if (isCalling.value) {
    // 重新开启麦克风以应用新的设置
    await toggleCall(false)
    await toggleCall(true)
  }
}

const formatRoomUserLabel = (user: RoomUser) => `${normalizeDisplayName(user.name || '')}(${user.ip})`

const editDisplayName = () => {
  const nextName = window.prompt('请输入昵称（最多20字）', displayName.value === '未命名' ? '' : displayName.value)
  if (nextName === null) return

  const normalizedName = normalizeDisplayName(nextName)
  if (normalizedName === displayName.value) return

  displayName.value = normalizedName
  localStorage.setItem('phonecall_displayName', normalizedName)

  if (isSocketOpen(controlWs)) {
    controlWs!.send(JSON.stringify({
      type: 'update_name',
      name: normalizedName
    }))
  }
}

const updateQQNumber = (qq: string) => {
  const normalizedQQ = qq.trim()
  qqNumber.value = normalizedQQ
  localStorage.setItem('phonecall_qqNumber', normalizedQQ)

  if (isSocketOpen(controlWs)) {
    controlWs!.send(JSON.stringify({
      type: 'update_avatar',
      avatar: getAvatarUrl(normalizedQQ)
    }))
  }
}

const getStatusColorClass = (status: string) => {
  if (status === '就绪') return 'ip-status-ok'
  if (status === '麦克风无权限') return 'ip-status-warn'
  if (status === '对讲中') return 'ip-status-talk'
  return ''
}

const joinRoom = async () => {
  const password = roomKey.value.trim()
  if (!password) {
    alert('请输入频道密码')
    return
  }

  if (password.length < 8 || password.length > 20) {
    alert('密码长度必须在8到20个字符之间')
    return
  }

  isConnecting.value = true
  logMsg('正在生成端到端加密密钥...')

  try {
    cryptoKey = await getKey(roomKey.value)
    await audioEngine.ensureAudioContextReady(audioConfig.value, () => {}, () => {})
    logMsg('生成端到端加密密钥成功')
    sessionStorage.setItem('roomPassword', roomKey.value)
  } catch (e: any) {
    alert('生成加密密钥失败: ' + e.message)
    isConnecting.value = false
    return
  }

  const roomId = await hashPassword(roomKey.value)
  currentRoomId = roomId
  let storedClientId = localStorage.getItem('phonecall_clientId')
  if (!storedClientId) {
    storedClientId = createClientId()
    localStorage.setItem('phonecall_clientId', storedClientId)
  }
  clientId = storedClientId
  currentRoomUsers.value = []
  isCleaningUp = false
  mediaChannelReady.value = false
  
  isInRoom.value = true
  isConnecting.value = false
  clearLogs()
  chatMessages.value = []
  logMsg('正在连接服务器进入频道...')
  connectControlChannel(roomId)
  fetchChatHistory(roomId)
}

const connectControlChannel = (roomId: string) => {
  controlWs = new WebSocket(buildWebSocketUrl('/ws/control', roomId, clientId, { 
    name: displayName.value,
    avatar: getAvatarUrl(qqNumber.value)
  }))

  controlWs.onopen = () => {
    logMsg('控制通道已连接')
    audioEngine.ensureAudioContextReady(audioConfig.value, () => {}, () => {}).catch(e => console.warn(e))
    bindAudioUnlockEvents()
    AudioEngine.checkMicPermission().then(granted => {
      if (granted) {
        reportStatus('就绪')
      } else {
        logMsg('麦克风无权限')
        reportStatus('麦克风无权限')
      }
    })
    connectMediaChannel(roomId)
  }

  controlWs.onmessage = (event) => {
    if (typeof event.data !== 'string') return
    try {
      const data = JSON.parse(event.data)
      if (data.type === 'room_info') {
        updateRoomInfo(data)
      } else if (data.type === 'error') {
        alert('系统提示: ' + data.message)
      } else if (data.type === 'kicked') {
        alert(data.message)
        leaveRoom()
      } else if (data.type === 'muted') {
        // 无需 alert
      } else if (data.type === 'media_muted') {
        // 无需 alert
        if (isCalling.value) toggleCall()
        if (isVideoOn.value) toggleVideo()
      } else if (data.type === 'unmuted') {
        // 无需 alert
      } else if (data.type === 'request_talk') {
        if (isCalling.value) {
          if (confirm(`[${data.fromIP}] 正在申请讲话，是否同意让出麦克风？`)) {
            toggleCall(false) // 关闭麦克风
            if (isSocketOpen(controlWs)) {
              controlWs!.send(JSON.stringify({ type: 'approve_talk', toIP: data.fromIP }))
            }
          }
        }
      } else if (data.type === 'approve_talk') {
        isRequestingTalk.value = false
        logMsg('对方已让出麦克风，你可以开始讲话了！')
        toggleCall(true) // 开启麦克风
      } else if (['webrtc_offer', 'webrtc_answer', 'webrtc_candidate'].includes(data.type)) {
        handleWebRTCSignal(data, clientId, audioConfig.value.quality, controlWs, getPeerConnection)
      } else if (data.type === 'chat_message') {
        replaceChatMessage(data.data)
      } else if (data.type === 'chat_message_revoked') {
        replaceChatMessage(data.data)
      }
    } catch (e) {}
  }

  controlWs.onclose = () => {
    if (isCleaningUp) return
    logMsg('控制通道已断开')
    leaveRoom()
  }
}

const connectMediaChannel = (roomId: string) => {
  if (audioConfig.value.protocol === 'webrtc') {
    mediaChannelReady.value = true
    logMsg('使用 WebRTC 协议，媒体通道已就绪')
    return
  }

  mediaChannelReady.value = false
  mediaWs = new WebSocket(buildWebSocketUrl('/ws/media', roomId, clientId))
  mediaWs.binaryType = 'arraybuffer'

  mediaWs.onopen = () => {
    mediaChannelReady.value = true
    logMsg('媒体通道已连接，等待其他人加入...')
  }

  mediaWs.onmessage = async (event) => {
    if (typeof event.data === 'string') return
    if (!(event.data instanceof ArrayBuffer)) return
    await handleMediaMessage(event.data)
  }

  mediaWs.onclose = () => {
    if (isCleaningUp) return
    mediaChannelReady.value = false
    logMsg('媒体通道已断开')
    leaveRoom()
  }
}

const updateRoomInfo = (data: any) => {
  currentRoomUsers.value = Array.isArray(data.users) ? data.users : []
  syncUsersWithVideoFromRoomInfo()

  if (audioConfig.value.protocol === 'webrtc') {
    currentRoomUsers.value.forEach(u => {
      if (u.id !== clientId) getPeerConnection(u.id)
    })

    const activeIds = currentRoomUsers.value.map(u => u.id)
    updatePeerConnectionsOnRoomInfo(activeIds)
  }

  if (data.count >= 2 && mediaChannelReady.value && !isCalling.value) {
    const isSomeoneTalking = currentRoomUsers.value.some(u => u.status === '对讲中')
    if (!isSomeoneTalking) {
      logMsg('频道内有 2 人，现在可以打开麦克风了！')
    }
  }
}

const leaveRoom = () => {
  if (isCleaningUp) return
  isCleaningUp = true
  
  cleanupMedia()
  sessionStorage.removeItem('roomPassword')

  if (controlWs) {
    controlWs.onclose = null
    controlWs.close()
    controlWs = null
  }
  if (mediaWs) {
    mediaWs.onclose = null
    mediaWs.close()
    mediaWs = null
  }
  
  isInRoom.value = false
  currentRoomUsers.value = []
  clearPendingImages()
  closeMessageMenu()
  logMsg('已退出频道')
  isCleaningUp = false
}

onMounted(async () => {
  bindAudioUnlockEvents()
  if (navigator.mediaDevices) {
    navigator.mediaDevices.addEventListener('devicechange', loadMediaDevices)
    loadMediaDevices()
  }
  try {
    const response = await fetch('/api/audio-config')
    if (response.ok) {
      const config = await response.json()
      setAudioConfig(config)
      console.log('Loaded audio config:', getAudioConfig())
    }
  } catch (e) {
    console.warn('Failed to load audio config, using defaults.')
  }

  const savedPassword = sessionStorage.getItem('roomPassword')
  if (savedPassword) {
    roomKey.value = savedPassword
    logMsg('检测到上次登录的频道密码，自动连接中...')
    joinRoom()
  }
})

onBeforeUnmount(() => {
  if (navigator.mediaDevices) {
    navigator.mediaDevices.removeEventListener('devicechange', loadMediaDevices)
  }
  cancelMessageLongPress()
  clearPendingImages()
})
</script>

<style>
.font-monospace {
  font-family: monospace;
}

.video-grid {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  padding: 12px;
  background: var(--md-sys-color-surface-container-highest);
  border-top-left-radius: 12px;
  border-top-right-radius: 12px;
  justify-content: center;
  align-items: center;
  max-height: 40vh;
  overflow-y: auto;
}
.video-grid-item {
  width: 100%;
  position: relative;
  background: #000;
  border-radius: 8px;
  overflow: hidden;
  aspect-ratio: 16 / 9;
  border: 2px solid var(--md-sys-color-outline-variant);
  box-sizing: border-box;
}
@media (min-width: 600px) {
  .video-grid-item {
    width: calc(50% - 6px);
  }
}
@media (min-width: 1000px) {
  .video-grid-item {
    width: calc(33.333% - 8px);
  }
}
.video-grid-item .user-video-container {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  width: 100%;
  height: 100%;
  border-radius: 0;
  display: flex;
  justify-content: center;
  align-items: center;
  background: #000;
}
.video-grid-item .user-video {
  max-height: 100%;
  max-width: 100%;
  width: 100%;
  height: 100%;
  object-fit: contain;
  display: block !important;
}
.video-user-label {
  position: absolute;
  bottom: 8px;
  left: 8px;
  background: rgba(0, 0, 0, 0.6);
  color: #fff;
  padding: 2px 8px;
  border-radius: 4px;
  font-size: 12px;
  pointer-events: none;
  z-index: 10;
}
.video-fullscreen-btn {
  position: absolute;
  bottom: 8px;
  right: 8px;
  background: rgba(0, 0, 0, 0.6);
  color: #fff;
  border: none;
  border-radius: 4px;
  padding: 4px;
  cursor: pointer;
  display: flex;
  justify-content: center;
  align-items: center;
  z-index: 10;
}
.video-fullscreen-btn:hover {
  background: rgba(0, 0, 0, 0.8);
}
.video-fullscreen-btn .material-symbols-outlined {
  font-size: 18px;
}
.chat-container {
  background: var(--md-sys-color-surface-container-highest);
  border-radius: 12px;
  display: flex;
  flex-direction: column;
  flex: 1;
  min-height: 0;
  overflow: hidden;
}
.video-grid + .chat-container {
  border-top-left-radius: 0;
  border-top-right-radius: 0;
}

.chat-messages {
  flex: 1;
  padding: 12px;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.chat-message {
  display: flex;
  flex-direction: column;
  max-width: 80%;
  align-self: flex-start;
}

.chat-message-self {
  align-self: flex-end;
}

.chat-message-header {
  font-size: 12px;
  color: var(--md-sys-color-on-surface-variant);
  margin-bottom: 4px;
  display: flex;
  gap: 8px;
}

.chat-message-self .chat-message-header {
  flex-direction: row-reverse;
}

.chat-message-content {
  background: var(--md-sys-color-surface);
  padding: 8px 12px;
  border-radius: 12px;
  border-top-left-radius: 4px;
  color: var(--md-sys-color-on-surface);
  word-break: break-word;
  white-space: pre-wrap;
}

.chat-message-content-image {
  padding: 0;
  background: transparent;
}

.chat-message-self .chat-message-content {
  background: var(--md-sys-color-primary-container);
  color: var(--md-sys-color-on-primary-container);
  border-top-left-radius: 12px;
  border-top-right-radius: 4px;
}

.chat-message-self .chat-message-content-image {
  background: transparent;
}

.chat-message-image-only {
  max-width: min(280px, 70%);
}

.chat-message-revoked {
  color: var(--md-sys-color-on-surface-variant);
  font-size: 13px;
  font-style: italic;
  padding: 6px 0;
}

.chat-image {
  max-width: 100%;
  max-height: 260px;
  border-radius: 8px;
  display: block;
}

.chat-image-grid {
  display: grid;
  gap: 4px;
  width: min(280px, 70vw);
}

.chat-image-grid-1 {
  grid-template-columns: 1fr;
}

.chat-image-grid-2 {
  grid-template-columns: repeat(2, 1fr);
}

.chat-image-grid-3 {
  grid-template-columns: repeat(3, 1fr);
}

.chat-image-grid-item {
  display: block;
  aspect-ratio: 1 / 1;
  overflow: hidden;
  border-radius: 8px;
  background: var(--md-sys-color-surface-container);
}

.chat-image-grid-image {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
}

.chat-file {
  display: flex;
  align-items: center;
  gap: 4px;
  text-decoration: none;
  color: inherit;
  font-weight: 500;
}

.chat-input-area {
  display: flex;
  align-items: center;
  padding: 8px;
  background: var(--md-sys-color-surface-container);
  border-bottom-left-radius: 12px;
  border-bottom-right-radius: 12px;
  gap: 4px;
}

.chat-pending-images {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  padding: 12px 12px 0;
  background: var(--md-sys-color-surface-container);
}

.chat-pending-image-item {
  position: relative;
  width: 72px;
  height: 72px;
  border-radius: 12px;
  overflow: hidden;
  background: var(--md-sys-color-surface);
}

.chat-pending-image {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
}

.chat-pending-remove {
  position: absolute;
  top: 4px;
  right: 4px;
  width: 22px;
  height: 22px;
  border: none;
  border-radius: 50%;
  background: rgba(0, 0, 0, 0.65);
  color: #fff;
  cursor: pointer;
  line-height: 1;
}

.chat-menu-mask {
  position: fixed;
  inset: 0;
  z-index: 300;
}

.chat-message-menu {
  position: fixed;
  width: 160px;
  padding: 6px 0;
  border-radius: 14px;
  background: var(--md-sys-color-surface);
  box-shadow: var(--md-elevation-2);
  border: 1px solid var(--md-sys-color-outline-variant);
}

.chat-menu-item {
  width: 100%;
  border: none;
  background: transparent;
  color: var(--md-sys-color-on-surface);
  text-align: left;
  padding: 12px 16px;
  font-size: 14px;
  cursor: pointer;
}

.chat-menu-item:hover {
  background: var(--md-sys-color-surface-container);
}

.chat-input-field {
  flex: 1;
}

.device-select-wrapper {
  width: 100%;
  margin-bottom: 8px;
}
.device-select {
  width: 100%;
  padding: 6px 12px;
  border-radius: 8px;
  border: 1px solid var(--md-sys-color-outline);
  background: var(--md-sys-color-surface);
  color: var(--md-sys-color-on-surface);
  font-size: 13px;
  outline: none;
}
.ip-list {
  flex: 1;
  background: var(--md-sys-color-surface);
  border-radius: 12px;
  padding: 12px;
  overflow-y: auto;
  border: 1px solid var(--md-sys-color-outline-variant);
  display: flex;
  flex-direction: column;
}
.ip-list-title {
  font-size: 13px;
  font-weight: 500;
  color: var(--md-sys-color-on-surface-variant);
  padding: 0 4px 8px 4px;
  margin-bottom: 4px;
  border-bottom: 1px solid var(--md-sys-color-outline-variant);
}
.ip-user-wrapper {
  display: flex;
  flex-direction: column;
  border-bottom: 1px solid var(--md-sys-color-outline-variant);
  margin-bottom: 8px;
  background-color: var(--md-sys-color-surface-container);
  border-radius: 8px;
  overflow: hidden;
}
.ip-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 16px;
  font-size: 14px;
  color: var(--md-sys-color-on-surface);
  transition: background-color 0.2s;
  border-bottom: none;
}
.ip-item-self {
  font-weight: bold;
  background-color: transparent;
  color: var(--md-sys-color-primary);
}
.ip-item-editable {
  cursor: pointer;
  border-radius: 4px;
}
.ip-item-editable:hover {
  background-color: var(--md-sys-color-surface-container-highest);
}
.user-video-container {
  width: 100%;
  background-color: #000;
  display: flex;
  justify-content: center;
  align-items: center;
  overflow: hidden;
  border-bottom-left-radius: 8px;
  border-bottom-right-radius: 8px;
}
.user-video {
  width: 100%;
  height: 100%;
  max-width: 100%;
  max-height: 100%;
  object-fit: contain;
  display: block;
}
.ip-item:last-child {
  border-bottom: none;
}

/* 移动端聊天区优化 */
@media (max-width: 800px) {
  .chat-container {
    border-radius: 0;
  }

  .chat-messages {
    padding: 8px;
    gap: 8px;
  }

  .chat-message {
    max-width: 85%;
  }

  .chat-input-area {
    border-radius: 0;
    padding: 6px 4px;
  }

  .chat-pending-images {
    padding: 8px 8px 0;
  }

  .video-grid {
    padding: 8px;
    gap: 8px;
    border-radius: 0;
  }
}
</style>
