export type RoomUser = {
  id: string
  ip: string
  name?: string
  status: string
  video?: boolean
}

export type ChatMessage = {
  id: string
  roomId: string
  senderId: string
  senderIp?: string
  senderName?: string
  type: string
  content: string
  fileName?: string
  fileSize?: number
  images?: ChatImage[]
  timestamp: number
  revoked?: boolean
  revokedAt?: number
}

export type ChatImage = {
  url: string
  fileName?: string
  fileSize?: number
}

export type PendingImage = {
  id: string
  file: File
  url: string
}

export type MessageMenuState = {
  visible: boolean
  x: number
  y: number
  message: ChatMessage | null
}
