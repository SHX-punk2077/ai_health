<script setup lang="ts">
// AI 问答页骨架：消息流 + 输入框。接入 /api/v1/ai/chat 后替换 messages 数据源。
import { ref } from 'vue'

const input = ref('')
const messages = ref<{ role: 'user' | 'assistant'; content: string }[]>([
  { role: 'assistant', content: '你好，我是你的健康助手。可以问我运动计划、饮食建议等问题。' },
])

function onSend() {
  const text = input.value.trim()
  if (!text) return
  messages.value.push({ role: 'user', content: text })
  input.value = ''
  messages.value.push({ role: 'assistant', content: '（AI 服务待接入：AIService 尚未实现）' })
}
</script>

<template>
  <div class="chat">
    <div class="messages">
      <div v-for="(m, i) in messages" :key="i" :class="['msg', m.role]">
        <div class="bubble">{{ m.content }}</div>
      </div>
    </div>
    <div class="input">
      <el-input v-model="input" type="textarea" :rows="2" placeholder="描述你的健康问题…" @keyup.enter.exact="onSend" />
      <el-button type="primary" @click="onSend">发送</el-button>
    </div>
  </div>
</template>

<style scoped>
.chat {
  height: calc(100vh - 56px);
  display: flex;
  flex-direction: column;
  background: #fff;
}
.messages {
  flex: 1;
  overflow-y: auto;
  padding: 16px;
}
.msg {
  display: flex;
  margin-bottom: 12px;
}
.msg.user {
  justify-content: flex-end;
}
.bubble {
  max-width: 70%;
  padding: 10px 14px;
  border-radius: 8px;
  background: #f4f4f5;
  white-space: pre-wrap;
}
.msg.user .bubble {
  background: #409eff;
  color: #fff;
}
.input {
  display: flex;
  gap: 8px;
  padding: 12px 16px;
  border-top: 1px solid #ebeef5;
}
</style>
