<template>
  <div class="auth-page">
    <div class="flip-wrap">
      <div ref="authCardEl" class="auth-card" :class="cardClass" :style="cardStyle">
        <div ref="contentEl" class="card-content">
          <div ref="cardHeadEl" class="card-head">
            <h1>{{ displayMode === 'login' ? '登录' : '注册' }}</h1>
            <button
              class="theme-switch"
              type="button"
              role="switch"
              :aria-checked="theme === 'dark'"
              :class="{ 'is-dark': theme === 'dark' }"
              @click="toggleTheme"
              :aria-label="themeAriaLabel"
            >
              <span class="thumb" aria-hidden="true">
                <svg v-if="theme === 'dark'" class="theme-icon" viewBox="0 0 24 24" fill="none">
                  <path d="M21 14.2A8.2 8.2 0 0 1 9.8 3a6.8 6.8 0 1 0 11.2 11.2Z" />
                </svg>
                <svg v-else class="theme-icon" viewBox="0 0 24 24" fill="none">
                  <circle cx="12" cy="12" r="4.5" />
                  <path d="M12 2.5v2.5M12 19v2.5M21.5 12h-2.5M5 12H2.5" />
                  <path d="M19.4 4.6l-1.8 1.8M6.4 17.6l-1.8 1.8" />
                  <path d="M19.4 19.4l-1.8-1.8M6.4 6.4L4.6 4.6" />
                </svg>
              </span>
              <span class="sr-only">切换主题</span>
            </button>
          </div>

          <div class="mode-slot">
            <div ref="loginEl" class="mode" :class="{ active: displayMode === 'login' }">
              <form id="login-form" class="form form--login" @submit.prevent="onLogin">
                <div class="fields-stack">
                  <label class="field" data-register-field="username">
                    <span class="sr-only">用户名</span>
                    <div class="input-row">
                      <span class="input-icon" aria-hidden="true">
                        <svg class="field-icon" viewBox="0 0 24 24" fill="none">
                          <path d="M20 21a8 8 0 0 0-16 0" />
                          <circle cx="12" cy="8" r="4" />
                        </svg>
                      </span>
                      <input
                        v-model.trim="loginUsername"
                        autocomplete="username"
                        aria-label="用户名"
                      />
                    </div>
                  </label>

                  <label class="field" data-register-field="email">
                    <span class="sr-only">密码</span>
                    <div class="input-row">
                      <span class="input-icon" aria-hidden="true">
                        <svg class="field-icon" viewBox="0 0 24 24" fill="none">
                          <path d="M7 11V8a5 5 0 0 1 10 0v3" />
                          <path
                            d="M6.5 11h11A2.5 2.5 0 0 1 20 13.5v5A2.5 2.5 0 0 1 17.5 21h-11A2.5 2.5 0 0 1 4 18.5v-5A2.5 2.5 0 0 1 6.5 11Z"
                          />
                        </svg>
                      </span>
                      <input
                        v-model="loginPassword"
                        :type="revealLoginPassword ? 'text' : 'password'"
                        autocomplete="current-password"
                        aria-label="密码"
                      />
                      <button
                        class="password-toggle"
                        type="button"
                        tabindex="-1"
                        aria-label="按住显示密码"
                        @pointerdown.prevent="startRevealLogin($event)"
                        @pointerup="endRevealLogin"
                        @pointercancel="endRevealLogin"
                        @pointerleave="endRevealLogin"
                        @keydown.space.prevent="revealLoginPassword = true"
                        @keyup.space="revealLoginPassword = false"
                      >
                        <svg
                          v-if="revealLoginPassword"
                          class="field-icon"
                          viewBox="0 0 24 24"
                          fill="none"
                        >
                          <path d="M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7S2 12 2 12Z" />
                          <circle cx="12" cy="12" r="3" />
                        </svg>
                        <svg v-else class="field-icon" viewBox="0 0 24 24" fill="none">
                          <path d="M3 3l18 18" />
                          <path
                            d="M10.6 10.6a3 3 0 0 0 4.2 4.2M9.8 5.1A10.5 10.5 0 0 1 12 5c6.5 0 10 7 10 7a17.6 17.6 0 0 1-3.2 4.2M6.1 6.1A17.4 17.4 0 0 0 2 12s3.5 7 10 7a10.8 10.8 0 0 0 3.1-.5"
                          />
                        </svg>
                      </button>
                    </div>
                  </label>

                  <p v-if="loginError" class="error">{{ loginError }}</p>
                </div>
              </form>
            </div>

            <div ref="registerEl" class="mode" :class="{ active: displayMode === 'register' }">
              <form id="register-form" class="form form--register" @submit.prevent="onRegister">
                <div class="fields-stack">
                  <label class="field">
                    <span class="sr-only">用户名</span>
                    <div class="input-row">
                      <span class="input-icon" aria-hidden="true">
                        <svg class="field-icon" viewBox="0 0 24 24" fill="none">
                          <path d="M20 21a8 8 0 0 0-16 0" />
                          <circle cx="12" cy="8" r="4" />
                        </svg>
                      </span>
                      <input
                        v-model.trim="regUsername"
                        autocomplete="username"
                        aria-label="用户名"
                      />
                    </div>
                  </label>

                  <label class="field">
                    <span class="sr-only">邮箱</span>
                    <div class="input-row">
                      <span class="input-icon" aria-hidden="true">
                        <svg class="field-icon" viewBox="0 0 24 24" fill="none">
                          <path d="M4 6.5h16v11H4v-11Z" />
                          <path d="m4 7 8 6 8-6" />
                        </svg>
                      </span>
                      <input
                        v-model.trim="regEmail"
                        type="email"
                        autocomplete="email"
                        aria-label="邮箱"
                      />
                    </div>
                  </label>

                  <label
                    ref="registerPasswordFieldEl"
                    class="field staged-field"
                    :class="{ revealed: registerFieldStage >= 1 }"
                    :aria-hidden="registerFieldStage < 1"
                  >
                    <span class="sr-only">密码</span>
                    <div class="input-row">
                      <span class="input-icon" aria-hidden="true">
                        <svg class="field-icon" viewBox="0 0 24 24" fill="none">
                          <path d="M7 11V8a5 5 0 0 1 10 0v3" />
                          <path
                            d="M6.5 11h11A2.5 2.5 0 0 1 20 13.5v5A2.5 2.5 0 0 1 17.5 21h-11A2.5 2.5 0 0 1 4 18.5v-5A2.5 2.5 0 0 1 6.5 11Z"
                          />
                        </svg>
                      </span>
                      <input
                        v-model="regPassword"
                        :type="revealRegPassword ? 'text' : 'password'"
                        autocomplete="new-password"
                        aria-label="密码"
                      />
                      <button
                        class="password-toggle"
                        type="button"
                        tabindex="-1"
                        aria-label="按住显示密码"
                        @pointerdown.prevent="startRevealReg($event)"
                        @pointerup="endRevealReg"
                        @pointercancel="endRevealReg"
                        @pointerleave="endRevealReg"
                        @keydown.space.prevent="revealRegPassword = true"
                        @keyup.space="revealRegPassword = false"
                      >
                        <svg
                          v-if="revealRegPassword"
                          class="field-icon"
                          viewBox="0 0 24 24"
                          fill="none"
                        >
                          <path d="M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7S2 12 2 12Z" />
                          <circle cx="12" cy="12" r="3" />
                        </svg>
                        <svg v-else class="field-icon" viewBox="0 0 24 24" fill="none">
                          <path d="M3 3l18 18" />
                          <path
                            d="M10.6 10.6a3 3 0 0 0 4.2 4.2M9.8 5.1A10.5 10.5 0 0 1 12 5c6.5 0 10 7 10 7a17.6 17.6 0 0 1-3.2 4.2M6.1 6.1A17.4 17.4 0 0 0 2 12s3.5 7 10 7a10.8 10.8 0 0 0 3.1-.5"
                          />
                        </svg>
                      </button>
                    </div>
                  </label>

                  <label
                    ref="registerConfirmFieldEl"
                    class="field staged-field"
                    :class="{ revealed: registerFieldStage >= 2 }"
                    :aria-hidden="registerFieldStage < 2"
                  >
                    <span class="sr-only">确认密码</span>
                    <div class="input-row">
                      <span class="input-icon" aria-hidden="true">
                        <svg class="field-icon" viewBox="0 0 24 24" fill="none">
                          <path d="M7 11V8a5 5 0 0 1 10 0v3" />
                          <path
                            d="M6.5 11h11A2.5 2.5 0 0 1 20 13.5v5A2.5 2.5 0 0 1 17.5 21h-11A2.5 2.5 0 0 1 4 18.5v-5A2.5 2.5 0 0 1 6.5 11Z"
                          />
                          <path d="m9 16 2 2 4-4" />
                        </svg>
                      </span>
                      <input
                        v-model="regConfirmPassword"
                        :type="revealRegConfirmPassword ? 'text' : 'password'"
                        autocomplete="new-password"
                        aria-label="确认密码"
                      />
                      <button
                        class="password-toggle"
                        type="button"
                        tabindex="-1"
                        aria-label="按住显示密码"
                        @pointerdown.prevent="startRevealRegConfirm($event)"
                        @pointerup="endRevealRegConfirm"
                        @pointercancel="endRevealRegConfirm"
                        @pointerleave="endRevealRegConfirm"
                        @keydown.space.prevent="revealRegConfirmPassword = true"
                        @keyup.space="revealRegConfirmPassword = false"
                      >
                        <svg
                          v-if="revealRegConfirmPassword"
                          class="field-icon"
                          viewBox="0 0 24 24"
                          fill="none"
                        >
                          <path d="M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7S2 12 2 12Z" />
                          <circle cx="12" cy="12" r="3" />
                        </svg>
                        <svg v-else class="field-icon" viewBox="0 0 24 24" fill="none">
                          <path d="M3 3l18 18" />
                          <path
                            d="M10.6 10.6a3 3 0 0 0 4.2 4.2M9.8 5.1A10.5 10.5 0 0 1 12 5c6.5 0 10 7 10 7a17.6 17.6 0 0 1-3.2 4.2M6.1 6.1A17.4 17.4 0 0 0 2 12s3.5 7 10 7a10.8 10.8 0 0 0 3.1-.5"
                          />
                        </svg>
                      </button>
                    </div>
                  </label>

                  <p v-if="regError" class="error">{{ regError }}</p>
                </div>
              </form>
            </div>
          </div>
          <div ref="actionsOverlayEl" class="actions-overlay">
            <button
              class="primary action-primary"
              type="submit"
              :form="activeFormId"
              :disabled="activeLoading || isSwitching"
            >
              {{ activePrimaryLabel }}
            </button>
            <button
              class="switch-btn switch-btn-left"
              type="button"
              :disabled="isSwitching"
              @click="goLogin"
            >
              去登录
            </button>
            <button
              class="switch-btn switch-btn-right"
              type="button"
              :disabled="isSwitching"
              @click="goRegister"
            >
              去注册
            </button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { postJson } from '@/lib/api'
