<template>
  <AppLayout title="Role Templates">
    <Teleport to="body">
      <div v-if="errorMsg" class="toast toast-error">{{ errorMsg }}</div>
    </Teleport>

    <div class="card">
      <div v-if="loading" style="padding:40px;text-align:center"><span class="spinner" style="width:22px;height:22px"/></div>
      <div v-else-if="templates.length === 0" class="empty-state">
        <h3>No templates yet</h3>
        <p>Save a role configuration as a template from the Create User or Edit forms.</p>
      </div>
      <div v-else class="table-wrap">
        <table>
          <thead>
            <tr>
              <th>Name</th>
              <th>Description</th>
              <th>Access</th>
              <th>Created</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            <template v-for="t in templates" :key="t.id">
              <tr style="cursor:pointer" @click="toggle(t.id)">
                <td class="font-mono" style="font-weight:600">
                  <span style="margin-right:6px;font-size:10px;color:var(--text-muted)">{{ expanded === t.id ? '▾' : '▸' }}</span>{{ t.name }}
                </td>
                <td class="text-muted">{{ t.description || '—' }}</td>
                <td>
                  <span v-if="t.clusterRole" class="badge badge-info">{{ t.clusterRole }}</span>
                  <span v-else-if="t.customRole && !t.namespaceBindings?.length" class="badge badge-purple">custom cluster</span>
                  <span v-else-if="t.namespaceBindings?.length" class="text-muted text-sm">{{ t.namespaceBindings.length }} namespace{{ t.namespaceBindings.length !== 1 ? 's' : '' }}</span>
                  <span v-else class="text-muted text-sm">groups only</span>
                </td>
                <td class="text-muted text-sm">{{ fmtDate(t.createdAt) }}</td>
                <td v-if="isAdmin" style="text-align:right" @click.stop>
                  <button class="btn btn-danger btn-sm" :disabled="deleting === t.id" @click="doDelete(t.id)">
                    <span v-if="deleting === t.id" class="spinner"/>
                    <span v-else>Delete</span>
                  </button>
                </td>
                <td v-else></td>
              </tr>

              <!-- Expanded detail row -->
              <tr v-if="expanded === t.id">
                <td colspan="5" style="padding:0;background:var(--bg-subtle,var(--bg))">
                  <div style="padding:12px 20px 16px;border-top:1px solid var(--border)">

                    <!-- Preset cluster role -->
                    <template v-if="t.clusterRole">
                      <div class="text-muted text-sm" style="margin-bottom:4px">Cluster role binding</div>
                      <span class="badge badge-info">{{ t.clusterRole }}</span>
                    </template>

                    <!-- Custom cluster rules -->
                    <template v-else-if="t.customRole && t.rules?.length">
                      <div class="text-muted text-sm" style="margin-bottom:8px">Custom cluster rules</div>
                      <div v-for="(rule, i) in t.rules" :key="i" style="margin-bottom:8px;padding:8px 10px;background:var(--bg-card,var(--surface));border:1px solid var(--border);border-radius:6px;font-size:12px;font-family:monospace">
                        <div><span style="color:var(--text-muted)">apiGroups:</span> {{ rule.apiGroups?.join(', ') || '(core)' }}</div>
                        <div><span style="color:var(--text-muted)">resources:</span> {{ rule.resources?.join(', ') }}</div>
                        <div><span style="color:var(--text-muted)">verbs:    </span> {{ rule.verbs?.join(', ') }}</div>
                      </div>
                    </template>

                    <!-- Namespace bindings -->
                    <template v-else-if="t.namespaceBindings?.length">
                      <div class="text-muted text-sm" style="margin-bottom:8px">Namespace bindings</div>
                      <div v-for="(nb, i) in t.namespaceBindings" :key="i" style="margin-bottom:8px;padding:8px 10px;background:var(--bg-card,var(--surface));border:1px solid var(--border);border-radius:6px;font-size:12px">
                        <div style="font-weight:600;font-family:monospace;margin-bottom:4px">{{ nb.namespace }}</div>
                        <template v-if="nb.role">
                          <span class="badge badge-info" style="font-size:11px">{{ nb.role }}</span>
                        </template>
                        <template v-else-if="nb.rules?.length">
                          <div v-for="(rule, ri) in nb.rules" :key="ri" style="font-family:monospace;margin-top:4px">
                            <div><span style="color:var(--text-muted)">apiGroups:</span> {{ rule.apiGroups?.join(', ') || '(core)' }}</div>
                            <div><span style="color:var(--text-muted)">resources:</span> {{ rule.resources?.join(', ') }}</div>
                            <div><span style="color:var(--text-muted)">verbs:    </span> {{ rule.verbs?.join(', ') }}</div>
                          </div>
                        </template>
                      </div>
                    </template>

                    <template v-else>
                      <span class="text-muted text-sm">Groups only — no direct RBAC</span>
                    </template>
                  </div>
                </td>
              </tr>
            </template>
          </tbody>
        </table>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import AppLayout from '@/components/AppLayout.vue'
import { listTemplates, deleteTemplate, type RoleTemplate } from '@/api/templates'

const templates = ref<RoleTemplate[]>([])
const loading   = ref(true)
const deleting  = ref<number | null>(null)
const errorMsg  = ref('')
const isAdmin   = ref(false)
const expanded  = ref<number | null>(null)

function toggle(id: number) {
  expanded.value = expanded.value === id ? null : id
}

onMounted(async () => {
  try {
    const token = localStorage.getItem('token')
    if (token) {
      const seg = token.split('.')[1].replace(/-/g, '+').replace(/_/g, '/')
      const padded = seg + '='.repeat((4 - seg.length % 4) % 4)
      isAdmin.value = JSON.parse(atob(padded))?.role === 'admin'
    }
    templates.value = await listTemplates()
  } catch {
    errorMsg.value = 'Failed to load templates'
  } finally {
    loading.value = false
  }
})

async function doDelete(id: number) {
  deleting.value = id
  try {
    await deleteTemplate(id)
    templates.value = templates.value.filter(t => t.id !== id)
    if (expanded.value === id) expanded.value = null
  } catch {
    errorMsg.value = 'Failed to delete template'
  } finally {
    deleting.value = null
  }
}

function fmtDate(s: string) {
  return new Date(s).toLocaleDateString()
}
</script>
