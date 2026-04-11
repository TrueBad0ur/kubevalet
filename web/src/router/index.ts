import { createRouter, createWebHistory, type RouteLocationRaw } from 'vue-router'
import LoginView        from '@/views/LoginView.vue'
import UsersView        from '@/views/UsersView.vue'
import CreateUserView   from '@/views/CreateUserView.vue'
import SettingsView     from '@/views/SettingsView.vue'
import IntegrationsView from '@/views/IntegrationsView.vue'
import GroupsView       from '@/views/GroupsView.vue'
import GraphView        from '@/views/GraphView.vue'

declare module 'vue-router' {
  interface RouteMeta {
    public?: boolean
  }
}

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/login',        component: LoginView,        meta: { public: true } },
    { path: '/',             component: UsersView },
    { path: '/users/new',    component: CreateUserView },
    { path: '/groups',       component: GroupsView },
    { path: '/graph',        component: GraphView },
    { path: '/settings',     component: SettingsView },
    { path: '/integrations', component: IntegrationsView },
    { path: '/:pathMatch(.*)*', redirect: '/' },
  ],
})

router.beforeEach((to): RouteLocationRaw | void => {
  const token = localStorage.getItem('token')
  if (!to.meta.public && !token) return '/login'
  if (to.path === '/login' && token) return '/'
})

export default router