import { clearAuthCache, isAuthenticated, setUserProfile } from '@/lib/auth'
import { useTheme } from '@/composables/useTheme'

const props = defineProps<{ mode: 'login' | 'register' }>()

const router = useRouter()
const route = useRoute()

const { theme, toggleTheme, themeAriaLabel } = useTheme()

const HEIGHT_TRANSITION_DURATION_MS = 250
const BUTTON_MERGE_MS = 90
const BUTTON_SPLIT_MS = HEIGHT_TRANSITION_DURATION_MS - BUTTON_MERGE_MS
const ACTION_HEIGHT_PX = 46
const CARD_VERTICAL_PADDING_PX = 44
const CARD_HEAD_GAP_PX = 18
const FORM_ACTION_GAP_PX = 26
const FIELD_GAP_PX = 26
const LOGIN_HIDE_CONFIRM_DELAY_MS = 0
const LOGIN_HIDE_PASSWORD_DELAY_MS = 36
const REGISTER_SHOW_PASSWORD_DELAY_MS = 28
const REGISTER_SHOW_CONFIRM_DELAY_MS = 92

const displayMode = ref<'login' | 'register'>(props.mode)
const heightDurMs = ref(HEIGHT_TRANSITION_DURATION_MS)
const isSwitching = ref(false)
const transitionPhase = ref<'idle' | 'to-register' | 'to-login'>('idle')
const registerFieldStage = ref(props.mode === 'register' ? 2 : 0)

