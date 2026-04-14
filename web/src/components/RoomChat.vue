<template>
  <div class="chat-container">
    <div class="chat-messages" ref="chatMessagesContainerRef">
      <template
        v-for="msg in chatMessages"
        :key="msg.id"
      >
        <div v-if="msg.type === 'system'" class="chat-message-system">
          <span>{{ msg.content }}</span>
        </div>
        <div v-else-if="msg.revoked" class="chat-message-system">
          <span>{{ msg.senderId === clientId ? '你' : getSenderDisplayName(msg) }} 撤回了一条消息</span>
        </div>
        <div
          v-else
          class="chat-message"
          :class="{
            'chat-message-self': msg.senderId === clientId,
            'chat-message-image-only': isImageLikeMessage(msg) && !msg.revoked
          }"
          @contextmenu.prevent="openMessageMenu($event, msg)"
          @touchstart="startMessageLongPress($event, msg)"
          @touchend="cancelMessageLongPress"
          @touchmove="cancelMessageLongPress"
          @touchcancel="cancelMessageLongPress"
        >
          <div class="chat-message-header">
            <img v-if="msg.senderAvatar" :src="msg.senderAvatar" class="user-avatar-small" alt="" />
            <span class="chat-sender">{{ getSenderDisplayName(msg) }}</span>
            <span class="chat-time">{{ new Date(msg.timestamp).toLocaleTimeString() }}</span>
          </div>
          <div
            class="chat-message-content"
            :class="{ 'chat-message-content-image': isImageLikeMessage(msg) }"
          >
            <template v-if="msg.type === 'text'">
              {{ msg.content }}
            </template>
            <template v-else-if="msg.type === 'image'">
              <a :href="msg.content" target="_blank">
                <img :src="msg.content" class="chat-image" />
              </a>
            </template>
            <template v-else-if="msg.type === 'file'">
              <a :href="msg.content" target="_blank" class="chat-file">
                <span class="material-symbols-outlined">description</span>
                {{ msg.fileName }} ({{ formatFileSize(msg.fileSize) }})
              </a>
            </template>
            <template v-else-if="msg.type === 'image_group'">
              <div class="chat-image-grid" :class="getImageGridClass(msg.images?.length || 0)">
                <a
                  v-for="(image, index) in msg.images"
                  :key="`${msg.id}_${index}`"
                  :href="image.url"
                  target="_blank"
                  class="chat-image-grid-item"
                >
                  <img :src="image.url" class="chat-image-grid-image" />
                </a>
              </div>
            </template>
          </div>
        </div>
      </template>
    </div>
    <div v-if="pendingImages.length > 0" class="chat-pending-images">
      <div v-for="item in pendingImages" :key="item.id" class="chat-pending-image-item">
        <img :src="item.url" class="chat-pending-image" />
        <button class="chat-pending-remove" @click="removePendingImage(item.id)" type="button">×</button>
      </div>
    </div>
    <div class="chat-input-area" :class="{ 'chat-input-disabled': isLocalTextMuted }">
      <md-icon-button @click="fileInputRef?.click()" aria-label="发送附件" :disabled="isLocalTextMuted">
        <span class="material-symbols-outlined">attach_file</span>
      </md-icon-button>
      <md-icon-button @click="imageInputRef?.click()" aria-label="发送图片" :disabled="isLocalTextMuted">
        <span class="material-symbols-outlined">image</span>
      </md-icon-button>
      <input type="file" ref="fileInputRef" style="display: none" @change="uploadFile($event, 'file')" :disabled="isLocalTextMuted" />
      <input type="file" ref="imageInputRef" style="display: none" accept="image/*" multiple @change="selectImages($event)" :disabled="isLocalTextMuted" />
      
      <md-outlined-text-field
        class="chat-input-field"
        :placeholder="isLocalTextMuted ? `禁言中(${localTextMutedCountdown}s)` : '输入消息(上限1000字)...'"
        :value="chatInput"
        @input="$emit('update:chatInput', ($event.target as HTMLInputElement).value)"
        @keyup.enter="sendTextMessage"
        @paste="handlePaste"
        maxlength="1000"
        :disabled="isLocalTextMuted"
      ></md-outlined-text-field>
      <md-icon-button @click="sendTextMessage" aria-label="发送" :disabled="isLocalTextMuted || (!chatInput.trim() && pendingImages.length === 0)">
        <span class="material-symbols-outlined">send</span>
      </md-icon-button>
    </div>

    <div v-if="messageMenu.visible" class="chat-menu-mask" @click="closeMessageMenu">
      <div class="chat-message-menu" :style="{ left: `${messageMenu.x}px`, top: `${messageMenu.y}px` }" @click.stop>
        <button
          v-if="messageMenu.message && canRevokeMessage(messageMenu.message)"
          class="chat-menu-item"
          type="button"
          @click="revokeMessage(messageMenu.message)"
        >
          撤回
        </button>
        <button
          v-if="messageMenu.message && canCopyMessage(messageMenu.message)"
          class="chat-menu-item"
          type="button"
          @click="copyMessage(messageMenu.message)"
        >
          复制
        </button>
        <button class="chat-menu-item" type="button" @click="pasteFromClipboard">
          粘贴
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch, nextTick } from 'vue'
import type { ChatMessage, PendingImage, MessageMenuState } from '../types'

