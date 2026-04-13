import { isSocketOpen } from '../utils/helpers'

export const rtcConfig = { iceServers: [{ urls: 'stun:stun.l.google.com:19302' }] }

type PeerConnectionRuntimeState = {
  makingOffer: boolean
  pendingNegotiation: boolean
  ignoreOffer: boolean
  pendingCandidates: RTCIceCandidateInit[]
}

export const setHighBitrateSDP = (sdp: string, quality: string): string => {
  if (quality !== 'lossless') return sdp
  return sdp.replace(/a=fmtp:111(.*)/g, 'a=fmtp:111$1;stereo=1;sprop-stereo=1;maxaveragebitrate=510000;useinbandfec=1;cbr=1;minptime=10')
}

const getPeerConnectionRuntimeState = (pc: RTCPeerConnection): PeerConnectionRuntimeState => {
  const runtimePc = pc as RTCPeerConnection & { runtimeState?: PeerConnectionRuntimeState }
  if (!runtimePc.runtimeState) {
    runtimePc.runtimeState = {
      makingOffer: false,
      pendingNegotiation: false,
      ignoreOffer: false,
      pendingCandidates: []
    }
  }
  return runtimePc.runtimeState
}

const flushPendingCandidates = async (pc: RTCPeerConnection) => {
  const runtimeState = getPeerConnectionRuntimeState(pc)
  if (!runtimeState.pendingCandidates.length) return
  const candidates = [...runtimeState.pendingCandidates]
  runtimeState.pendingCandidates = []
  for (const candidate of candidates) {
    await pc.addIceCandidate(new RTCIceCandidate(candidate)).catch(e => console.error('Delayed candidate error:', e))
  }
}

export const initPeerConnection = (
  targetId: string,
  controlWs: WebSocket | null,
  mediaStream: MediaStream | null,
  quality: string,
  onTrack: (targetId: string, stream: MediaStream) => void,
  onClose: (targetId: string) => void
): RTCPeerConnection => {
  const pc = new RTCPeerConnection(rtcConfig)
  const runtimeState = getPeerConnectionRuntimeState(pc)

  const negotiate = async () => {
    if (pc.signalingState === 'closed') return
    if (runtimeState.makingOffer || pc.signalingState !== 'stable') {
      runtimeState.pendingNegotiation = true
      return
    }

    runtimeState.pendingNegotiation = false
    runtimeState.makingOffer = true
    try {
      const offer = await pc.createOffer()
      if (pc.signalingState !== 'stable') {
        runtimeState.pendingNegotiation = true
        return
      }
      offer.sdp = setHighBitrateSDP(offer.sdp || '', quality)
      await pc.setLocalDescription(offer)
      if (isSocketOpen(controlWs)) {
        controlWs!.send(JSON.stringify({ type: 'webrtc_offer', targetID: targetId, sdp: pc.localDescription || offer }))
      }
    } catch (e) {
      console.error('Negotiation error:', e)
    } finally {
      runtimeState.makingOffer = false
      if (runtimeState.pendingNegotiation && pc.signalingState === 'stable') {
        queueMicrotask(() => {
          negotiate().catch(e => console.error('Negotiation retry error:', e))
        })
      }
    }
  }

  pc.onicecandidate = event => {
    if (event.candidate && isSocketOpen(controlWs)) {
      controlWs!.send(JSON.stringify({ type: 'webrtc_candidate', targetID: targetId, candidate: event.candidate }))
    }
  }

  pc.onnegotiationneeded = () => {
    negotiate().catch(e => console.error('Negotiation error:', e))
  }

  pc.onsignalingstatechange = () => {
    if (pc.signalingState === 'stable' && runtimeState.pendingNegotiation && !runtimeState.makingOffer) {
      negotiate().catch(e => console.error('Negotiation error:', e))
    }
  }

  pc.ontrack = event => {
    // WebRTC ontrack 触发时流可能还不包含对应的轨道，确保直接传入对应的 track
    const stream = event.streams && event.streams[0] ? event.streams[0] : new MediaStream([event.track])
    onTrack(targetId, stream)
  }

  pc.oniceconnectionstatechange = () => {
    if (['disconnected', 'failed', 'closed'].includes(pc.iceConnectionState)) {
      onClose(targetId)
    }
  }

  if (mediaStream) {
    // 根据已有轨道创建对应类型的 transceiver
    const hasAudio = mediaStream.getAudioTracks().length > 0
    const hasVideo = mediaStream.getVideoTracks().length > 0

    if (hasAudio) {
      pc.addTransceiver(mediaStream.getAudioTracks()[0], { direction: 'sendrecv', streams: [mediaStream] })
    } else {
      pc.addTransceiver('audio', { direction: 'recvonly', streams: [] })
    }

    if (hasVideo) {
      pc.addTransceiver(mediaStream.getVideoTracks()[0], { direction: 'sendrecv', streams: [mediaStream] })
    } else {
      pc.addTransceiver('video', { direction: 'recvonly', streams: [] })
    }
  } else {
    // 初始化时就添加接收音频和视频的 transceiver，防止单向通信没有通道
    pc.addTransceiver('audio', { direction: 'recvonly', streams: [] })
    pc.addTransceiver('video', { direction: 'recvonly', streams: [] })
  }

  return pc
}

export const handleWebRTCSignal = async (
  data: any,
  clientId: string,
  quality: string,
  controlWs: WebSocket | null,
  getPeerConnection: (targetId: string) => RTCPeerConnection
) => {
  const fromId = data.fromID
  if (!fromId) return

  if (data.type === 'webrtc_offer') {
    const pc = getPeerConnection(fromId)
    const runtimeState = getPeerConnectionRuntimeState(pc)
    try {
      const isPolitePeer = clientId > fromId
      const hasOfferCollision = runtimeState.makingOffer || pc.signalingState !== 'stable'
      runtimeState.ignoreOffer = !isPolitePeer && hasOfferCollision

      if (runtimeState.ignoreOffer) {
        console.warn('Glare detected, ignoring incoming offer from', fromId)
        return
      }

      if (hasOfferCollision) {
        console.warn('Glare detected, rolling back local offer')
        await pc.setLocalDescription({ type: 'rollback' }).catch(e => console.warn('Rollback failed:', e))
      }

      await pc.setRemoteDescription(new RTCSessionDescription(data.sdp))
      await flushPendingCandidates(pc)

      const answer = await pc.createAnswer()
      answer.sdp = setHighBitrateSDP(answer.sdp || '', quality)
      await pc.setLocalDescription(answer)

      if (isSocketOpen(controlWs)) {
        controlWs!.send(JSON.stringify({ type: 'webrtc_answer', targetID: fromId, sdp: pc.localDescription || answer }))
      }
    } catch (e) {
      console.error('Handle offer error:', e)
    }
  } else if (data.type === 'webrtc_answer') {
    const pc = getPeerConnection(fromId)
    try {
      if (pc.signalingState === 'have-local-offer') {
        await pc.setRemoteDescription(new RTCSessionDescription(data.sdp))
        await flushPendingCandidates(pc)
      }
    } catch (e) {
      console.error('Handle answer error:', e)
    }
  } else if (data.type === 'webrtc_candidate') {
    const pc = getPeerConnection(fromId)
    const runtimeState = getPeerConnectionRuntimeState(pc)
    try {
      if (pc.remoteDescription) {
        await pc.addIceCandidate(new RTCIceCandidate(data.candidate))
      } else {
        runtimeState.pendingCandidates.push(data.candidate)
      }
    } catch (e) {
      console.error('Handle candidate error:', e)
    }
  }
}