const authCardEl = ref<HTMLElement | null>(null)
const contentEl = ref<HTMLElement | null>(null)
const cardHeadEl = ref<HTMLElement | null>(null)
const actionsOverlayEl = ref<HTMLElement | null>(null)
const loginEl = ref<HTMLElement | null>(null)
const registerEl = ref<HTMLElement | null>(null)
const registerPasswordFieldEl = ref<HTMLElement | null>(null)
const registerConfirmFieldEl = ref<HTMLElement | null>(null)
const loginHeight = ref(540)
const registerHeight = ref(620)
const cardHeightPx = ref(540)

let resizeObserver: ResizeObserver | null = null
let switchTimers: number[] = []

const cardClass = computed(() => ({
  'is-switching': isSwitching.value,
  'is-login-layout': !isSwitching.value && displayMode.value === 'login',
  'is-register-layout': !isSwitching.value && displayMode.value === 'register',
  'is-to-register': transitionPhase.value === 'to-register',
  'is-to-login': transitionPhase.value === 'to-login',
}))

const activeFormId = computed(() =>
  displayMode.value === 'login' ? 'login-form' : 'register-form',
)
const activeLoading = computed(() =>
  displayMode.value === 'login' ? loginLoading.value : regLoading.value,
)
const activePrimaryLabel = computed(() => {
  if (displayMode.value === 'login') return loginLoading.value ? '登录中...' : '登录'
  return regLoading.value ? '注册中...' : '注册'
})

