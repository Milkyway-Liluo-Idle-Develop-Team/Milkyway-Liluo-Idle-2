<template>
  <div class="auth-page">
    <div class="card">
      <h1>登录</h1>

      <form class="form" @submit.prevent="onSubmit">
        <label class="field">
          <span>用户名</span>
          <input v-model.trim="username" autocomplete="username" placeholder="请输入用户名" />
        </label>

        <label class="field">
          <span>密码</span>
          <input
            v-model="password"
            type="password"
            autocomplete="current-password"
            placeholder="请输入密码"
          />
        </label>

        <p v-if="error" class="error">{{ error }}</p>

        <button class="primary" type="submit" :disabled="loading">
          {{ loading ? '登录中...' : '登录' }}
        </button>
      </form>

      <p class="footer">
        还没有账号？
        <RouterLink to="/register">去注册</RouterLink>
      </p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { postJson } from '@/lib/api'
import { clearAuthCache, setUserProfile } from '@/lib/auth'

const router = useRouter()
const route = useRoute()

const username = ref('')
const password = ref('')
const loading = ref(false)
const error = ref('')

type LoginResponse = { user: { uid: number; username: string; email: string; created_at: number }; session?: string; expires_at?: string }

const onSubmit = async () => {
  error.value = ''
  loading.value = true
  try {
    const res = await postJson<LoginResponse>(
      '/api/v1/auth/login',
      { username: username.value, password: password.value },
      { credentials: 'include' },
    )
    if (!res.ok) {
      error.value = res.error
      return
    }
    clearAuthCache()
    setUserProfile({
      uid: res.data.user.uid,
      username: res.data.user.username,
      email: res.data.user.email,
      created_at: 0,
    })
    const redirect = typeof route.query.redirect === 'string' ? route.query.redirect : '/main'
    window.location.href = redirect
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.auth-page {
  min-height: 100vh;
  display: grid;
  place-items: center;
  padding: 24px;
  background: radial-gradient(900px 520px at 10% -10%, rgba(0, 122, 204, 0.22), transparent 60%),
    radial-gradient(760px 520px at 92% -10%, rgba(0, 178, 148, 0.18), transparent 58%),
    linear-gradient(180deg, var(--bg), var(--bg-2));
  font-family: ui-sans-serif, system-ui, -apple-system, 'Segoe UI', Roboto, Arial, 'PingFang SC',
    'Microsoft YaHei', sans-serif;
  color: var(--text);
}

.card {
  width: min(420px, 100%);
  background: color-mix(in srgb, var(--surface) 92%, transparent);
  border: 1px solid var(--border);
  border-radius: 18px;
  padding: 22px 22px 18px;
  box-shadow: var(--shadow-md);
}

h1 {
  margin: 0 0 14px;
  font-size: 1.5rem;
  letter-spacing: 0.2px;
  background: linear-gradient(90deg, var(--brand), var(--brand-2));
  -webkit-background-clip: text;
  background-clip: text;
  -webkit-text-fill-color: transparent;
}

.form {
  display: grid;
  gap: 12px;
}

.field {
  display: grid;
  gap: 6px;
  font-weight: 700;
  color: var(--muted);
}

input {
  border-radius: 12px;
  padding: 10px 12px;
  border: 1px solid var(--border);
  background: color-mix(in srgb, var(--surface-2) 78%, transparent);
  color: var(--text);
  outline: none;
}

input:focus {
  border-color: color-mix(in srgb, var(--brand) 55%, transparent);
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--brand) 22%, transparent);
}

.error {
  margin: 0;
  color: var(--danger);
  font-weight: 700;
}

.primary {
  border: 1px solid rgba(255, 255, 255, 0.12);
  border-radius: 999px;
  padding: 10px 16px;
  font-weight: 800;
  cursor: pointer;
  background: linear-gradient(
    135deg,
    color-mix(in srgb, var(--brand) 82%, #000),
    color-mix(in srgb, var(--brand-2) 78%, #000)
  );
  color: rgba(255, 255, 255, 0.96);
}

.primary:disabled {
  cursor: not-allowed;
  opacity: 0.72;
}

.footer {
  margin: 14px 0 0;
  color: var(--muted);
  font-weight: 700;
}

a {
  color: var(--brand);
  text-decoration: none;
}

a:hover {
  text-decoration: underline;
}
</style>

