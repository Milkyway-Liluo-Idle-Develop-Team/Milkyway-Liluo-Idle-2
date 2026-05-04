<template>
  <div class="auth-page">
    <div class="card">
      <h1>注册</h1>

      <form class="form" @submit.prevent="onSubmit">
        <label class="field">
          <span>用户名</span>
          <input v-model.trim="username" autocomplete="username" placeholder="请输入用户名" />
        </label>

        <label class="field">
          <span>邮箱</span>
          <input v-model.trim="email" type="email" autocomplete="email" placeholder="请输入邮箱" />
        </label>

        <label class="field">
          <span>密码</span>
          <input
            v-model="password"
            type="password"
            autocomplete="new-password"
            placeholder="请输入密码"
          />
        </label>

        <label class="field">
          <span>确认密码</span>
          <input
            v-model="confirmPassword"
            type="password"
            autocomplete="new-password"
            placeholder="请再次输入密码"
          />
        </label>

        <p v-if="error" class="error">{{ error }}</p>

        <button class="primary" type="submit" :disabled="loading">
          {{ loading ? '注册中...' : '注册' }}
        </button>
      </form>

      <p class="footer">
        已有账号？
        <RouterLink to="/login">去登录</RouterLink>
      </p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { postJson } from '@/lib/api'
import { clearAuthCache, setUserProfile } from '@/lib/auth'

const router = useRouter()

const username = ref('')
const email = ref('')
const password = ref('')
const confirmPassword = ref('')
const loading = ref(false)
const error = ref('')

type RegisterResponse = { uid: number; username: string; email: string; created_at: number }

const onSubmit = async () => {
  error.value = ''
  if (!username.value || !email.value || !password.value) {
    error.value = '请填写用户名、邮箱和密码'
    return
  }
  if (password.value !== confirmPassword.value) {
    error.value = '两次输入的密码不一致'
    return
  }

  loading.value = true
  try {
    const res = await postJson<RegisterResponse>(
      '/api/v1/auth/register',
      { username: username.value, email: email.value, password: password.value },
      { credentials: 'include' },
    )
    if (!res.ok) {
      error.value = res.error
      return
    }
    clearAuthCache()
    setUserProfile({
      uid: res.data.uid,
      username: res.data.username,
      email: res.data.email,
      created_at: res.data.created_at,
    })
    window.location.href = '/main'
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
  width: min(460px, 100%);
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