const clearSwitchTimers = () => {
  switchTimers.forEach((timer) => window.clearTimeout(timer))
  switchTimers = []
}

const setSwitchTimer = (callback: () => void, delayMs: number) => {
  const timer = window.setTimeout(callback, delayMs)
  switchTimers.push(timer)
}

const getModeContentHeight = (modeEl: HTMLElement | null) => {
  if (!modeEl) return 0
  const formEl = modeEl.querySelector<HTMLElement>('.form')
  return formEl?.offsetHeight ?? modeEl.offsetHeight
}

const getCardChromeHeight = () => {
  const headHeight = cardHeadEl.value?.offsetHeight ?? 0
  return CARD_VERTICAL_PADDING_PX + headHeight + CARD_HEAD_GAP_PX
}

const getRegisterVisibleFieldHeights = () => {
  const registerFields = registerEl.value?.querySelectorAll<HTMLElement>(
    '.fields-stack > .field:not(.staged-field)',
  )
  const errorEl = registerEl.value?.querySelector<HTMLElement>('.fields-stack > .error')
  const baseHeights = Array.from(registerFields ?? []).reduce((sum, el) => sum + el.offsetHeight, 0)
  const passwordHeight = Math.max(
    registerPasswordFieldEl.value?.scrollHeight ?? 0,
    registerPasswordFieldEl.value?.offsetHeight ?? 0,
  )
  const confirmHeight = Math.max(
    registerConfirmFieldEl.value?.scrollHeight ?? 0,
    registerConfirmFieldEl.value?.offsetHeight ?? 0,
  )
  const errorHeight = errorEl?.offsetHeight ?? 0
  return { baseHeights, passwordHeight, confirmHeight, errorHeight }
}

const getRegisterStageContentHeight = (stage: 0 | 1 | 2) => {
  const { baseHeights, passwordHeight, confirmHeight, errorHeight } =
    getRegisterVisibleFieldHeights()

  let itemCount = 2
  let fieldsHeight = baseHeights

  if (stage >= 1 && passwordHeight > 0) {
    fieldsHeight += passwordHeight
    itemCount += 1
  }
  if (stage >= 2 && confirmHeight > 0) {
    fieldsHeight += confirmHeight
    itemCount += 1
  }
  if (errorHeight > 0) {
    fieldsHeight += errorHeight
    itemCount += 1
  }

  const gapsHeight = Math.max(0, itemCount - 1) * FIELD_GAP_PX
  return fieldsHeight + gapsHeight + FORM_ACTION_GAP_PX + ACTION_HEIGHT_PX
}

const measureHeights = () => {
  const chromeHeight = getCardChromeHeight()
  const lh = getModeContentHeight(loginEl.value)
  const rh = Math.max(getModeContentHeight(registerEl.value), getRegisterStageContentHeight(2))

  if (lh > 0) loginHeight.value = chromeHeight + lh
  if (rh > 0) registerHeight.value = chromeHeight + rh

  if (!isSwitching.value) {
    cardHeightPx.value = displayMode.value === 'login' ? loginHeight.value : registerHeight.value
  }
}

const getRegisterFieldRevealDelay = (
  fieldIndex: 1 | 2,
  startHeight: number,
  targetHeight: number,
) => {
  const fixedDelay =
    fieldIndex === 1 ? REGISTER_SHOW_PASSWORD_DELAY_MS : REGISTER_SHOW_CONFIRM_DELAY_MS
  const totalGrowth = Math.max(1, targetHeight - startHeight)
  const perFieldGrowth = Math.max(1, (registerHeight.value - loginHeight.value) / 2)
  const rawDelay = Math.round(
    (perFieldGrowth * fieldIndex * HEIGHT_TRANSITION_DURATION_MS) / totalGrowth,
  )
  return Math.min(fixedDelay, Math.max(0, rawDelay))
}

