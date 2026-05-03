<template>
  <div class="auth-page">
    <div class="auth-card">
      <h1 class="brand">
        <span class="gradient">MLI</span>
      </h1>
      <div class="tabs">
        <button :class="{ active: mode === 'login' }" @click="switchMode('login')">登录</button>
        <button :class="{ active: mode === 'register' }" @click="switchMode('register')">注册</button>
      </div>
      <form @submit.prevent="onSubmit">
        <div class="field">
          <label>用户名</label>
          <input v-model="form.username" type="text" required autocomplete="username" />
        </div>
        <div class="field">
          <label>密码</label>
          <input v-model="form.password" type="password" required autocomplete="current-password" />
        </div>
        <div v-if="mode === 'register'" class="field">
          <label>确认密码</label>
          <input v-model="form.confirmPassword" type="password" required autocomplete="new-password" />
        </div>
        <div v-if="error" class="error">{{ error }}</div>
        <button type="submit" class="submit" :disabled="loading">
          {{ loading ? '请稍候...' : mode === 'login' ? '登录' : '注册' }}
        </button>
      </form>
    </div>
    <button class="theme-toggle" @click="toggleTheme()" title="切换主题">
      {{ isDark ? '☀️' : '🌙' }}
    </button>
  </div>
</template>

<script setup lang="ts">
import { reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { postJson } from '@/lib/api'
import { setUserProfile, clearAuthCache } from '@/lib/auth'
import { isDark, toggleTheme } from '@/composables/useTheme'

const props = defineProps<{ mode: 'login' | 'register' }>()
const router = useRouter()

const form = reactive({ username: '', password: '', confirmPassword: '' })
const loading = ref(false)
const error = ref('')

function switchMode(m: 'login' | 'register') {
  error.value = ''
  router.replace({ name: m })
}

async function onSubmit() {
  error.value = ''
  if (props.mode === 'register' && form.password !== form.confirmPassword) {
    error.value = '两次输入的密码不一致'
    return
  }
  loading.value = true
  try {
    const endpoint = props.mode === 'login' ? '/api/v1/auth/login' : '/api/v1/auth/register'
    type UserResp = { id: number; username: string; email: string; created_at: number }
    type LoginResp = { user: UserResp; session?: string; expires_at?: string }
    const res = await postJson<UserResp | LoginResp>(
      endpoint,
      { username: form.username, password: form.password },
      { credentials: 'include' },
    )
    if (!res.ok) {
      error.value = res.error
      return
    }
    clearAuthCache()
    let user: UserResp
    if ('user' in res.data) {
      user = res.data.user
    } else {
      user = res.data as UserResp
    }
    setUserProfile({
      uid: user.id,
      username: user.username,
      email: user.email,
      created_at: user.created_at,
    })
    router.push('/main')
  } catch (e: any) {
    error.value = e.message || '请求失败'
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.auth-page {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--bg);
}
.auth-card {
  width: 360px;
  padding: 28px;
  border-radius: 16px;
  background: var(--surface);
  box-shadow: var(--shadow-sm);
  border: 1px solid var(--border);
}
.brand {
  text-align: center;
  margin: 0 0 20px;
  font-size: 28px;
}
.gradient {
  background: linear-gradient(90deg, var(--brand), var(--brand-2));
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
}
.tabs {
  display: flex;
  gap: 8px;
  margin-bottom: 20px;
}
.tabs button {
  flex: 1;
  padding: 10px;
  border: none;
  border-radius: 8px;
  background: var(--surface-2);
  color: var(--muted);
  cursor: pointer;
  font-size: 14px;
}
.tabs button.active {
  background: var(--brand);
  color: #fff;
}
.field {
  margin-bottom: 14px;
}
.field label {
  display: block;
  margin-bottom: 6px;
  font-size: 13px;
  color: var(--muted);
}
.field input {
  width: 100%;
  padding: 10px 12px;
  border-radius: 8px;
  border: 1px solid var(--border);
  background: var(--bg);
  color: var(--text);
  font-size: 14px;
  box-sizing: border-box;
}
.error {
  color: var(--danger);
  font-size: 13px;
  margin-bottom: 12px;
}
.submit {
  width: 100%;
  padding: 12px;
  border: none;
  border-radius: 8px;
  background: linear-gradient(90deg, var(--brand), var(--brand-2));
  color: #fff;
  font-size: 15px;
  font-weight: 600;
  cursor: pointer;
}
.submit:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}
.theme-toggle {
  position: fixed;
  top: 16px;
  right: 16px;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 8px 12px;
  font-size: 18px;
  cursor: pointer;
}
</style>
