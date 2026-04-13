import { ref, nextTick } from 'vue'

export function useLogs() {
  const logs = ref<string[]>([])
  const logsContainer = ref<HTMLElement | null>(null)
  const showLogs = ref(false)

  const toggleLogs = () => {
    showLogs.value = !showLogs.value
    if (showLogs.value) {
      nextTick(() => {
        if (logsContainer.value) {
          logsContainer.value.scrollTop = logsContainer.value.scrollHeight
        }
      })
    }
  }

  const logMsg = (msg: string) => {
    const time = new Date().toLocaleTimeString()
    logs.value.push(`[${time}] ${msg}`)
    nextTick(() => {
      if (logsContainer.value) {
        logsContainer.value.scrollTop = logsContainer.value.scrollHeight
      }
    })
  }

  const clearLogs = () => {
    logs.value = []
  }

  return {
    logs,
    logsContainer,
    showLogs,
    toggleLogs,
    logMsg,
    clearLogs
  }
}