const cardStyle = computed(() => ({
  height: `${cardHeightPx.value}px`,
  '--height-dur': `${heightDurMs.value}ms`,
  '--merge-dur': `${BUTTON_MERGE_MS}ms`,
  '--split-dur': `${BUTTON_SPLIT_MS}ms`,
}))

watch(
  () => props.mode,
  async () => {
    loginError.value = ''
    regError.value = ''
    if (props.mode === displayMode.value) {
      await nextTick()
      measureHeights()
      return
    }

    clearSwitchTimers()
    heightDurMs.value = HEIGHT_TRANSITION_DURATION_MS
    await nextTick()
    measureHeights()

    const startHeight = cardHeightPx.value
    const targetHeight = props.mode === 'login' ? loginHeight.value : registerHeight.value

    isSwitching.value = true
    transitionPhase.value = props.mode === 'register' ? 'to-register' : 'to-login'
    registerFieldStage.value = props.mode === 'register' ? 0 : 2

    await new Promise<void>((resolve) => requestAnimationFrame(() => resolve()))
    cardHeightPx.value = targetHeight

    const swapDelay = BUTTON_MERGE_MS

    if (props.mode === 'register') {
      setSwitchTimer(() => {
        displayMode.value = 'register'
      }, swapDelay)
    } else {
      setSwitchTimer(() => {
        registerFieldStage.value = 1
      }, LOGIN_HIDE_CONFIRM_DELAY_MS)
      setSwitchTimer(() => {
        registerFieldStage.value = 0
      }, LOGIN_HIDE_PASSWORD_DELAY_MS)
    }

    if (props.mode === 'register') {
      setSwitchTimer(
        () => {
          registerFieldStage.value = 1
        },
        getRegisterFieldRevealDelay(1, startHeight, targetHeight),
      )
      setSwitchTimer(
        () => {
          registerFieldStage.value = 2
        },
        getRegisterFieldRevealDelay(2, startHeight, targetHeight),
      )
    }

    setSwitchTimer(() => {
      displayMode.value = props.mode
      registerFieldStage.value = props.mode === 'register' ? 2 : 0
      isSwitching.value = false
      transitionPhase.value = 'idle'
      measureHeights()
    }, HEIGHT_TRANSITION_DURATION_MS)
  },
)

const goRegister = () => router.push({ path: '/register', query: route.query })
const goLogin = () => router.push({ path: '/login', query: route.query })

// ---------------------------------------------------------------------------
// Password reveal (press-and-hold)
// ---------------------------------------------------------------------------
const revealLoginPassword = ref(false)
const revealRegPassword = ref(false)
const revealRegConfirmPassword = ref(false)

const stopAllReveal = () => {
  revealLoginPassword.value = false
  revealRegPassword.value = false
  revealRegConfirmPassword.value = false
}

const startReveal = (target: { value: boolean }, e: PointerEvent) => {
  target.value = true
  const el = e.currentTarget as HTMLElement | null
  el?.setPointerCapture?.(e.pointerId)
}

const endReveal = (target: { value: boolean }) => {
  target.value = false
}

const startRevealLogin = (e: PointerEvent) => startReveal(revealLoginPassword, e)
const endRevealLogin = () => endReveal(revealLoginPassword)
const startRevealReg = (e: PointerEvent) => startReveal(revealRegPassword, e)
const endRevealReg = () => endReveal(revealRegPassword)
const startRevealRegConfirm = (e: PointerEvent) => startReveal(revealRegConfirmPassword, e)
const endRevealRegConfirm = () => endReveal(revealRegConfirmPassword)

// ---------------------------------------------------------------------------
// Login
// ---------------------------------------------------------------------------
const sharedUsername = ref('')
const sharedPassword = ref('')

const loginUsername = sharedUsername
const loginPassword = sharedPassword
const loginLoading = ref(false)
const loginError = ref('')

type LoginResponse = { success: true; uid: number; token: string; username: string; email: string; created_at: number }

const onLogin = async () => {
  loginError.value = ''
  loginLoading.value = true
  try {
    const res = await postJson<LoginResponse>(
      '/api/login',
      { username: loginUsername.value, password: loginPassword.value },
      { credentials: 'include' },
    )
    if (!res.ok) {
      loginError.value = res.error
      return
    }
    clearAuthCache()
    setUserProfile(res.data)
    const redirect = typeof route.query.redirect === 'string' ? route.query.redirect : '/main'
    window.location.href = redirect
  } finally {
    loginLoading.value = false
  }
}

