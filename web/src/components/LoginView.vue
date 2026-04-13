<template>
  <div class="card-container login-container" style="max-width: 400px;">
    <h1>加入频道</h1>
    <div class="form-group">
      <md-outlined-text-field
        label="输入频道密码"
        type="password"
        minlength="8"
        maxlength="20"
        :value="roomKey"
        @input="$emit('update:roomKey', ($event.target as HTMLInputElement).value)"
        @keyup.enter="$emit('join')"
      ></md-outlined-text-field>
    </div>
    <div style="text-align: right; margin-top: 24px;">
      <md-filled-button @click="$emit('join')" :disabled="isConnecting">
        {{ isConnecting ? '连接中...' : '进入频道' }}
      </md-filled-button>
    </div>
  </div>
</template>

<script setup lang="ts">
defineProps<{
  roomKey: string
  isConnecting: boolean
}>()
defineEmits<{
  (e: 'update:roomKey', value: string): void
  (e: 'join'): void
}>()
</script>
