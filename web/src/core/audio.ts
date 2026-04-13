export type AudioRuntimeConfig = {
  mode: string
  quality: string
  sampleRate: number
  bufferSize: number
  protocol: string
  audio?: boolean
  video?: boolean
  videoDeviceId?: string
}

export type CaptureWorkletMessage = {
  type: 'capture'
  samples: Float32Array
}

export const buildVideoConstraints = (config: AudioRuntimeConfig): MediaTrackConstraints | false => {
  if (!config.video) return false
  const videoConstraints: MediaTrackConstraints = {
    aspectRatio: { ideal: 16 / 9 },
    width: { ideal: 1920 },
    height: { ideal: 1080 },
    frameRate: { ideal: 30, max: 60 }
  }
  if (config.videoDeviceId) {
    videoConstraints.deviceId = { exact: config.videoDeviceId }
  }
  ;(videoConstraints as MediaTrackConstraints & { resizeMode?: string }).resizeMode = 'none'
  return videoConstraints
}

export class AudioEngine {
  public context: AudioContext | null = null
  public outputGainNode: GainNode | null = null
  public captureMonitorGainNode: GainNode | null = null
  public captureWorkletNode: AudioWorkletNode | null = null
  public playbackWorkletNode: AudioWorkletNode | null = null
  public audioInput: MediaStreamAudioSourceNode | null = null
  public mediaStream: MediaStream | null = null
  public remoteAudioElements: Record<string, HTMLMediaElement> = {}
  
  private audioWorkletsReadyPromise: Promise<void> | null = null
  private isMuted: boolean = false
  public logMsg: (msg: string) => void

  constructor(logMsg: (msg: string) => void) {
    this.logMsg = logMsg
  }

  setMuted(muted: boolean) {
    this.isMuted = muted
    if (this.outputGainNode) {
      this.outputGainNode.gain.value = muted ? 0 : 1
    }
    Object.values(this.remoteAudioElements).forEach(audioEl => {
      audioEl.muted = muted
    })
  }

  getMuted(): boolean {
    return this.isMuted
  }

  ensureOutputGainReady() {
    if (!this.context) return null
    if (!this.outputGainNode) {
      this.outputGainNode = this.context.createGain()
      this.outputGainNode.connect(this.context.destination)
    }
    this.outputGainNode.gain.value = this.isMuted ? 0 : 1
    return this.outputGainNode
  }

  async syncRemoteAudioElement(audioEl: HTMLMediaElement, onBlocked?: () => void) {
    audioEl.muted = this.isMuted
    try {
      await audioEl.play()
    } catch (e) {
      if (onBlocked) onBlocked()
    }
  }

  async syncAllRemoteAudioPlayback(onBlocked?: () => void) {
    await Promise.allSettled(
      Object.values(this.remoteAudioElements).map(audioEl => this.syncRemoteAudioElement(audioEl, onBlocked))
    )
  }

  async ensureAudioContextReady(
    config: AudioRuntimeConfig,
    onReady: () => void,
    onBlocked: () => void
  ): Promise<boolean> {
    if (!this.context || this.context.state === 'closed') {
      const contextOptions: AudioContextOptions = {}
      if (config.sampleRate && config.sampleRate > 0) {
        contextOptions.sampleRate = config.sampleRate
      }
      this.context = new (window.AudioContext || (window as any).webkitAudioContext)(contextOptions)
      console.log('AudioContext initialized with sample rate:', this.context.sampleRate)
      this.outputGainNode = null
      this.captureMonitorGainNode = null
      this.captureWorkletNode = null
      this.playbackWorkletNode = null
      this.audioWorkletsReadyPromise = null
      this.ensureOutputGainReady()
    }

    if (this.context.state === 'suspended') {
      try {
        await this.context.resume()
      } catch (e) {
        onBlocked()
      }
    }

    this.ensureOutputGainReady()

    if (this.context.state === 'running') {
      onReady()
      
      if ('mediaSession' in navigator) {
        navigator.mediaSession.metadata = null
        navigator.mediaSession.playbackState = 'none'
        const actionHandlers = ['play', 'pause', 'seekbackward', 'seekforward', 'previoustrack', 'nexttrack', 'stop']
        actionHandlers.forEach(action => {
          try {
            navigator.mediaSession.setActionHandler(action as MediaSessionAction, null)
          } catch (e) {}
        })
      }
      
      return true
    }

    return false
  }

