<script setup lang="ts">
import { onMounted } from 'vue'
import { useUserStore } from '@/stores/user'

const userStore = useUserStore()

onMounted(() => {
  // 进入后台先补齐用户信息（刷新页面时 store 会丢）
  if (!userStore.user) {
    void userStore.fetchProfile()
  }
})

const cards = [
  { title: '健康档案', desc: '身高 / 体重 / 血压 / 血糖等基线信息', to: '/health/profile' },
  { title: '健康记录', desc: '按时间轴记录每次测量，支持趋势对比', to: '/health/profile' },
  { title: 'AI 健康问答', desc: '结合档案给出运动与作息建议', to: '/ai/chat' },
]
</script>

<template>
  <div class="page-container">
    <el-row :gutter="16">
      <el-col v-for="c in cards" :key="c.title" :span="8">
        <el-card shadow="hover" class="card" @click="$router.push(c.to)">
          <h3>{{ c.title }}</h3>
          <p>{{ c.desc }}</p>
        </el-card>
      </el-col>
    </el-row>

    <el-card class="page-card" style="margin-top: 16px">
      <template #header>当前账号</template>
      <el-descriptions :column="3" border>
        <el-descriptions-item label="用户名">{{ userStore.user?.username }}</el-descriptions-item>
        <el-descriptions-item label="昵称">{{ userStore.user?.nickname }}</el-descriptions-item>
        <el-descriptions-item label="角色">
          {{ (userStore.roles || []).join(' / ') || '-' }}
        </el-descriptions-item>
      </el-descriptions>
    </el-card>
  </div>
</template>

<style scoped>
.card {
  cursor: pointer;
}
.card h3 {
  margin: 0 0 8px;
}
.card p {
  margin: 0;
  color: #909399;
  font-size: 13px;
}
</style>
