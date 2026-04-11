<template>
  <div class="layout">
    <!-- Sidebar -->
    <aside class="sidebar">
      <RouterLink to="/" class="sidebar-brand">
        <svg width="24" height="24" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
          <path d="M12 2L2 7l10 5 10-5-10-5zM2 17l10 5 10-5M2 12l10 5 10-5" stroke="#60a5fa" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
        </svg>
        kubevalet
      </RouterLink>

      <nav class="sidebar-nav">
        <RouterLink to="/" :class="{ active: route.path === '/' }">
          <svg viewBox="0 0 20 20" fill="currentColor"><path d="M9 6a3 3 0 11-6 0 3 3 0 016 0zM17 6a3 3 0 11-6 0 3 3 0 016 0zM12.93 17c.046-.327.07-.66.07-1a6.97 6.97 0 00-1.5-4.33A5 5 0 0119 16v1h-6.07zM6 11a5 5 0 015 5v1H1v-1a5 5 0 015-5z"/></svg>
          Users
        </RouterLink>
      </nav>

      <div class="sidebar-footer">
        <span>{{ username ?? '—' }}</span>
        <button class="btn btn-ghost btn-sm" style="width:100%" @click="doLogout">Sign out</button>
      </div>
    </aside>

    <!-- Content -->
    <div class="main">
      <header class="topbar">
        <span class="topbar-title">{{ title }}</span>
        <div class="topbar-right">
          <slot name="actions" />
        </div>
      </header>
      <main class="content">
        <slot />
      </main>
    </div>
  </div>
</template>

<script setup lang="ts">
import { RouterLink, useRoute, useRouter } from 'vue-router'
import { useAuth } from '@/composables/useAuth'

defineProps<{ title: string }>()

const route  = useRoute()
const router = useRouter()
const { username, logout } = useAuth()

function doLogout() {
  logout()
  router.push('/login')
}
</script>
