import { ref } from 'vue'
import type { ChatMessage, PendingImage, MessageMenuState } from '../types'

export function useChat(
  getCurrentRoomId: () => string,
  getClientId: () => string,
  getControlWs: () => WebSocket | null,
  logMsg: (msg: string) => void
) {
  const chatMessages = ref<ChatMessage[]>([])
  const chatInput = ref('')
  const pendingImages = ref<PendingImage[]>([])
  const messageMenu = ref<MessageMenuState>({
    visible: false,
    x: 0,
    y: 0,
    message: null
  })
  let messageLongPressTimer: number | null = null

  const formatFileSize = (bytes?: number) => {
    if (bytes === undefined) return '0 B'
    if (bytes === 0) return '0 B'
    const k = 1024
    const sizes = ['B', 'KB', 'MB', 'GB']
    const i = Math.floor(Math.log(bytes) / Math.log(k))
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
  }

  const getSenderDisplayName = (msg: ChatMessage) => {
    if (msg.senderId === getClientId()) return '我'
    if (msg.senderName) return msg.senderName
    return msg.senderIp ? msg.senderIp.slice(-4) : msg.senderId.slice(0, 4)
  }

  const scrollToBottom = () => {
    // Moved to RoomChat.vue
  }

  const fetchChatHistory = async (roomId: string) => {
    try {
      const res = await fetch(`/api/chat/history?room=${roomId}`)
      if (res.ok) {
        const data = await res.json()
        chatMessages.value = data || []
        scrollToBottom()
      }
    } catch (e) {
      console.error('Failed to fetch chat history', e)
    }
  }

  const isImageLikeMessage = (message: ChatMessage) => message.type === 'image' || message.type === 'image_group'

  const getImageGridClass = (count: number) => {
    if (count <= 1) return 'chat-image-grid-1'
    if (count === 2 || count === 4) return 'chat-image-grid-2'
    return 'chat-image-grid-3'
  }

  const replaceChatMessage = (message: ChatMessage) => {
    const index = chatMessages.value.findIndex(item => item.id === message.id)
    if (index === -1) {
      chatMessages.value.push(message)
      scrollToBottom()
      return
    }
    chatMessages.value[index] = {
      ...chatMessages.value[index],
      ...message
    }
  }

  const clearPendingImages = () => {
    pendingImages.value.forEach(item => URL.revokeObjectURL(item.url))
    pendingImages.value = []
  }

  const removePendingImage = (id: string) => {
    const index = pendingImages.value.findIndex(item => item.id === id)
    if (index === -1) return
    URL.revokeObjectURL(pendingImages.value[index].url)
    pendingImages.value.splice(index, 1)
  }

  const addPendingImages = (files: File[]) => {
    if (pendingImages.value.length >= 9) {
      alert('最多一次发送 9 张图片')
      return
    }
    files.forEach(file => {
      if (!file.type.startsWith('image/')) return
      if (file.size > 20 * 1024 * 1024) {
        alert('图片太大！单张图片限制20MB')
        return
      }
      if (pendingImages.value.length >= 9) return
      pendingImages.value.push({
        id: `${Date.now()}_${Math.random().toString(16).slice(2)}`,
        file,
        url: URL.createObjectURL(file)
      })
    })
    if (pendingImages.value.length >= 9 && files.length > 9) {
      alert('最多一次发送 9 张图片')
    }
  }

  const appendInputText = (text: string) => {
    if (!text) return
    chatInput.value = chatInput.value ? `${chatInput.value}${text}` : text
  }

  const isSocketOpen = (ws: WebSocket | null) => ws && ws.readyState === WebSocket.OPEN

  const uploadFileObj = async (file: File, type: 'image' | 'file') => {
    const maxSize = type === 'image' ? 20 * 1024 * 1024 : 100 * 1024 * 1024
    if (file.size > maxSize) {
      alert(`文件太大！${type === 'image' ? '图片限制20MB' : '附件限制100MB'}`)
      return false
    }

    const formData = new FormData()
    formData.append('file', file)
    formData.append('room', getCurrentRoomId())
    formData.append('client', getClientId())
    formData.append('type', type)

    try {
      logMsg(`正在上传${type === 'image' ? '图片' : '附件'}...`)
      const res = await fetch('/api/chat/upload', {
        method: 'POST',
        body: formData
      })
      
      if (!res.ok) {
        const err = await res.text()
        alert('上传失败: ' + err)
        return false
      } else {
        logMsg('上传成功')
        return true
      }
    } catch (e) {
      console.error('Upload error', e)
      alert('上传发生错误')
      return false
    }
  }

  const uploadImageGroup = async (files: File[]) => {
    if (files.length === 0) return false
    if (files.length === 1) {
      return uploadFileObj(files[0], 'image')
    }

    const formData = new FormData()
    formData.append('room', getCurrentRoomId())
    formData.append('client', getClientId())
    files.forEach(file => {
      formData.append('files', file)
    })

    try {
      logMsg(`正在上传 ${files.length} 张图片...`)
      const res = await fetch('/api/chat/upload-images', {
        method: 'POST',
        body: formData
      })

      if (!res.ok) {
        const err = await res.text()
        alert('上传失败: ' + err)
        return false
      }

      logMsg('多图上传成功')
      return true
    } catch (e) {
      console.error('Upload group error', e)
      alert('多图上传发生错误')
      return false
    }
  }

  const sendTextMessage = async () => {
    const content = chatInput.value.trim()
    const hasPendingImages = pendingImages.value.length > 0
    if (!content && !hasPendingImages) return
    
    if (content.length > 1000) {
      alert('消息内容超过1000字限制！')
      return
    }

    const controlWs = getControlWs()

    if (content) {
      if (!isSocketOpen(controlWs)) return
      controlWs!.send(JSON.stringify({
        type: 'chat',
        content: content
      }))
      chatInput.value = ''
    }

    if (!hasPendingImages) return

    const images = [...pendingImages.value]
    if (images.length > 1) {
      const success = await uploadImageGroup(images.map(item => item.file))
      if (success) {
        clearPendingImages()
      }
      return
    }

    for (const item of images) {
      const success = await uploadFileObj(item.file, 'image')
      if (!success) return
      removePendingImage(item.id)
    }
  }

  const uploadFile = async (event: Event, type: 'image' | 'file') => {
    const target = event.target as HTMLInputElement
    const files = target.files ? Array.from(target.files) : []
    if (type === 'image') {
      addPendingImages(files)
      target.value = ''
      return
    }
    const file = files[0]
    if (file) {
      await uploadFileObj(file, type)
    }
    target.value = ''
  }

  const selectImages = (event: Event) => {
    uploadFile(event, 'image')
  }

  const handlePaste = (e: ClipboardEvent) => {
    const items = e.clipboardData?.items
    if (!items) return
    const imageFiles: File[] = []
    for (let i = 0; i < items.length; i++) {
      const item = items[i]
      if (item.type.startsWith('image/')) {
        const file = item.getAsFile()
        if (file) {
          imageFiles.push(file)
        }
      }
    }
    if (imageFiles.length > 0) {
      e.preventDefault()
      addPendingImages(imageFiles)
    }
  }

  const canRevokeMessage = (message: ChatMessage) => {
    if (message.senderId !== getClientId() || message.revoked) return false
    return Date.now() - message.timestamp <= 2 * 60 * 1000
  }

  const canCopyMessage = (message: ChatMessage) => !message.revoked

  const closeMessageMenu = () => {
    messageMenu.value = {
      visible: false,
      x: 0,
      y: 0,
      message: null
    }
  }

  const openMessageMenuAt = (x: number, y: number, message: ChatMessage) => {
    const itemCount = [
      canRevokeMessage(message),
      canCopyMessage(message),
      true
    ].filter(Boolean).length
    const menuWidth = 160
    const menuHeight = itemCount * 44 + 12
    const padding = 12
    messageMenu.value = {
      visible: true,
      x: Math.min(x, window.innerWidth - menuWidth - padding),
      y: Math.min(y, window.innerHeight - menuHeight - padding),
      message
    }
  }

  const openMessageMenu = (event: MouseEvent, message: ChatMessage) => {
    openMessageMenuAt(event.clientX, event.clientY, message)
  }

  const cancelMessageLongPress = () => {
    if (messageLongPressTimer !== null) {
      window.clearTimeout(messageLongPressTimer)
      messageLongPressTimer = null
    }
  }

  const startMessageLongPress = (event: TouchEvent, message: ChatMessage) => {
    cancelMessageLongPress()
    const touch = event.touches[0]
    if (!touch) return
    messageLongPressTimer = window.setTimeout(() => {
      openMessageMenuAt(touch.clientX, touch.clientY, message)
      messageLongPressTimer = null
    }, 450)
  }

  const copyMessage = async (message: ChatMessage) => {
    closeMessageMenu()
    try {
      if (message.type === 'image' && 'clipboard' in navigator && 'write' in navigator.clipboard && typeof ClipboardItem !== 'undefined') {
        const response = await fetch(message.content)
        if (response.ok) {
          const blob = await response.blob()
          await navigator.clipboard.write([new ClipboardItem({ [blob.type]: blob })])
          logMsg('已复制图片')
          return
        }
      }

      if (message.type === 'image_group') {
        const textToCopy = (message.images || [])
          .map(image => new URL(image.url, window.location.origin).toString())
          .join('\n')
        await navigator.clipboard.writeText(textToCopy)
        logMsg('已复制图片链接')
        return
      }

      const textToCopy = message.type === 'text'
        ? message.content
        : new URL(message.content, window.location.origin).toString()
      await navigator.clipboard.writeText(textToCopy)
      logMsg(message.type === 'file' ? '已复制文件链接' : '已复制消息')
    } catch (e) {
      alert('复制失败，请检查浏览器权限设置')
    }
  }

  const pasteFromClipboard = async () => {
    closeMessageMenu()
    try {
      const imageFiles: File[] = []
      let text = ''

      if ('clipboard' in navigator && 'read' in navigator.clipboard) {
        const items = await navigator.clipboard.read()
        for (const item of items) {
          const imageType = item.types.find(type => type.startsWith('image/'))
          if (imageType) {
            const blob = await item.getType(imageType)
            imageFiles.push(new File([blob], `pasted-${Date.now()}.${imageType.split('/')[1] || 'png'}`, { type: imageType }))
            continue
          }
          if (item.types.includes('text/plain')) {
            const textBlob = await item.getType('text/plain')
            text += await textBlob.text()
          }
        }
      } else if ('clipboard' in navigator && 'readText' in navigator.clipboard) {
        text = await navigator.clipboard.readText()
      }

      if (imageFiles.length > 0) {
        addPendingImages(imageFiles)
      }
      if (text) {
        appendInputText(text)
      }
    } catch (e) {
      alert('粘贴失败，请检查浏览器权限设置')
    }
  }

  const revokeMessage = async (message: ChatMessage) => {
    closeMessageMenu()
    if (!canRevokeMessage(message)) {
      alert('该消息已超过撤回时限')
      return
    }
    try {
      const response = await fetch('/api/chat/revoke', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json'
        },
        body: JSON.stringify({
          roomId: getCurrentRoomId(),
          clientId: getClientId(),
          messageId: message.id
        })
      })
      if (!response.ok) {
        alert(await response.text())
        return
      }
      const updatedMessage = await response.json()
      replaceChatMessage(updatedMessage)
    } catch (e) {
      alert('撤回失败，请稍后再试')
    }
  }

  return {
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
    addPendingImages,
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
  }
}