// ---------------------------------------------------------------------------
// Register
// ---------------------------------------------------------------------------
const regUsername = sharedUsername
const regEmail = ref('')
const regPassword = sharedPassword
const regConfirmPassword = ref('')
const regLoading = ref(false)
const regError = ref('')

watch([loginError, regError], async () => {
  await nextTick()
  measureHeights()
})

type RegisterResponse = { success: true; uid: number; token: string; username: string; email: string; created_at: number }

const onRegister = async () => {
  regError.value = ''
  if (!regUsername.value || !regEmail.value || !regPassword.value) {
    regError.value = '请填写用户名、邮箱和密码'
    return
  }
  if (regPassword.value !== regConfirmPassword.value) {
    regError.value = '两次输入的密码不一致'
    return
  }

  regLoading.value = true
  try {
    const res = await postJson<RegisterResponse>(
      '/api/register',
      { username: regUsername.value, email: regEmail.value, password: regPassword.value },
      { credentials: 'include' },
    )
    if (!res.ok) {
      regError.value = res.error
      return
    }
    clearAuthCache()
    setUserProfile(res.data)
    window.location.href = '/main'
  } finally {
    regLoading.value = false
  }
}

onMounted(async () => {
  await nextTick()
  measureHeights()
  cardHeightPx.value = displayMode.value === 'login' ? loginHeight.value : registerHeight.value

  if (typeof ResizeObserver === 'undefined') return
  resizeObserver = new ResizeObserver(() => measureHeights())
  if (loginEl.value) resizeObserver.observe(loginEl.value)
  if (registerEl.value) resizeObserver.observe(registerEl.value)

  if (await isAuthenticated()) {
    const redirect = typeof route.query.redirect === 'string' ? route.query.redirect : '/main'
    window.location.href = redirect
  }
})

onBeforeUnmount(() => {
  resizeObserver?.disconnect()
  resizeObserver = null
})

onMounted(() => {
  window.addEventListener('pointerup', stopAllReveal)
  window.addEventListener('pointercancel', stopAllReveal)
  window.addEventListener('blur', stopAllReveal)
})

onBeforeUnmount(() => {
  window.removeEventListener('pointerup', stopAllReveal)
  window.removeEventListener('pointercancel', stopAllReveal)
  window.removeEventListener('blur', stopAllReveal)
  clearSwitchTimers()
})
</script>

<style scoped>
.auth-page {
  min-height: 100vh;
  display: grid;
  place-items: center;
  padding: 24px;
  background:
    radial-gradient(900px 520px at 10% -10%, rgba(0, 122, 204, 0.22), transparent 60%),
    radial-gradient(760px 520px at 92% -10%, rgba(0, 178, 148, 0.18), transparent 58%),
    linear-gradient(180deg, var(--bg), var(--bg-2));
  font-family:
    ui-sans-serif,
    system-ui,
    -apple-system,
    'Segoe UI',
    Roboto,
    Arial,
    'PingFang SC',
    'Microsoft YaHei',
    sans-serif;
  color: var(--text);
}

.flip-wrap {
  width: min(480px, 100%);
}

.auth-card {
  position: relative;
  width: 100%;
  border: 1px solid var(--border);
  border-radius: 18px;
  background: color-mix(in srgb, var(--surface) 92%, transparent);
  box-shadow: var(--shadow-md);
  overflow: hidden;
  transition-property: height;
  transition-duration: var(--height-dur, 250ms);
  transition-timing-function: cubic-bezier(0.2, 0.85, 0.15, 1);
  will-change: height;
}

.card-content {
  position: relative;
  display: flex;
  flex-direction: column;
  box-sizing: border-box;
  height: 100%;
  padding: 22px;
}

.mode-slot {
  position: relative;
  flex: 1;
  min-height: 0;
  z-index: 1;
}

.mode {
  position: absolute;
  left: 0;
  right: 0;
  top: 0;
  opacity: 0;
  visibility: hidden;
  pointer-events: none;
}

.mode.active {
  position: relative;
  left: auto;
  right: auto;
  top: auto;
  height: 100%;
  opacity: 1;
  visibility: visible;
  pointer-events: auto;
}

.card-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 18px;
}

h1 {
  margin: 0;
  font-size: 1.5rem;
  letter-spacing: 0.2px;
  background: linear-gradient(90deg, var(--brand), var(--brand-2));
  -webkit-background-clip: text;
  background-clip: text;
  -webkit-text-fill-color: transparent;
}

