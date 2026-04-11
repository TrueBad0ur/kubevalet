import { ref, computed } from 'vue'
import { me } from '@/api/auth'

const token = ref<string | null>(localStorage.getItem('token'))
const username = ref<string | null>(null)

export function useAuth() {
  const isAuthenticated = computed(() => !!token.value)

  function setToken(t: string) {
    token.value = t
    localStorage.setItem('token', t)
  }

  function logout() {
    token.value = null
    username.value = null
    localStorage.removeItem('token')
  }

  async function fetchMe() {
    if (!token.value) return
    try {
      const data = await me()
      username.value = data.username
    } catch {
      logout()
    }
  }

  return { isAuthenticated, username, setToken, logout, fetchMe }
}
