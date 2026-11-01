<template>
  <div class="login-page">
    <div class="login-card">
      <div class="brand">
        <div class="login-mark"><AppIcon name="wall" /></div>
        <div class="logo">城墙巡检</div>
        <div class="slogan">古城墙病害巡检系统 · PC 管理端</div>
      </div>
      <el-alert v-if="route.query.expired === '1'" title="登录已失效，请重新登录后继续。" type="warning" :closable="false" show-icon class="session-notice" />
      <el-alert v-if="error" :title="error" type="error" :closable="false" show-icon class="session-notice" />
      <el-form :model="form" label-position="top" @compositionstart="composing = true" @compositionend="endComposition" @keydown.enter.capture="guardEnter" @submit.prevent="submit">
        <el-form-item label="账号">
          <el-input v-model="form.username" :disabled="loading" autocomplete="username" placeholder="请输入巡检管理账号" />
        </el-form-item>
        <el-form-item label="密码">
          <el-input v-model="form.password" :disabled="loading" type="password" autocomplete="current-password" show-password placeholder="请输入密码" />
        </el-form-item>
        <el-button type="primary" native-type="submit" :loading="loading" class="submit" block>
          {{ loading ? '正在登录…' : '登录工作台' }}
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
import AppIcon from '@/shared/ui/AppIcon.vue'

const router = useRouter()
const route = useRoute()
const form = reactive({ username: 'admin', password: 'Admin123' })
const loading = ref(false)
const error = ref('')
const composing = ref(false)
let compositionEndedAt = 0
function endComposition() { composing.value = false; compositionEndedAt = Date.now() }
function guardEnter(event: KeyboardEvent) { if (event.isComposing || composing.value || event.keyCode === 229 || Date.now() - compositionEndedAt < 500) event.preventDefault() }

async function submit() {
  if (loading.value) return
  error.value = ''
  if (!form.username.trim() || !form.password) {
    error.value = '请填写账号和密码后登录。'
    return
  }
  loading.value = true
  try {
    const session = await login(form.username.trim(), form.password)
    localStorage.setItem('city_wall_token', session.token)
    localStorage.setItem('city_wall_roles', JSON.stringify(session.roles))
    localStorage.setItem('city_wall_user', JSON.stringify(session.user))
    ElMessage.success('登录成功')
    const redirect = (route.query.redirect as string) || (session.roles.some(role => ['ADMIN', 'REVIEWER'].includes(role)) ? '/overview' : '/tasks')
    router.replace(redirect)
  } catch (e) {
    error.value = e instanceof Error ? e.message : '登录未能完成，请稍后重试。'
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
  background: var(--app-background);
  padding: var(--space-content);
  box-sizing: border-box;
}
.login-card {
  box-sizing: border-box;
  width: min(380px, 100%);
  background: var(--surface);
  border-radius: var(--radius-panel);
  padding: 36px 32px;
  box-shadow: 0 8px 30px rgba(43, 43, 43, 0.08);
}
.brand {
  margin-bottom: 28px;
}
.session-notice {
  margin-bottom: 20px;
}
.logo {
  font-size: 22px;
  font-weight: 700;
  color: var(--brand);
}
.slogan {
  font-size: 13px;
  color: var(--text-muted);
  margin-top: 6px;
}
.submit {
  width: 100%;
  height: 40px;
}
.tip {
  margin-top: 18px;
  font-size: 12px;
  color: var(--text-muted);
  line-height: 1.6;
}
.login-mark { --icon-size: 30px; width: 54px; height: 54px; border-radius: var(--radius-panel); background: var(--brand-ink); color: var(--brand-accent); display: grid; place-items: center; margin-bottom: var(--space-content); }
</style>