.theme-switch {
  border: none;
  border-radius: 999px;
  width: 54px;
  height: 34px;
  padding: 0;
  background: color-mix(in srgb, var(--surface-2) 90%, var(--surface-3) 10%);
  color: var(--text);
  cursor: pointer;
  box-shadow: var(--shadow-sm);
  display: inline-flex;
  align-items: center;
  justify-content: flex-start;
  position: relative;
  transition:
    background 0.18s ease,
    box-shadow 0.18s ease,
    filter 0.18s ease;
}

.theme-switch:hover {
  filter: saturate(1.05);
}

.theme-switch.is-dark {
  background: linear-gradient(
    135deg,
    color-mix(in srgb, var(--purple) 58%, var(--surface-2)),
    color-mix(in srgb, var(--brand) 42%, var(--surface-2))
  );
}

.theme-switch:focus-visible {
  outline: 2px solid color-mix(in srgb, var(--brand) 48%, transparent);
  outline-offset: 3px;
}

.thumb {
  width: 28px;
  height: 28px;
  border-radius: 999px;
  margin-left: 3px;
  background: color-mix(in srgb, var(--surface) 88%, transparent);
  box-shadow: 0 10px 22px rgba(2, 6, 23, 0.22);
  display: inline-flex;
  align-items: center;
  justify-content: center;
  transform: translateX(0px);
  transition:
    transform 220ms cubic-bezier(0.2, 0.85, 0.15, 1),
    background 0.18s ease;
}

.theme-switch.is-dark .thumb {
  transform: translateX(20px);
}

.theme-switch.is-dark .thumb {
  background: color-mix(in srgb, rgba(255, 255, 255, 0.16) 32%, var(--surface));
}

.theme-icon {
  width: 18px;
  height: 18px;
  stroke: currentColor;
  stroke-width: 2;
  stroke-linecap: round;
  stroke-linejoin: round;
}

.sr-only {
  position: absolute;
  width: 1px;
  height: 1px;
  padding: 0;
  margin: -1px;
  overflow: hidden;
  clip: rect(0, 0, 0, 0);
  white-space: nowrap;
  border: 0;
}

.form {
  --action-h: 46px;
  position: relative;
  box-sizing: border-box;
  min-height: auto;
  padding-bottom: calc(var(--action-h) + 26px);
}

.fields-stack {
  display: flex;
  flex-direction: column;
}

.fields-stack > .field + .field,
.fields-stack > .error {
  margin-top: 26px;
}

.field {
  display: grid;
  gap: 10px;
  font-weight: 800;
  color: var(--muted);
}

.staged-field {
  max-height: 0;
  overflow: hidden;
  margin-top: 0 !important;
  opacity: 0;
  visibility: hidden;
  pointer-events: none;
  transform: translateY(10px);
  transition:
    max-height 0.2s ease,
    margin-top 0.2s ease,
    opacity 0.12s ease,
    transform 0.12s ease,
    visibility 0s linear 0.12s;
}

.staged-field.revealed {
  max-height: 120px;
  margin-top: 26px !important;
  opacity: 1;
  visibility: visible;
  pointer-events: auto;
  transform: translateY(0);
  transition-delay: 0s;
}

.auth-card.is-to-login .staged-field {
  transition:
    max-height 0.08s ease,
    margin-top 0.08s ease,
    opacity 0.06s ease,
    transform 0.06s ease,
    visibility 0s linear 0.08s;
}

.input-row {
  display: flex;
  align-items: center;
  height: 48px;
  border-radius: 14px;
  border: 1px solid var(--border);
  background: color-mix(in srgb, var(--surface-2) 78%, transparent);
  overflow: hidden;
}

:global(:root[data-theme='dark']) .input-row {
  background: color-mix(in srgb, var(--surface-3) 86%, rgba(255, 255, 255, 0.06));
}

.input-icon {
  width: 46px;
  height: 48px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  color: color-mix(in srgb, var(--muted) 78%, var(--text));
  flex: 0 0 auto;
}

.password-toggle {
  width: 46px;
  height: 48px;
  border: none;
  padding: 0;
  background: transparent;
  color: color-mix(in srgb, var(--muted) 78%, var(--text));
  display: inline-flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  flex: 0 0 auto;
  -webkit-tap-highlight-color: transparent;
}

.password-toggle:hover {
  filter: saturate(1.08);
}

.password-toggle:focus-visible {
  outline: 2px solid color-mix(in srgb, var(--brand) 48%, transparent);
  outline-offset: 2px;
  border-radius: 12px;
}

