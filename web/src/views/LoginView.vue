<template>
  <div class="login-page">
    <div class="login-box">
      <div class="login-logo">kube<span>valet</span></div>

      <form @submit.prevent="submit">
        <div v-if="error" class="alert alert-error">{{ error }}</div>

        <div class="form-group">
          <label class="form-label">Username <span class="required">*</span></label>
          <input
            v-model="form.username"
            type="text"
            class="form-input"
            autocomplete="username"
            autofocus
            required
          />
        </div>

        <div class="form-group">
          <label class="form-label">Password <span class="required">*</span></label>
          <input
            v-model="form.password"
            type="password"
            class="form-input"
            autocomplete="current-password"
            required
          />
        </div>

        <button type="submit" class="btn btn-primary" style="display:block;margin:0 auto;min-width:120px" :disabled="loading">
          <span v-if="loading" class="spinner" />
          <span v-else>Sign in</span>
        </button>
      </form>
    </div>
  </div>
</template>

<script setup lang="ts">
import { reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { login } from '@/api/auth'
import { useAuth } from '@/composables/useAuth'

const router = useRouter()
const { setToken, fetchMe } = useAuth()

const form = reactive({ username: '', password: '' })
const error = ref('')
const loading = ref(false)

async function submit() {
  error.value = ''
  loading.value = true
  try {
    const token = await login(form.username, form.password)
    setToken(token)
    await fetchMe()
    router.push('/')
  } catch (e: any) {
    error.value = e.response?.data?.error ?? 'Login failed'
  } finally {
    loading.value = false
  }
}
</script>
