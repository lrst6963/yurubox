<template>
  <div class="chat-container">
    <div class="chat-messages" ref="chatMessagesContainerRef">
      <div
        v-for="msg in chatMessages"
        :key="msg.id"
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
          <span class="chat-sender">{{ getSenderDisplayName(msg) }}</span>
          <span class="chat-time">{{ new Date(msg.timestamp).toLocaleTimeString() }}</span>
        </div>
        <div
          v-if="msg.revoked"
          class="chat-message-revoked"
        >
          {{ msg.senderId === clientId ? '你撤回了一条消息' : '对方撤回了一条消息' }}
        </div>
        <div
          v-else
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
    </div>
    <div v-if="pendingImages.length > 0" class="chat-pending-images">
      <div v-for="item in pendingImages" :key="item.id" class="chat-pending-image-item">
        <img :src="item.url" class="chat-pending-image" />
        <button class="chat-pending-remove" @click="removePendingImage(item.id)" type="button">×</button>
      </div>
    </div>
    <div class="chat-input-area">
      <md-icon-button @click="fileInputRef?.click()" aria-label="发送附件">
        <span class="material-symbols-outlined">attach_file</span>
      </md-icon-button>
      <md-icon-button @click="imageInputRef?.click()" aria-label="发送图片">
        <span class="material-symbols-outlined">image</span>
      </md-icon-button>
      <input type="file" ref="fileInputRef" style="display: none" @change="uploadFile($event, 'file')" />
      <input type="file" ref="imageInputRef" style="display: none" accept="image/*" multiple @change="selectImages($event)" />
      
      <md-outlined-text-field
        class="chat-input-field"
        placeholder="输入消息(上限1000字)..."
        :value="chatInput"
        @input="$emit('update:chatInput', ($event.target as HTMLInputElement).value)"
        @keyup.enter="sendTextMessage"
        @paste="handlePaste"
        maxlength="1000"
      ></md-outlined-text-field>
      <md-icon-button @click="sendTextMessage" aria-label="发送" :disabled="!chatInput.trim() && pendingImages.length === 0">
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