.field-icon {
  width: 18px;
  height: 18px;
  stroke: currentColor;
  stroke-width: 2;
  stroke-linecap: round;
  stroke-linejoin: round;
}

input {
  flex: 1;
  height: 48px;
  padding: 0 14px 0 0;
  border: none;
  background: transparent;
  color: var(--text);
  outline: none;
}

input:focus {
  box-shadow: none;
}

.field:focus-within .input-row {
  border-color: color-mix(in srgb, var(--brand) 55%, transparent);
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--brand) 18%, transparent);
}

.error {
  margin: 0;
  color: var(--danger);
  font-weight: 800;
}

.primary {
  border: 1px solid rgba(255, 255, 255, 0.12);
  border-radius: 999px;
  padding: 12px 18px;
  font-weight: 850;
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

.primary,
.switch-btn {
  height: var(--action-h);
  display: inline-flex;
  align-items: center;
  justify-content: center;
  transform-origin: center;
}

.actions-overlay {
  --switch-w: 92px;
  --actions-gap: 12px;
  --action-h: 46px;
  position: absolute;
  left: 22px;
  right: 22px;
  bottom: 22px;
  height: var(--action-h);
  z-index: 5;
  pointer-events: none;
}

.action-primary,
.switch-btn-left,
.switch-btn-right {
  position: absolute;
  top: 0;
  pointer-events: auto;
}

.action-primary {
  left: 0;
  width: calc(100% - var(--switch-w) - var(--actions-gap));
  min-width: 150px;
  transition:
    left var(--height-dur, 250ms) cubic-bezier(0.2, 0.85, 0.15, 1),
    filter 0.15s ease,
    opacity 0.16s ease;
}

.switch-btn-left {
  left: 0;
}

.switch-btn-right {
  right: 0;
}

.switch-btn {
  border: none;
  border-radius: 999px;
  padding: 0 14px;
  width: var(--switch-w);
  max-width: var(--switch-w);
  flex: 0 0 var(--switch-w);
  overflow: hidden;
  white-space: nowrap;
  background: linear-gradient(
    135deg,
    color-mix(in srgb, var(--purple) 78%, #000),
    color-mix(in srgb, var(--brand) 72%, #000)
  );
  color: rgba(255, 255, 255, 0.94);
  font-size: 0.84rem;
  font-weight: 800;
  letter-spacing: 0.2px;
  cursor: pointer;
  box-shadow: 0 10px 22px rgba(2, 6, 23, 0.22);
  transition:
    opacity var(--merge-dur, 90ms) ease,
    filter 0.15s ease,
    box-shadow 0.15s ease;
  opacity: 0;
  pointer-events: none;
}

.switch-btn:hover {
  filter: saturate(1.12);
  box-shadow: 0 14px 30px rgba(2, 6, 23, 0.26);
}

.auth-card.is-login-layout .action-primary {
  left: 0;
}

.auth-card.is-register-layout .action-primary {
  left: calc(var(--switch-w) + var(--actions-gap));
}

.auth-card.is-login-layout .switch-btn-right,
.auth-card.is-register-layout .switch-btn-left {
  opacity: 1;
  pointer-events: auto;
}

.auth-card.is-switching .actions-overlay {
  pointer-events: none;
}

.auth-card.is-switching .actions-overlay > * {
  pointer-events: none;
}

.auth-card.is-to-register .action-primary {
  left: calc(var(--switch-w) + var(--actions-gap));
}

.auth-card.is-to-login .action-primary {
  left: 0;
}

.auth-card.is-to-register .switch-btn-right,
.auth-card.is-to-login .switch-btn-left {
  opacity: 0;
}

.auth-card.is-to-register .switch-btn-left,
.auth-card.is-to-login .switch-btn-right {
  opacity: 1;
  transition:
    opacity var(--split-dur, 160ms) ease calc(var(--height-dur, 250ms) - var(--split-dur, 160ms)),
    filter 0.15s ease,
    box-shadow 0.15s ease;
}

.auth-card.is-to-register .switch-btn-right,
.auth-card.is-to-login .switch-btn-left {
  transition:
    opacity var(--merge-dur, 90ms) ease,
    filter 0.15s ease,
    box-shadow 0.15s ease;
}

@media (prefers-reduced-motion: reduce) {
  .auth-card {
    transition: none;
  }

  .staged-field,
  .switch-btn,
  .action-primary {
    transition: none;
  }
}
</style>
