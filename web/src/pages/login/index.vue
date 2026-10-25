<template>
  <div class="login-page">
    <div class="login-card">
      <div class="brand">
        <div class="logo">台州府城墙</div>
        <div class="slogan">古城墙病害巡检系统 · PC 管理端</div>
      </div>
      <el-form :model="form" label-position="top" @submit.prevent="submit">
        <el-form-item label="账号">
          <el-input v-model="form.username" placeholder="admin / reviewer" />
        </el-form-item>
        <el-form-item label="密码">
          <el-input v-model="form.password" type="password" show-password placeholder="Admin123 / Review123" />
        </el-form-item>
        <el-button type="primary" native-type="submit" :loading="loading" class="submit" block>
          登 录
        </el-button>
      </el-form>
      <div class="tip">本地演示账号：admin / Admin123（管理员）、reviewer / Review123（审核员）</div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { login } from '@/shared/api/auth'

const router = useRouter()
const route = useRoute()
const form = reactive({ username: 'admin', password: 'Admin123' })
const loading = ref(false)

async function submit() {
  if (!form.username || !form.password) {
    ElMessage.warning('请输入账号与密码')
    return
  }
  loading.value = true
  try {
    const session = await login(form.username, form.password)
    localStorage.setItem('city_wall_token', session.token)
    localStorage.setItem('city_wall_roles', JSON.stringify(session.roles))
    localStorage.setItem('city_wall_user', JSON.stringify(session.user))
    ElMessage.success('登录成功')
    const redirect = (route.query.redirect as string) || '/tasks'
    router.replace(redirect)
  } catch (e) {
    ElMessage.error('登录失败，请检查账号密码或服务状态')
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.login-page {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #f5f1e8;
}
.login-card {
  width: 380px;
  background: #fff;
  border-radius: 12px;
  padding: 36px 32px;
  box-shadow: 0 8px 30px rgba(43, 43, 43, 0.08);
}
.brand {
  margin-bottom: 28px;
}
.logo {
  font-size: 22px;
  font-weight: 700;
  color: #25584f;
}
.slogan {
  font-size: 13px;
  color: #6b6b6b;
  margin-top: 6px;
}
.submit {
  width: 100%;
  height: 40px;
}
.tip {
  margin-top: 18px;
  font-size: 12px;
  color: #9b9b9b;
  line-height: 1.6;
}
</style>
