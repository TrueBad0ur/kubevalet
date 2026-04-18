<template>
  <AppLayout title="Clusters">
    <template #actions>
      <button class="btn btn-primary" @click="openAdd">+ Add Cluster</button>
    </template>

    <Teleport to="body">
      <div v-if="errorMsg" class="toast toast-error">{{ errorMsg }}</div>
    </Teleport>

    <div class="card">
      <div v-if="loading" style="padding:40px;text-align:center"><span class="spinner" style="width:22px;height:22px"/></div>
      <div v-else-if="clusters.length === 0" class="empty-state">
        <h3>No clusters</h3>
        <p>Add a cluster by providing its kubeconfig.</p>
      </div>
      <div v-else class="table-wrap">
        <table>
          <thead>
            <tr>
              <th>Name</th>
              <th>Description</th>
              <th>API Server</th>
              <th>Added</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="c in clusters" :key="c.id" :class="{ 'row-active': c.id === currentID }">
              <td class="font-mono" style="font-weight:600">
                {{ c.name }}
                <span v-if="c.id === currentID" class="badge badge-success" style="margin-left:6px;font-size:10px">active</span>
              </td>
              <td class="text-muted">{{ c.description || '—' }}</td>
              <td class="text-muted text-sm font-mono">{{ c.apiServer || '—' }}</td>
              <td class="text-muted text-sm">{{ fmtDate(c.createdAt) }}</td>
              <td style="text-align:right;white-space:nowrap">
                <button class="btn btn-ghost btn-sm" style="margin-right:4px" @click="selectCluster(c.id)">Switch</button>
                <button class="btn btn-danger btn-sm" :disabled="deleting === c.id" @click="doDelete(c.id)">
                  <span v-if="deleting === c.id" class="spinner"/>
                  <span v-else>Delete</span>
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- Add cluster modal -->
    <div v-if="addOpen" class="modal-overlay">
      <div class="modal" style="max-width:560px">
        <div class="modal-header">Add cluster</div>
        <div class="modal-body">
          <div class="form-group">
            <label class="form-label">Name <span class="required">*</span></label>
            <input v-model="form.name" type="text" class="form-input" placeholder="e.g. prod-eu" pattern="[a-z0-9][a-z0-9\-]*" />
          </div>
          <div class="form-group">
            <label class="form-label">Description</label>
            <input v-model="form.description" type="text" class="form-input" placeholder="Optional" />
          </div>
          <div class="form-group">
            <label class="form-label">API Server <span class="required">*</span></label>
            <input v-model="form.apiServer" type="text" class="form-input" placeholder="https://api.example.com:6443" />
            <p class="form-hint">URL embedded in generated kubeconfigs for users of this cluster.</p>
          </div>
          <div class="form-group">
            <label class="form-label">Cluster name in kubeconfig</label>
            <input v-model="form.clusterName" type="text" class="form-input" placeholder="kubernetes" />
          </div>
          <div class="form-group" style="margin-bottom:0">
            <label class="form-label">Kubeconfig <span class="required">*</span></label>
            <textarea v-model="form.kubeconfig" class="form-input code-block" rows="8"
              placeholder="Paste kubeconfig YAML here" style="font-size:11px;font-family:monospace;resize:vertical" />
            <p class="form-hint">Service account kubeconfig with cluster-admin permissions.</p>
          </div>
          <div v-if="addError" class="alert alert-error" style="margin-top:12px">{{ addError }}</div>
        </div>
        <div class="modal-footer">
          <button class="btn btn-ghost" @click="addOpen = false">Cancel</button>
          <button class="btn btn-primary" :disabled="adding || !form.name || !form.kubeconfig || !form.apiServer" @click="doAdd">
            <span v-if="adding" class="spinner"/>
            <span v-else>Add</span>
          </button>
        </div>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import AppLayout from '@/components/AppLayout.vue'
import { listClusters, createCluster, deleteCluster } from '@/api/clusters'
import { useCluster } from '@/composables/useCluster'

const { clusters, currentID, setClusters, selectCluster } = useCluster()

const loading  = ref(true)
const deleting = ref<number | null>(null)
const errorMsg = ref('')
const addOpen  = ref(false)
const adding   = ref(false)
const addError = ref('')

const form = ref({ name: '', description: '', apiServer: '', clusterName: '', kubeconfig: '' })

onMounted(async () => {
  try {
    const list = await listClusters()
    setClusters(list)
  } catch {
    errorMsg.value = 'Failed to load clusters'
  } finally {
    loading.value = false
  }
})

function openAdd() {
  form.value = { name: '', description: '', apiServer: '', clusterName: '', kubeconfig: '' }
  addError.value = ''
  addOpen.value = true
}

async function doAdd() {
  adding.value = true
  addError.value = ''
  try {
    const c = await createCluster({
      name: form.value.name,
      description: form.value.description,
      apiServer: form.value.apiServer,
      clusterName: form.value.clusterName || 'kubernetes',
      kubeconfig: form.value.kubeconfig,
    })
    setClusters([...clusters.value, c])
    addOpen.value = false
  } catch (e: any) {
    addError.value = e.response?.data?.error ?? 'Failed to add cluster'
  } finally {
    adding.value = false
  }
}

async function doDelete(id: number) {
  deleting.value = id
  try {
    await deleteCluster(id)
    setClusters(clusters.value.filter(c => c.id !== id))
  } catch (e: any) {
    errorMsg.value = e.response?.data?.error ?? 'Failed to delete cluster'
  } finally {
    deleting.value = null
  }
}

function fmtDate(s: string) {
  return new Date(s).toLocaleDateString()
}
</script>
