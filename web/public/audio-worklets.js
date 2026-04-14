class CaptureAudioProcessor extends AudioWorkletProcessor {
  process(inputs, outputs) {
    const inputChannels = inputs[0]
    const outputChannels = outputs[0]
    const input = inputChannels && inputChannels[0]
    const output = outputChannels && outputChannels[0]

    if (output) {
      output.fill(0)
    }

    if (input && input.length > 0) {
      const samples = new Float32Array(input.length)
      samples.set(input)
      this.port.postMessage({ type: 'capture', samples }, [samples.buffer])
    }

    return true
  }
}

class PlaybackAudioProcessor extends AudioWorkletProcessor {
  constructor() {
    super()
    this.queue = []
    this.readOffset = 0
    this.queuedSamples = 0
    this.maxQueuedSamples = sampleRate * 2
    this.port.onmessage = event => {
      const message = event.data
      if (!message || typeof message !== 'object') {
        return
      }
      if (message.type === 'clear') {
        this.queue = []
        this.readOffset = 0
        this.queuedSamples = 0
        return
      }
      if (message.type !== 'push' || !(message.samples instanceof Float32Array) || message.samples.length === 0) {
        return
      }

      this.queue.push(message.samples)
      this.queuedSamples += message.samples.length

      while (this.queuedSamples > this.maxQueuedSamples && this.queue.length > 1) {
        const removed = this.queue.shift()
        if (!removed) {
          break
        }
        this.queuedSamples -= removed.length
        if (this.readOffset > 0) {
          this.readOffset = 0
        }
      }
    }
  }

  process(inputs, outputs) {
    const outputChannels = outputs[0]
    const output = outputChannels && outputChannels[0]
    if (!output) {
      return true
    }

    output.fill(0)

    let writeOffset = 0
    while (writeOffset < output.length && this.queue.length > 0) {
      const current = this.queue[0]
      const available = current.length - this.readOffset
      const copyLength = Math.min(available, output.length - writeOffset)
      output.set(current.subarray(this.readOffset, this.readOffset + copyLength), writeOffset)
      writeOffset += copyLength
      this.readOffset += copyLength
      this.queuedSamples -= copyLength

      if (this.readOffset >= current.length) {
        this.queue.shift()
        this.readOffset = 0
      }
    }

    return true
  }
}

registerProcessor('capture-audio-processor', CaptureAudioProcessor)
registerProcessor('playback-audio-processor', PlaybackAudioProcessor)
