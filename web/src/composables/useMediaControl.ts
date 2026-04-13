import { ref, computed, nextTick } from 'vue'
import { encryptAudio, decryptAudio } from '../utils/crypto'
import { float32ToInt16, int16ToFloat32, isSocketOpen } from '../utils/helpers'
import { AudioEngine, AudioRuntimeConfig, buildVideoConstraints } from '../core/audio'
import { initPeerConnection } from '../core/connection'
import type { RoomUser } from '../types'

export function useMediaControl(
  getClientId: () => string,
  getCurrentRoomUsers: () => RoomUser[],
  getControlWs: () => WebSocket | null,
  getMediaWs: () => WebSocket | null,
  getCryptoKey: () => CryptoKey | null,
  logMsg: (msg: string) => void
) {
  const audioEngine = new AudioEngine(logMsg)
  
  let audioConfig: AudioRuntimeConfig = { mode: 'normal', quality: 'lossless', sampleRate: 48000, bufferSize: 4096, protocol: 'ws' }
  let peerConnections: Record<string, RTCPeerConnection> = {}
  let audioUnlockBound = false

  const usersWithVideo = ref<RoomUser[]>([])
  const hasAnyVideo = computed(() => usersWithVideo.value.length > 0)

  const isCalling = ref(false)
  const isMuted = ref(false)
  const isVideoOn = ref(false)
  const videoDevices = ref<MediaDeviceInfo[]>([])
  const selectedVideoDeviceId = ref('')
  const mediaChannelReady = ref(false)
  const isRequestingTalk = ref(false)

  const userCount = computed(() => getCurrentRoomUsers().length)

  const showCallBtn = computed(() => {
    const isSomeoneTalking = getCurrentRoomUsers().some(u => u.status === '对讲中')
    if (audioConfig.mode === 'walkie-talkie' && isSomeoneTalking && !isCalling.value) {
      return false
    }
    return true
  })

  const showRequestTalkBtn = computed(() => {
    const isSomeoneTalking = getCurrentRoomUsers().some(u => u.status === '对讲中')
    if (audioConfig.mode === 'walkie-talkie' && isSomeoneTalking && !isCalling.value) {
      return true
    }
    return false
  })

  const isCallBtnDisabled = computed(() => {
    if (userCount.value < 2) return true
    if (!mediaChannelReady.value) return true
    return false
  })

  const callBtnText = computed(() => {
    if (isCalling.value) return '关闭麦克风'
    if (userCount.value < 2) return '打开麦克风 (需至少2人在线)'
    if (!mediaChannelReady.value) return '打开麦克风 (媒体连接中...)'
    return '打开麦克风'
  })

  const isRequestTalkBtnDisabled = computed(() => isRequestingTalk.value)
  const requestTalkBtnText = computed(() => isRequestingTalk.value ? '申请中...' : '申请麦克风')

  const applyVideoElementStyle = (videoEl: HTMLMediaElement) => {
    videoEl.style.width = '100%'
    videoEl.style.height = '100%'
    videoEl.style.maxWidth = '100%'
    videoEl.style.maxHeight = '100%'
    videoEl.style.objectFit = 'contain'
    videoEl.style.display = 'block'
    videoEl.style.backgroundColor = '#000'
  }

  const isMissingVideoDeviceError = (error: unknown) => {
    if (!(error instanceof Error)) return false
    const message = error.message.toLowerCase()
    const name = error.name.toLowerCase()
    return name.includes('notfound') || message.includes('requested device not found')
  }

  const normalizeSelectedVideoDevice = () => {
    if (videoDevices.value.length === 0) {
      selectedVideoDeviceId.value = ''
      return false
    }
    const exists = videoDevices.value.some(device => device.deviceId === selectedVideoDeviceId.value)
    if (!selectedVideoDeviceId.value || !exists) {
      selectedVideoDeviceId.value = videoDevices.value[0].deviceId
      return true
    }
    return false
  }

  const loadVideoDevices = async () => {
    try {
      const devices = await navigator.mediaDevices.enumerateDevices()
      videoDevices.value = devices.filter(d => d.kind === 'videoinput')
      const switched = normalizeSelectedVideoDevice()
      if (switched && isVideoOn.value) {
        logMsg('检测到摄像头列表变化，已自动选择可用设备')
      }
    } catch (e) {
      console.warn('获取摄像头列表失败', e)
    }
  }

  const logVideoTrackSettings = (videoTrack: MediaStreamTrack) => {
    const settings = videoTrack.getSettings()
    const resolutionText = settings.width && settings.height
      ? `${settings.width}x${settings.height}`
      : '未知'
    logMsg(`采集到的实际分辨率: ${resolutionText}`)
    logMsg(`视频轨道设置: ${JSON.stringify(settings)}`)
  }

  const hasRenderableVideoTrack = (stream: MediaStream) => {
    return stream.getVideoTracks().some(track => track.readyState === 'live' && !track.muted)
  }

  const syncSingleTrackKind = (kind: 'audio' | 'video', track: MediaStreamTrack | null, pc: RTCPeerConnection) => {
    const transceiver = pc.getTransceivers().find(t => t.receiver.track.kind === kind)
    
    if (!transceiver) {
      if (track && audioEngine.mediaStream) {
        pc.addTransceiver(track, { direction: 'sendrecv', streams: [audioEngine.mediaStream] })
      } else {
        pc.addTransceiver(kind, { direction: 'recvonly', streams: [] })
      }
      return
    }

    if (track) {
      if (transceiver.sender.track !== track) {
        transceiver.sender.replaceTrack(track).catch(e => console.warn(`Replace ${kind} track failed:`, e))
      }
      if (transceiver.direction !== 'sendrecv' && transceiver.direction !== 'sendonly') {
        transceiver.direction = 'sendrecv'
      }
    } else {
      // 停止发送，但保留接收，设置为空 track 会导致报错，我们应该将其设为 null
      if (transceiver.sender.track) {
        try {
          transceiver.sender.replaceTrack(null).catch(e => console.warn(`Replace null ${kind} track failed:`, e))
        } catch (e) {
          console.warn(`Remove ${kind} track failed:`, e)
        }
      }
      if (transceiver.direction !== 'recvonly' && transceiver.direction !== 'inactive') {
        transceiver.direction = 'recvonly'
      }
    }
  }

  // 同步所有 PeerConnection 的音频和视频轨道
  const syncPeerConnectionTracks = () => {
    if (audioConfig.protocol !== 'webrtc') return
    Object.keys(peerConnections).forEach(pcId => {
      const pc = peerConnections[pcId]
      if (pc.signalingState === 'closed') return

      const nextAudioTrack = (audioConfig.audio === false || !audioEngine.mediaStream) ? null : audioEngine.mediaStream.getAudioTracks()[0] || null
      const nextVideoTrack = (!audioEngine.mediaStream) ? null : audioEngine.mediaStream.getVideoTracks()[0] || null

      syncSingleTrackKind('audio', nextAudioTrack, pc)
      syncSingleTrackKind('video', nextVideoTrack, pc)
    })
  }

  // 只同步音频轨道，不影响视频
  const syncAudioTrackOnly = () => {
    if (audioConfig.protocol !== 'webrtc') return
    Object.keys(peerConnections).forEach(pcId => {
      const pc = peerConnections[pcId]
      if (pc.signalingState === 'closed') return

      const nextAudioTrack = (audioConfig.audio === false || !audioEngine.mediaStream) ? null : audioEngine.mediaStream.getAudioTracks()[0] || null
      syncSingleTrackKind('audio', nextAudioTrack, pc)
    })
  }

  const updateLocalVideoPreview = () => {
    const localStream = audioEngine.mediaStream
    const hasVideo = !!localStream && localStream.getVideoTracks().length > 0
    const clientId = getClientId()

    if (hasVideo || isVideoOn.value) {
      const userIndex = usersWithVideo.value.findIndex(u => u.id === clientId)
      if (userIndex === -1) {
        const user = getCurrentRoomUsers().find(u => u.id === clientId)
        if (user) {
          usersWithVideo.value.push(user)
        }
      }
    } else {
      const userIndex = usersWithVideo.value.findIndex(u => u.id === clientId)
      if (userIndex !== -1) {
        usersWithVideo.value.splice(userIndex, 1)
      }
    }

    nextTick(() => {
      const updatedLocalVideoContainer = document.getElementById(`video_container_${clientId}`)
      if (updatedLocalVideoContainer) {
        let localVideo = document.getElementById(`video_${clientId}`) as HTMLVideoElement
        if (!localVideo) {
          localVideo = document.createElement('video')
          localVideo.id = `video_${clientId}`
          localVideo.className = 'user-video'
          localVideo.autoplay = true
          localVideo.muted = true
          localVideo.playsInline = true
        }
        applyVideoElementStyle(localVideo)
        if (!updatedLocalVideoContainer.contains(localVideo)) {
          updatedLocalVideoContainer.appendChild(localVideo)
        }
        localVideo.srcObject = hasVideo ? localStream : null
        localVideo.style.display = hasVideo ? 'block' : 'none'
      }
    })
  }

  const syncUsersWithVideoFromRoomInfo = () => {
    const nextUsersWithVideo = getCurrentRoomUsers().filter(user => !!user.video)
    usersWithVideo.value = nextUsersWithVideo

    nextTick(() => {
      const clientId = getClientId()
      nextUsersWithVideo.forEach(user => {
        if (user.id === clientId) return
        const remoteVideo = audioEngine.remoteAudioElements[user.id]
        const container = document.getElementById(`video_container_${user.id}`)
        if (remoteVideo && container && !container.contains(remoteVideo)) {
          container.appendChild(remoteVideo)
        }
      })
    })
  }

  const removeLocalVideoTracks = () => {
    if (!audioEngine.mediaStream) return
    audioEngine.mediaStream.getVideoTracks().forEach(track => {
      audioEngine.mediaStream!.removeTrack(track)
      track.stop()
    })
  }

  const reportVideoState = (hasVideo: boolean) => {
    const controlWs = getControlWs()
    if (isSocketOpen(controlWs)) {
      controlWs!.send(JSON.stringify({ type: 'update_video', video: hasVideo }))
    }
  }

  const startVideoTrackWithFallback = async () => {
    const requestVideoTrack = async () => {
      audioConfig.videoDeviceId = selectedVideoDeviceId.value || undefined
      const stream = await navigator.mediaDevices.getUserMedia({
        audio: false,
        video: buildVideoConstraints(audioConfig)
      })
      const videoTrack = stream.getVideoTracks()[0]
      if (!videoTrack) {
        throw new Error('未获取到视频轨道')
      }
      logVideoTrackSettings(videoTrack)
      if (!audioEngine.mediaStream) {
        audioEngine.mediaStream = new MediaStream()
      }
      removeLocalVideoTracks()
      audioEngine.mediaStream.addTrack(videoTrack)
      syncPeerConnectionTracks()
      updateLocalVideoPreview()
    }

    try {
      await requestVideoTrack()
      return
    } catch (error) {
      if (!audioConfig.video || !isMissingVideoDeviceError(error)) {
        throw error
      }
      await loadVideoDevices()
      const switched = normalizeSelectedVideoDevice()
      if (!switched) {
        throw error
      }
      logMsg('当前摄像头设备已失效，已自动切换到可用设备')
      await requestVideoTrack()
    }
  }

  const startAudio = async () => {
    // 创建只请求音频的配置副本，避免影响视频轨道
    const audioOnlyConfig = { ...audioConfig, video: false }

    await audioEngine.startCapture(
      audioOnlyConfig,
      () => {},
      () => {},
      async (samples) => {
        // 每次回调时实时获取 mediaWs 和 cryptoKey，避免闭包捕获过期引用
        const currentMediaWs = getMediaWs()
        const currentCryptoKey = getCryptoKey()
        if (!isSocketOpen(currentMediaWs) || !currentCryptoKey) return
        try {
          let encryptedBuffer
          if (audioConfig.quality === 'lossless') {
            const float32Data = samples.slice()
            encryptedBuffer = await encryptAudio(float32Data, currentCryptoKey)
          } else {
            const int16Data = float32ToInt16(samples)
            encryptedBuffer = await encryptAudio(int16Data, currentCryptoKey)
          }
          currentMediaWs!.send(encryptedBuffer)
        } catch (err) {
          console.error('Audio encryption failed', err)
        }
      }
    )

    syncAudioTrackOnly()
    updateLocalVideoPreview()
  }

  const stopAudio = () => {
    isCalling.value = false
    audioConfig.audio = false
    const clientId = getClientId()

    audioEngine.stopAudioOnly()
    // 只同步音频轨道，不影响视频
    syncAudioTrackOnly()

    if (isVideoOn.value) {
      updateLocalVideoPreview()
      logMsg('已关闭麦克风')
      return
    }

    audioEngine.stopCapture()
    logMsg('已关闭麦克风')

    const localVideo = document.getElementById(`video_${clientId}`) as HTMLVideoElement
    if (localVideo) {
      localVideo.srcObject = null
      localVideo.style.display = 'none'
    }
    
    const userIndex = usersWithVideo.value.findIndex(u => u.id === clientId)
    if (userIndex !== -1) {
      usersWithVideo.value.splice(userIndex, 1)
    }
  }

  const startCaptureWithVideoDeviceFallback = async () => {
    try {
      await startAudio()
    } catch (error) {
      throw error
    }
  }

  const bindAudioUnlockEvents = () => {
    if (audioUnlockBound) return
    const unlockAudio = async () => {
      await audioEngine.ensureAudioContextReady(audioConfig, () => {}, () => {})
      await audioEngine.ensurePlaybackWorkletReady(audioConfig, () => {}, () => {})
      await audioEngine.syncAllRemoteAudioPlayback()
    }
    document.addEventListener('click', unlockAudio, { passive: true })
    document.addEventListener('keydown', unlockAudio)
    document.addEventListener('touchstart', unlockAudio, { passive: true })
    audioUnlockBound = true
  }

  const getPeerConnection = (targetId: string) => {
    if (peerConnections[targetId]) return peerConnections[targetId]
    const controlWs = getControlWs()

    const pc = initPeerConnection(
      targetId,
      controlWs,
      audioEngine.mediaStream,
      audioConfig.quality,
      (id, stream) => {
        const remoteAudio = audioEngine.remoteAudioElements[id] || document.createElement('video')
        applyVideoElementStyle(remoteAudio)
        
        // 处理流合并，防止 ontrack 时单个流覆盖了整个对象
        if (!remoteAudio.srcObject) {
          remoteAudio.srcObject = stream
        } else if (remoteAudio.srcObject instanceof MediaStream) {
          stream.getTracks().forEach(track => {
            if (!(remoteAudio.srcObject as MediaStream).getTracks().includes(track)) {
              ;(remoteAudio.srcObject as MediaStream).addTrack(track)
            }
          })
          stream = remoteAudio.srcObject as MediaStream // 以合并后的流作为后续判断依据
        }

        remoteAudio.autoplay = true
        remoteAudio.setAttribute('playsinline', 'true')
        remoteAudio.id = `video_${id}`
        remoteAudio.className = 'user-video'
        remoteAudio.style.display = 'none'
        audioEngine.remoteAudioElements[id] = remoteAudio
        
        const updateVisibility = () => {
          const hasVideo = hasRenderableVideoTrack(stream)
          remoteAudio.style.display = hasVideo ? 'block' : 'none'
          const user = getCurrentRoomUsers().find(item => item.id === id)
          if (hasVideo && user?.video) {
            nextTick(() => {
              const updatedContainer = document.getElementById(`video_container_${id}`)
              if (updatedContainer && !updatedContainer.contains(remoteAudio)) {
                updatedContainer.appendChild(remoteAudio)
              }
            })
          }
        }
        const bindVideoTrackEvents = () => {
          stream.getVideoTracks().forEach(track => {
            track.onmute = updateVisibility
            track.onunmute = updateVisibility
            track.onended = updateVisibility
          })
        }
        bindVideoTrackEvents()
        updateVisibility()
        stream.onaddtrack = () => {
          bindVideoTrackEvents()
          updateVisibility()
        }
        stream.onremovetrack = updateVisibility

        if (!remoteAudio.isConnected) {
          document.body.appendChild(remoteAudio)
        }
        
        audioEngine.syncRemoteAudioElement(remoteAudio).catch(e => console.warn(e))
      },
      (id) => {
        const audioEl = audioEngine.remoteAudioElements[id]
        if (audioEl) {
          audioEl.remove()
          delete audioEngine.remoteAudioElements[id]
        }
        pc.close()
        delete peerConnections[id]
        
        const userIndex = usersWithVideo.value.findIndex(u => u.id === id)
        if (userIndex !== -1) {
          usersWithVideo.value.splice(userIndex, 1)
        }
      }
    )

    peerConnections[targetId] = pc
    return pc
  }

  const reportStatus = (status: string) => {
    const controlWs = getControlWs()
    if (isSocketOpen(controlWs)) {
      controlWs!.send(JSON.stringify({ type: 'update_status', status: status }))
    }
  }

  const toggleCall = async (forceState?: boolean | Event) => {
    const isEvent = forceState instanceof Event
    const targetState = (!isEvent && forceState !== undefined) ? forceState as boolean : !isCalling.value

    if (!targetState) {
      stopAudio()
      reportStatus('就绪')
      return
    }

    const isSomeoneTalking = getCurrentRoomUsers().some(u => u.status === '对讲中')
    if (audioConfig.mode === 'walkie-talkie' && isSomeoneTalking) {
      alert('当前频道已有其他人在讲话，请稍后再试或点击申请麦克风！')
      return
    }

    const mediaWs = getMediaWs()
    if (audioConfig.protocol !== 'webrtc' && !isSocketOpen(mediaWs)) {
      alert('媒体通道尚未建立，请稍后再试。')
      return
    }

    try {
      audioConfig.audio = true
      // 不改 audioConfig.video，startAudio 使用 audioOnlyConfig 独立处理
      if (isVideoOn.value && audioEngine.mediaStream) {
        await startAudio()
      } else {
        await startCaptureWithVideoDeviceFallback()
      }
      isCalling.value = true
      logMsg('已打开麦克风')
      reportStatus('对讲中')
    } catch (e: any) {
      audioConfig.audio = false
      logMsg('开启麦克风失败: ' + e.message)
      reportStatus('麦克风无权限')
    }
  }

  const toggleVideo = async () => {
    isVideoOn.value = !isVideoOn.value
    audioConfig.video = isVideoOn.value
    
    if (isVideoOn.value) {
      try {
        await startVideoTrackWithFallback()
        reportVideoState(true)
        logMsg('已打开摄像头')
        await loadVideoDevices()
      } catch (e: any) {
        isVideoOn.value = false
        audioConfig.video = false
        reportVideoState(false)
        logMsg('无法打开摄像头: ' + e.message)
      }
    } else {
      try {
        removeLocalVideoTracks()
        syncPeerConnectionTracks()
        updateLocalVideoPreview()
        reportVideoState(false)
        logMsg('已关闭摄像头')
        
        // If we just turned off video and audio is also off, stop capture completely
        if (!isCalling.value) {
          audioEngine.stopCapture()
        }
      } catch (e: any) {
        logMsg('关闭摄像头失败: ' + e.message)
      }
    }
  }

  const changeVideoDevice = async () => {
    audioConfig.videoDeviceId = selectedVideoDeviceId.value || undefined
    if (isVideoOn.value) {
      try {
        if (!audioEngine.mediaStream) {
          audioEngine.mediaStream = new MediaStream()
        }
        await startVideoTrackWithFallback()
        reportVideoState(true)
        logMsg('已切换摄像头')
      } catch (e: any) {
        console.error(e)
      }
    }
  }

  const toggleMute = () => {
    isMuted.value = !isMuted.value
    audioEngine.setMuted(isMuted.value)
    logMsg(isMuted.value ? '已静音' : '已开启扬声器')

    if (!isMuted.value) {
      audioEngine.ensureAudioContextReady(audioConfig, () => {}, () => {}).catch(e => console.warn(e))
      audioEngine.ensurePlaybackWorkletReady(audioConfig, () => {}, () => {}).catch(e => console.warn(e))
      audioEngine.syncAllRemoteAudioPlayback().catch(e => console.warn(e))
    }
  }

  const requestTalk = () => {
    const controlWs = getControlWs()
    if (isSocketOpen(controlWs)) {
      logMsg('已发送申请讲话请求，等待对方同意...')
      controlWs!.send(JSON.stringify({ type: 'request_talk' }))
      isRequestingTalk.value = true
      
      setTimeout(() => {
        if (isRequestingTalk.value) {
          isRequestingTalk.value = false
          logMsg('申请讲话超时未响应')
        }
      }, 5000)
    }
  }

  const handleMediaMessage = async (data: ArrayBuffer) => {
    if (audioEngine.getMuted()) return
    const cryptoKey = getCryptoKey()
    if (!cryptoKey) return
    const isAudioReady = await audioEngine.ensureAudioContextReady(audioConfig, () => {}, () => {})
    if (!isAudioReady || !audioEngine.context) return

    try {
      let pcmData: Float32Array
      if (audioConfig.quality === 'lossless') {
        const decryptedBuffer = await decryptAudio(data, cryptoKey)
        pcmData = new Float32Array(decryptedBuffer)
      } else {
        const int16Data = await decryptAudio(data, cryptoKey)
        pcmData = int16ToFloat32(new Int16Array(int16Data))
      }
      await audioEngine.enqueueAudioData(pcmData, audioConfig, () => {}, () => {})
    } catch (e) {
      console.warn('Audio Decryption Failed', e)
    }
  }

  const cleanupMedia = () => {
    usersWithVideo.value = []
    stopAudio()

    audioEngine.cleanup()
    
    Object.values(peerConnections).forEach(pc => pc.close())
    peerConnections = {}
    
    mediaChannelReady.value = false
    isRequestingTalk.value = false
  }

  const updatePeerConnectionsOnRoomInfo = (activeIds: string[]) => {
    Object.keys(peerConnections).forEach(peerId => {
      if (!activeIds.includes(peerId)) {
        peerConnections[peerId].close()
        delete peerConnections[peerId]
        const audioEl = audioEngine.remoteAudioElements[peerId]
        if (audioEl) {
          audioEl.remove()
          delete audioEngine.remoteAudioElements[peerId]
        }
        const userIndex = usersWithVideo.value.findIndex(u => u.id === peerId)
        if (userIndex !== -1) {
          usersWithVideo.value.splice(userIndex, 1)
        }
      }
    })
  }

  const setAudioConfig = (config: Partial<AudioRuntimeConfig>) => {
    audioConfig = { ...audioConfig, ...config }
  }
  
  const getAudioConfig = () => audioConfig

  return {
    audioEngine,
    usersWithVideo,
    hasAnyVideo,
    isCalling,
    isMuted,
    isVideoOn,
    videoDevices,
    selectedVideoDeviceId,
    mediaChannelReady,
    isRequestingTalk,
    showCallBtn,
    showRequestTalkBtn,
    isCallBtnDisabled,
    callBtnText,
    isRequestTalkBtnDisabled,
    requestTalkBtnText,
    
    setAudioConfig,
    getAudioConfig,
    bindAudioUnlockEvents,
    loadVideoDevices,
    getPeerConnection,
    toggleCall,
    toggleVideo,
    changeVideoDevice,
    toggleMute,
    requestTalk,
    syncUsersWithVideoFromRoomInfo,
    handleMediaMessage,
    cleanupMedia,
    updatePeerConnectionsOnRoomInfo,
    reportStatus
  }
}