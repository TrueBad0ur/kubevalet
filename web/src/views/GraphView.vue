<template>
  <AppLayout title="Graph">
    <div class="graph-layout">

      <!-- ── Left panel: user list ── -->
      <div class="graph-sidebar">
        <div class="graph-sidebar-toolbar">
          <select v-model="filterGroup" class="form-select" style="font-size:12px;padding:5px 8px">
            <option value="">All users</option>
            <option v-for="g in allGroups" :key="g" :value="g">{{ g }}</option>
          </select>
          <span class="text-muted" style="font-size:11px;margin-top:4px">{{ filteredUsers.length }} user{{ filteredUsers.length !== 1 ? 's' : '' }}</span>
        </div>

        <div v-if="loading" style="padding:32px;text-align:center"><span class="spinner" style="width:20px;height:20px"/></div>
        <div v-else-if="error" class="alert alert-error" style="margin:12px;font-size:12px">{{ error }}</div>
        <div v-else class="graph-user-list">
          <button
            v-for="u in filteredUsers" :key="u.name"
            class="graph-user-item"
            :class="{ active: selected?.name === u.name }"
            @click="selected = u"
          >
            <div style="display:flex;align-items:center;justify-content:space-between;gap:8px">
              <span class="font-mono" style="font-size:12.5px;font-weight:600;overflow:hidden;text-overflow:ellipsis;white-space:nowrap">{{ u.name }}</span>
              <span class="badge" :class="statusClass(u.status)" style="font-size:10px;flex-shrink:0">{{ u.status }}</span>
            </div>
            <div style="display:flex;flex-wrap:wrap;gap:3px;margin-top:4px">
              <span v-for="g in u.groups" :key="g" class="badge badge-info" style="font-size:10px">{{ g }}</span>
              <span v-if="!u.groups?.length" style="font-size:11px;color:var(--text-muted)">no groups</span>
            </div>
            <div style="margin-top:5px;font-size:11px;color:var(--text-muted)">
              <span v-if="u.clusterRole">cluster / {{ u.clusterRole }}</span>
              <span v-else-if="u.customRole">cluster / custom</span>
              <span v-else-if="u.namespaceBindings?.length">{{ u.namespaceBindings.length }} namespace{{ u.namespaceBindings.length !== 1 ? 's' : '' }}</span>
              <span v-else>no bindings</span>
            </div>
          </button>
          <div v-if="filteredUsers.length === 0" style="padding:24px;text-align:center;color:var(--text-muted);font-size:13px">
            No users match this filter.
          </div>
        </div>
      </div>

      <!-- ── Right panel: detail ── -->
      <div class="graph-detail">

        <!-- Empty state -->
        <div v-if="!selected" class="graph-empty-state">
          <svg width="36" height="36" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" style="color:var(--text-muted);margin-bottom:12px">
            <path stroke-linecap="round" stroke-linejoin="round" d="M15.75 6a3.75 3.75 0 11-7.5 0 3.75 3.75 0 017.5 0zM4.501 20.118a7.5 7.5 0 0114.998 0A17.933 17.933 0 0112 21.75c-2.676 0-5.216-.584-7.499-1.632z"/>
          </svg>
          <p style="margin:0;font-size:13px;color:var(--text-muted)">Select a user to view their permissions</p>
        </div>

        <!-- User detail -->
        <template v-else>
          <div class="graph-detail-header">
            <div style="display:flex;align-items:center;gap:10px;flex-wrap:wrap">
              <span class="font-mono" style="font-size:15px;font-weight:700">{{ selected.name }}</span>
              <span class="badge" :class="statusClass(selected.status)">{{ selected.status }}</span>
            </div>
            <div v-if="selected.groups?.length" style="display:flex;flex-wrap:wrap;gap:4px;margin-top:8px">
              <span v-for="g in selected.groups" :key="g" class="badge badge-info">{{ g }}</span>
            </div>
          </div>

          <!-- Cluster access -->
          <div v-if="selected.clusterRole || selected.customRole" class="graph-access-block">
            <div class="graph-access-title">
              <svg viewBox="0 0 20 20" fill="currentColor" width="13" height="13"><path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM4.332 8.027a6.012 6.012 0 011.912-2.706C6.512 5.73 6.974 6 7.5 6A1.5 1.5 0 019 7.5V8a2 2 0 004 0 2 2 0 011.523-1.943A5.977 5.977 0 0116 10c0 .34-.028.675-.083 1H15a2 2 0 00-2 2v2.197A5.973 5.973 0 0110 16v-2a2 2 0 00-2-2 2 2 0 01-2-2 2 2 0 00-1.668-1.973z" clip-rule="evenodd"/></svg>
              Cluster-wide
            </div>
            <div class="graph-access-body">
              <div v-if="selected.clusterRole" style="display:flex;align-items:center;gap:8px">
                <span class="text-muted text-sm">Role</span>
                <span class="badge" :class="roleClass(selected.clusterRole)">{{ selected.clusterRole }}</span>
              </div>
              <template v-else-if="selected.customRole">
                <div style="margin-bottom:10px;display:flex;align-items:center;gap:8px">
                  <span class="text-muted text-sm">Role</span>
                  <span class="badge badge-purple">custom</span>
                </div>
                <div class="graph-rules-table-wrap">
                  <table class="graph-rules-table">
                    <thead><tr><th>API Groups</th><th>Resources</th><th>Verbs</th></tr></thead>
                    <tbody>
                      <tr v-for="(rule, i) in selected.rules" :key="i">
                        <td class="font-mono muted">{{ rule.apiGroups.join('') === '' ? '(core)' : rule.apiGroups.join(', ') }}</td>
                        <td class="font-mono">{{ rule.resources.join(', ') }}</td>
                        <td><span v-for="v in rule.verbs" :key="v" class="badge badge-info" style="margin:1px;font-size:10px">{{ v }}</span></td>
                      </tr>
                    </tbody>
                  </table>
                </div>
              </template>
            </div>
          </div>

          <!-- Namespace access -->
          <div v-if="selected.namespaceBindings?.length" class="graph-access-block">
            <div class="graph-access-title">
              <svg viewBox="0 0 20 20" fill="currentColor" width="13" height="13"><path d="M3 4a1 1 0 011-1h12a1 1 0 011 1v2a1 1 0 01-1 1H4a1 1 0 01-1-1V4zM3 10a1 1 0 011-1h6a1 1 0 011 1v6a1 1 0 01-1 1H4a1 1 0 01-1-1v-6zM14 9a1 1 0 00-1 1v6a1 1 0 001 1h2a1 1 0 001-1v-6a1 1 0 00-1-1h-2z"/></svg>
              Namespaces ({{ selected.namespaceBindings.length }})
            </div>
            <div v-for="nb in selected.namespaceBindings" :key="nb.namespace" class="graph-ns-row">
              <div class="graph-ns-row-header">
                <code class="graph-ns-name">{{ nb.namespace }}</code>
                <span v-if="nb.role" class="badge" :class="roleClass(nb.role)">{{ nb.role }}</span>
                <span v-else-if="nb.customRole" class="badge badge-purple">custom</span>
              </div>
              <div v-if="nb.rules?.length" class="graph-rules-table-wrap" style="margin-top:8px">
                <table class="graph-rules-table">
                  <thead><tr><th>API Groups</th><th>Resources</th><th>Verbs</th></tr></thead>
                  <tbody>
                    <tr v-for="(rule, i) in nb.rules" :key="i">
                      <td class="font-mono muted">{{ rule.apiGroups.join('') === '' ? '(core)' : rule.apiGroups.join(', ') }}</td>
                      <td class="font-mono">{{ rule.resources.join(', ') }}</td>
                      <td><span v-for="v in rule.verbs" :key="v" class="badge badge-info" style="margin:1px;font-size:10px">{{ v }}</span></td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </div>
          </div>

          <!-- No bindings -->
          <div v-if="!selected.clusterRole && !selected.customRole && !selected.namespaceBindings?.length"
            class="graph-empty-state">
            <p style="margin:0;font-size:13px;color:var(--text-muted)">No RBAC bindings for this user.</p>
          </div>
        </template>
      </div>

    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import AppLayout from '@/components/AppLayout.vue'
import { listUsers, type User } from '@/api/users'

const users       = ref<User[]>([])
const loading     = ref(true)
const error       = ref('')
const selected    = ref<User | null>(null)
const filterGroup = ref('')

onMounted(async () => {
  try {
    users.value = await listUsers()
  } catch (e: any) {
    error.value = e.response?.data?.error ?? 'Failed to load users'
  } finally {
    loading.value = false
  }
})

const allGroups = computed(() => {
  const s = new Set<string>()
  for (const u of users.value) u.groups?.forEach(g => s.add(g))
  return [...s].sort()
})

const filteredUsers = computed(() =>
  filterGroup.value
    ? users.value.filter(u => u.groups?.includes(filterGroup.value))
    : users.value
)

function roleClass(role: string): string {
  if (role === 'cluster-admin') return 'badge-danger'
  if (role === 'admin')         return 'badge-warning'
  if (role === 'edit')          return 'badge-primary-soft'
  if (role === 'view')          return 'badge-gray'
  return 'badge-gray'
}

function statusClass(status: string): string {
  return { Active: 'badge-success', Pending: 'badge-warning', Denied: 'badge-danger', Failed: 'badge-danger' }[status] ?? 'badge-gray'
}
</script>
