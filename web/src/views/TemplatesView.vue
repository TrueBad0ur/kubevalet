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
            <tr v-for="t in templates" :key="t.id">
              <td class="font-mono" style="font-weight:600">{{ t.name }}</td>
              <td class="text-muted">{{ t.description || '—' }}</td>
              <td>
                <span v-if="t.clusterRole" class="badge badge-info">{{ t.clusterRole }}</span>
                <span v-else-if="t.customRole && !t.namespaceBindings?.length" class="badge badge-purple">custom cluster</span>
                <span v-else-if="t.namespaceBindings?.length" class="text-muted text-sm">{{ t.namespaceBindings.length }} namespace{{ t.namespaceBindings.length !== 1 ? 's' : '' }}</span>
                <span v-else class="text-muted text-sm">groups only</span>
              </td>
              <td class="text-muted text-sm">{{ fmtDate(t.createdAt) }}</td>
              <td v-if="isAdmin" style="text-align:right">
                <button class="btn btn-danger btn-sm" :disabled="deleting === t.id" @click="doDelete(t.id)">
                  <span v-if="deleting === t.id" class="spinner"/>
                  <span v-else>Delete</span>
                </button>
              </td>
              <td v-else></td>
            </tr>
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