  async ensureAudioWorkletsReady(config: AudioRuntimeConfig, onReady: () => void, onBlocked: () => void): Promise<boolean> {
    const isAudioReady = await this.ensureAudioContextReady(config, onReady, onBlocked)
    if (!this.context || !isAudioReady) return false

    if (!this.audioWorkletsReadyPromise) {
      this.audioWorkletsReadyPromise = this.context.audioWorklet.addModule('/audio-worklets.js').catch(error => {
        this.audioWorkletsReadyPromise = null
        throw error
      })
    }

    await this.audioWorkletsReadyPromise
    return true
  }

  async ensurePlaybackWorkletReady(config: AudioRuntimeConfig, onReady: () => void, onBlocked: () => void): Promise<boolean> {
    const isWorkletReady = await this.ensureAudioWorkletsReady(config, onReady, onBlocked)
    if (!isWorkletReady || !this.context) return false

    if (!this.playbackWorkletNode) {
      const outputNode = this.ensureOutputGainReady()
      if (!outputNode) return false
      this.playbackWorkletNode = new AudioWorkletNode(this.context, 'playback-audio-processor', {
        numberOfInputs: 0,
        numberOfOutputs: 1,
        outputChannelCount: [1]
      })
      this.playbackWorkletNode.connect(outputNode)
    }

    return true
  }

  async enqueueAudioData(pcmFloat32Data: Float32Array, config: AudioRuntimeConfig, onReady: () => void, onBlocked: () => void) {
    const isPlaybackReady = await this.ensurePlaybackWorkletReady(config, onReady, onBlocked)
    if (!isPlaybackReady || !this.playbackWorkletNode) return
    this.playbackWorkletNode.port.postMessage({ type: 'push', samples: pcmFloat32Data }, [pcmFloat32Data.buffer])
  }