const props = defineProps<{
  clientId: string
  chatMessages: ChatMessage[]
  chatInput: string
  isLocalTextMuted: boolean
  localTextMutedCountdown?: number
  pendingImages: PendingImage[]
  messageMenu: MessageMenuState

  isImageLikeMessage: (msg: ChatMessage) => boolean
  getSenderDisplayName: (msg: ChatMessage) => string
  formatFileSize: (size?: number) => string
  getImageGridClass: (count: number) => string
  removePendingImage: (id: string) => void
  uploadFile: (event: Event, type: 'image' | 'file') => void
  selectImages: (event: Event) => void
  sendTextMessage: () => void
  handlePaste: (e: ClipboardEvent) => void
  openMessageMenu: (event: MouseEvent, message: ChatMessage) => void
  startMessageLongPress: (event: TouchEvent, message: ChatMessage) => void
  cancelMessageLongPress: () => void
  closeMessageMenu: () => void
  canRevokeMessage: (message: ChatMessage) => boolean
  revokeMessage: (message: ChatMessage) => void
  canCopyMessage: (message: ChatMessage) => boolean
  copyMessage: (message: ChatMessage) => void
  pasteFromClipboard: () => void
}>()

defineEmits<{
  (e: 'update:chatInput', value: string): void
}>()

const chatMessagesContainerRef = ref<HTMLElement | null>(null)
const fileInputRef = ref<HTMLInputElement | null>(null)
const imageInputRef = ref<HTMLInputElement | null>(null)

const scrollToBottom = () => {
  nextTick(() => {
    if (chatMessagesContainerRef.value) {
      chatMessagesContainerRef.value.scrollTop = chatMessagesContainerRef.value.scrollHeight
    }
  })
}

watch(() => props.chatMessages, scrollToBottom, { deep: true })

defineExpose({
  scrollToBottom
})

</script>

<style scoped>
.chat-message-system {
  display: flex;
  justify-content: center;
  margin: 12px 0;
}
.chat-message-system span {
  background-color: var(--md-sys-color-surface-variant);
  color: var(--md-sys-color-on-surface-variant);
  padding: 4px 12px;
  border-radius: 12px;
  font-size: 12px;
  text-align: center;
}
.chat-input-disabled {
  opacity: 0.6;
  pointer-events: none;
}

.chat-menu-mask {
  position: fixed;
  inset: 0;
  z-index: 300;
}

.chat-message-menu {
  position: fixed;
  width: 180px;
  padding: 6px 0;
  border-radius: 14px;
  background: var(--md-sys-color-surface);
  box-shadow: var(--md-elevation-2);
  border: 1px solid var(--md-sys-color-outline-variant);
  max-height: 90vh;
  overflow-y: auto;
}
</style>
