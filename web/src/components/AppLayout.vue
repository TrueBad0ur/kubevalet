<template>
  <div class="layout">
    <!-- Sidebar -->
    <aside class="sidebar">
      <RouterLink to="/" class="sidebar-brand">
        <img src="/favicon.svg" width="22" height="31" style="flex-shrink:0" alt="kubevalet"/>
        kubevalet
      </RouterLink>

      <nav class="sidebar-nav">
        <RouterLink to="/" :class="{ active: route.path === '/' || route.path.startsWith('/users') }">
          <svg viewBox="0 0 20 20" fill="currentColor"><path d="M9 6a3 3 0 11-6 0 3 3 0 016 0zM17 6a3 3 0 11-6 0 3 3 0 016 0zM12.93 17c.046-.327.07-.66.07-1a6.97 6.97 0 00-1.5-4.33A5 5 0 0119 16v1h-6.07zM6 11a5 5 0 015 5v1H1v-1a5 5 0 015-5z"/></svg>
          Users
        </RouterLink>

        <RouterLink to="/groups" :class="{ active: route.path === '/groups' }">
          <svg viewBox="0 0 20 20" fill="currentColor"><path d="M13 6a3 3 0 11-6 0 3 3 0 016 0zM18 8a2 2 0 11-4 0 2 2 0 014 0zM14 15a4 4 0 00-8 0v3h8v-3zM6 8a2 2 0 11-4 0 2 2 0 014 0zM16 18v-3a5.972 5.972 0 00-.75-2.906A3.005 3.005 0 0119 15v3h-3zM4.75 12.094A5.973 5.973 0 004 15v3H1v-3a3 3 0 013.75-2.906z"/></svg>
          Groups
        </RouterLink>

        <RouterLink to="/graph" :class="{ active: route.path === '/graph' }">
          <svg viewBox="0 0 20 20" fill="currentColor"><path fill-rule="evenodd" d="M5 2a1 1 0 00-1 1v1H3a1 1 0 000 2h1v1a1 1 0 002 0V6h1a1 1 0 000-2H6V3a1 1 0 00-1-1zm9 0a1 1 0 00-1 1v1h-1a1 1 0 000 2h1v1a1 1 0 002 0V6h1a1 1 0 000-2h-1V3a1 1 0 00-1-1zM5 12a1 1 0 00-1 1v1H3a1 1 0 000 2h1v1a1 1 0 002 0v-1h1a1 1 0 000-2H6v-1a1 1 0 00-1-1zm7 1a1 1 0 012 0v1h1a1 1 0 010 2h-1v1a1 1 0 01-2 0v-1h-1a1 1 0 010-2h1v-1z" clip-rule="evenodd"/></svg>
          Graph
        </RouterLink>

        <div class="sidebar-divider"></div>

        <RouterLink to="/settings" :class="{ active: route.path === '/settings' }">
          <svg viewBox="0 0 20 20" fill="currentColor"><path fill-rule="evenodd" d="M11.49 3.17c-.38-1.56-2.6-1.56-2.98 0a1.532 1.532 0 01-2.286.948c-1.372-.836-2.942.734-2.106 2.106.54.886.061 2.042-.947 2.287-1.561.379-1.561 2.6 0 2.978a1.532 1.532 0 01.947 2.287c-.836 1.372.734 2.942 2.106 2.106a1.532 1.532 0 012.287.947c.379 1.561 2.6 1.561 2.978 0a1.533 1.533 0 012.287-.947c1.372.836 2.942-.734 2.106-2.106a1.533 1.533 0 01.947-2.287c1.561-.379 1.561-2.6 0-2.978a1.532 1.532 0 01-.947-2.287c.836-1.372-.734-2.942-2.106-2.106a1.532 1.532 0 01-2.287-.947zM10 13a3 3 0 100-6 3 3 0 000 6z" clip-rule="evenodd"/></svg>
          Settings
        </RouterLink>

        <RouterLink to="/integrations" :class="{ active: route.path === '/integrations' }">
          <svg viewBox="0 0 20 20" fill="currentColor"><path fill-rule="evenodd" d="M12.316 3.051a1 1 0 01.633 1.265l-4 12a1 1 0 11-1.898-.632l4-12a1 1 0 011.265-.633zM5.707 6.293a1 1 0 010 1.414L3.414 10l2.293 2.293a1 1 0 11-1.414 1.414l-3-3a1 1 0 010-1.414l3-3a1 1 0 011.414 0zm8.586 0a1 1 0 011.414 0l3 3a1 1 0 010 1.414l-3 3a1 1 0 11-1.414-1.414L16.586 10l-2.293-2.293a1 1 0 010-1.414z" clip-rule="evenodd"/></svg>
          Integrations
        </RouterLink>
      </nav>

      <div class="sidebar-footer">
        <span>{{ username ?? '—' }}</span>
        <button class="btn btn-ghost btn-sm" @click="doLogout">Sign out</button>
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
