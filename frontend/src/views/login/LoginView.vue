<script setup lang="ts">
import { reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, type FormInstance, type FormRules } from 'element-plus'
import { useUserStore } from '@/stores/user'

const route = useRoute()
const router = useRouter()
const userStore = useUserStore()

const formRef = ref<FormInstance>()
const form = reactive({ account: '', password: '' })

const rules: FormRules = {
  account: [{ required: true, message: '请输入用户名 / 邮箱 / 手机号', trigger: 'blur' }],
  password: [{ required: true, min: 8, message: '密码至少 8 位', trigger: 'blur' }],
}

async function onSubmit() {
  if (!formRef.value) return
  const valid = await formRef.value.validate().catch(() => false)
  if (!valid) return

  try {
    await userStore.login(form.account, form.password)
    const redirect = (route.query.redirect as string) || '/dashboard'
    await router.replace(redirect)
    ElMessage.success('登录成功')
  } catch {
    // 错误提示已在 axios 拦截器统一处理
  }
}
</script>

<template>
  <div class="login">
    <el-card class="box">
      <h2 class="title">AI 健康管理系统</h2>
      <el-form ref="formRef" :model="form" :rules="rules" label-position="top" @keyup.enter="onSubmit">
        <el-form-item label="账号" prop="account">
          <el-input v-model="form.account" placeholder="用户名 / 邮箱 / 手机号" clearable />
        </el-form-item>
        <el-form-item label="密码" prop="password">
          <el-input v-model="form.password" type="password" show-password placeholder="请输入密码" />
        </el-form-item>
        <el-button type="primary" class="submit" :loading="userStore.loading" @click="onSubmit">
          登录
        </el-button>
      </el-form>
      <p class="tip">首次使用请先调用 /api/v1/auth/register 注册</p>
    </el-card>
  </div>
</template>

<style scoped>
.login {
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(135deg, #1f3b73, #4a90d9);
}
.box {
  width: 380px;
  padding: 8px 12px;
}
.title {
  text-align: center;
  margin: 8px 0 24px;
}
.submit {
  width: 100%;
}
.tip {
  margin-top: 16px;
  color: #909399;
  font-size: 12px;
  text-align: center;
}
</style>