  async startCapture(
    config: AudioRuntimeConfig,
    onReady: () => void,
    onBlocked: () => void,
    onCapture: (samples: Float32Array) => void
  ): Promise<MediaStream> {
    await this.ensureAudioContextReady(config, onReady, onBlocked)
    const isWorkletReady = await this.ensureAudioWorkletsReady(config, onReady, onBlocked)
    if (!isWorkletReady || !this.context) {
      throw new Error('音频处理模块加载失败')
    }

    const isLossless = config.quality === 'lossless'
    const videoConstraints = buildVideoConstraints(config)

    const constraints: MediaStreamConstraints = {
      audio: config.audio === false ? false : {
        echoCancellation: !isLossless,
        noiseSuppression: !isLossless,
        autoGainControl: false
      },
      video: videoConstraints
    }
    
    if (isLossless && config.protocol === 'webrtc') {
      (constraints.audio as any).channelCount = 2;
      (constraints.audio as any).sampleRate = 48000;
    }

    if (!isLossless && config.sampleRate && config.sampleRate > 0) {
      (constraints.audio as any).sampleRate = config.sampleRate
    }

    let existingVideoTrack: MediaStreamTrack | null = null
    if (this.mediaStream) {
      existingVideoTrack = this.mediaStream.getVideoTracks()[0] || null
      // 只停止音频轨道，保留视频轨道不受影响
      this.mediaStream.getAudioTracks().forEach(track => {
        track.stop()
        this.mediaStream?.removeTrack(track)
      })
    }

    // 已有视频轨道时不请求新视频；config.video 为 false 也不请求，但不停止已有视频
    if (existingVideoTrack || !config.video) {
      constraints.video = false
    }

    if (!this.mediaStream) {
      this.mediaStream = new MediaStream()
    }

    if (constraints.audio || constraints.video) {
      const newStream = await navigator.mediaDevices.getUserMedia(constraints)
      newStream.getTracks().forEach(track => {
        this.mediaStream!.addTrack(track)
      })
    }

    if (existingVideoTrack && config.video && !this.mediaStream.getVideoTracks().includes(existingVideoTrack)) {
      this.mediaStream.addTrack(existingVideoTrack)
    }

    if (config.audio !== false) {
      const audioTrack = this.mediaStream.getAudioTracks()[0]
      if (audioTrack) {
        this.logMsg(`音频轨道设置: ${JSON.stringify(audioTrack.getSettings())}`)
      } else {
        this.logMsg('已请求麦克风，但未获取到音频轨道')
      }
    }

    if (config.video) {
      const videoTrack = this.mediaStream.getVideoTracks()[0]
      if (videoTrack) {
        const settings = videoTrack.getSettings()
        const resolutionText = settings.width && settings.height
          ? `${settings.width}x${settings.height}`
          : '未知'
        this.logMsg(`采集到的实际分辨率: ${resolutionText}`)
        this.logMsg(`视频轨道设置: ${JSON.stringify(settings)}`)
      } else {
        this.logMsg('视频已开启，但未获取到视频轨道')
      }
    }

    if (config.protocol === 'webrtc') {
      return this.mediaStream
    }

    if (config.audio !== false) {
      this.audioInput = this.context.createMediaStreamSource(this.mediaStream)
      const outputNode = this.ensureOutputGainReady()
      
      if (!this.captureMonitorGainNode) {
        this.captureMonitorGainNode = this.context.createGain()
        this.captureMonitorGainNode.gain.value = 0
        this.captureMonitorGainNode.connect(outputNode || this.context.destination)
      }

      this.captureWorkletNode = new AudioWorkletNode(this.context, 'capture-audio-processor', {
        numberOfInputs: 1,
        numberOfOutputs: 1,
        channelCount: 1,
        outputChannelCount: [1]
      })

      this.captureWorkletNode.port.onmessage = (event: MessageEvent<CaptureWorkletMessage>) => {
        if (event.data?.type !== 'capture') return
        onCapture(event.data.samples)
      }

      this.audioInput.connect(this.captureWorkletNode)
      this.captureWorkletNode.connect(this.captureMonitorGainNode)
    }

    return this.mediaStream
  }

  stopAudioOnly() {
    if (this.audioInput) {
      this.audioInput.disconnect()
      this.audioInput = null
    }
    if (this.captureWorkletNode) {
      this.captureWorkletNode.port.onmessage = null
      this.captureWorkletNode.disconnect()
      this.captureWorkletNode = null
    }
    if (this.mediaStream) {
      this.mediaStream.getAudioTracks().forEach(track => track.stop())
      this.mediaStream.getAudioTracks().forEach(track => this.mediaStream?.removeTrack(track))
    }
  }

  stopCapture() {
    this.stopAudioOnly()
    if (this.mediaStream) {
      this.mediaStream.getVideoTracks().forEach(track => track.stop())
      this.mediaStream = null
    }
  }

  cleanup() {
    this.stopCapture()
    if (this.context) {
      this.context.close()
      this.context = null
    }
    this.outputGainNode = null
    this.captureMonitorGainNode = null
    this.captureWorkletNode = null
    this.playbackWorkletNode = null
    this.audioWorkletsReadyPromise = null
    
    Object.keys(this.remoteAudioElements).forEach(peerId => {
      this.remoteAudioElements[peerId].remove()
      delete this.remoteAudioElements[peerId]
    })
  }

  static async checkMicPermission(): Promise<boolean> {
    try {
      const stream = await navigator.mediaDevices.getUserMedia({ audio: true, video: false })
      stream.getTracks().forEach(track => track.stop())
      return true
    } catch (e) {
      return false
    }
  }
}
