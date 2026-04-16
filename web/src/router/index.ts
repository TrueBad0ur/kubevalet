import { createRouter, createWebHistory, type RouteLocationRaw } from 'vue-router'
import LoginView        from '@/views/LoginView.vue'
import UsersView        from '@/views/UsersView.vue'
import CreateUserView   from '@/views/CreateUserView.vue'
import SettingsView     from '@/views/SettingsView.vue'
import IntegrationsView from '@/views/IntegrationsView.vue'
import GroupsView       from '@/views/GroupsView.vue'
import GraphView        from '@/views/GraphView.vue'
import LocalUsersView   from '@/views/LocalUsersView.vue'

declare module 'vue-router' {
  interface RouteMeta {
    public?: boolean
    adminOnly?: boolean
  }
}

function roleFromToken(): string | null {
  const t = localStorage.getItem('token')
  if (!t) return null
  try {
    const seg = t.split('.')[1].replace(/-/g, '+').replace(/_/g, '/')
    const padded = seg + '='.repeat((4 - seg.length % 4) % 4)
    return JSON.parse(atob(padded))?.role ?? null
  } catch { return null }
}

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/login',        component: LoginView,        meta: { public: true } },
    { path: '/',             component: UsersView },
    { path: '/users/new',    component: CreateUserView,   meta: { adminOnly: true } },
    { path: '/groups',       component: GroupsView },
    { path: '/graph',        component: GraphView },
    { path: '/settings',     component: SettingsView },
    { path: '/local-users',  component: LocalUsersView,   meta: { adminOnly: true } },
    { path: '/integrations', component: IntegrationsView },
    { path: '/:pathMatch(.*)*', redirect: '/' },
  ],
})

router.beforeEach((to): RouteLocationRaw | void => {
  const token = localStorage.getItem('token')
  if (!to.meta.public && !token) return '/login'
  if (to.path === '/login' && token) return '/'
  if (to.meta.adminOnly && roleFromToken() !== 'admin') return '/'
})

export default router
