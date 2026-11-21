<script setup lang="ts">
// 健康档案页：表单字段与后端 dto/request.ProfileSaveReq 一一对应。
// 后端 HealthService 尚未实现（返回 10003），这里先展示骨架与字段分组。
const groups = [
  { title: '基础信息', fields: ['真实姓名', '年龄', '性别', '出生日期'] },
  { title: '体征指标', fields: ['身高 cm', '体重 kg', '收缩压/舒张压', '心率', '视力 左/右'] },
  { title: '生化指标', fields: ['血糖 mmol/L', '总胆固醇 mmol/L'] },
  { title: '生活方式', fields: ['睡眠时长', '睡眠质量', '吸烟/饮酒', '运动习惯', '每日饮水量'] },
]
</script>

<template>
  <div class="page-container">
    <el-alert
      type="info"
      :closable="false"
      title="后端 HealthService 待实现"
      description="接口 /api/v1/health/profile 目前返回 code=10003；表单结构已按 ProfileSaveReq 预留。"
      style="margin-bottom: 16px"
    />
    <el-row :gutter="16">
      <el-col v-for="g in groups" :key="g.title" :span="12">
        <el-card class="page-card" style="margin-bottom: 16px">
          <template #header>{{ g.title }}</template>
          <el-space wrap>
            <el-tag v-for="f in g.fields" :key="f" type="info">{{ f }}</el-tag>
          </el-space>
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>
