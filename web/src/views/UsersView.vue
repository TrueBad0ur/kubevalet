<template>
  <AppLayout title="Users">
    <template #actions>
      <RouterLink to="/users/new" class="btn btn-primary">
        <svg width="14" height="14" viewBox="0 0 20 20" fill="currentColor"><path fill-rule="evenodd" d="M10 3a1 1 0 011 1v5h5a1 1 0 110 2h-5v5a1 1 0 11-2 0v-5H4a1 1 0 110-2h5V4a1 1 0 011-1z" clip-rule="evenodd"/></svg>
        New User
      </RouterLink>
    </template>

    <div v-if="error" class="alert alert-error">{{ error }}</div>

    <div class="card">
      <div v-if="loading" style="padding:40px;text-align:center">
        <span class="spinner" style="width:24px;height:24px" />
      </div>

      <div v-else-if="users.length === 0" class="empty-state">
        <h3>No users yet</h3>
        <p>Create your first Kubernetes user to get started.</p>
        <RouterLink to="/users/new" class="btn btn-primary">Create User</RouterLink>
      </div>

      <div v-else class="table-wrap">
        <table>
          <thead>
            <tr>
              <th>Name</th>
              <th>Groups</th>
              <th>Binding</th>
              <th>Status</th>
              <th>Created</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="u in users" :key="u.name">
              <td class="font-mono">{{ u.name }}</td>
              <td>
                <span v-if="u.groups?.length" class="flex gap-2" style="flex-wrap:wrap">
                  <span v-for="g in u.groups" :key="g" class="badge badge-info">{{ g }}</span>
                </span>
                <span v-else class="text-muted text-sm">—</span>
              </td>
              <td>
                <span v-if="u.customRole && !u.namespace" class="badge badge-gray"
                  style="cursor:pointer" @click="rulesTarget = u" title="Click to view rules">
                  cluster / custom ▾
                </span>
                <span v-else-if="u.customRole && u.namespace" class="badge badge-gray"
                  style="cursor:pointer" @click="rulesTarget = u" title="Click to view rules">
                  {{ u.namespace }} / custom ▾
                </span>
                <span v-else-if="u.clusterRole" class="badge badge-gray">cluster / {{ u.clusterRole }}</span>
                <span v-else-if="u.namespace" class="badge badge-gray">{{ u.namespace }} / {{ u.role }}</span>
                <span v-else class="text-muted text-sm">—</span>
              </td>
              <td>
                <span class="badge" :class="statusClass(u.status)">{{ u.status }}</span>
              </td>
              <td class="text-muted text-sm">{{ formatDate(u.createdAt) }}</td>
              <td>
                <div class="flex gap-2">
                  <button class="btn btn-ghost btn-sm" @click="viewKubeconfig(u.name)" :disabled="viewing === u.name">
                    <span v-if="viewing === u.name" class="spinner" />
                    <span v-else>View</span>
                  </button>
                  <button class="btn btn-ghost btn-sm" @click="downloadKubeconfig(u.name)" :disabled="downloading === u.name">
                    <span v-if="downloading === u.name" class="spinner" />
                    <template v-else>
                      <svg width="13" height="13" viewBox="0 0 20 20" fill="currentColor"><path fill-rule="evenodd" d="M3 17a1 1 0 011-1h12a1 1 0 110 2H4a1 1 0 01-1-1zm3.293-7.707a1 1 0 011.414 0L9 10.586V3a1 1 0 112 0v7.586l1.293-1.293a1 1 0 111.414 1.414l-3 3a1 1 0 01-1.414 0l-3-3a1 1 0 010-1.414z" clip-rule="evenodd"/></svg>
                      kubeconfig
                    </template>
                  </button>
                  <button class="btn btn-danger btn-sm" @click="confirmDelete(u.name)" :disabled="deleting === u.name">
                    <span v-if="deleting === u.name" class="spinner" />
                    <span v-else>Delete</span>
                  </button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- Kubeconfig viewer modal -->
    <div v-if="viewTarget" class="modal-overlay" @click.self="viewTarget = null">
      <div class="modal" style="max-width:640px">
        <div class="modal-header">
          <span>kubeconfig — <strong class="font-mono">{{ viewTarget }}</strong></span>
        </div>
        <div class="modal-body">
          <textarea readonly class="code-block" style="width:100%;resize:none;border:none;outline:none;cursor:text"
            :rows="viewContent.split('\n').length">{{ viewContent }}</textarea>
        </div>
        <div class="modal-footer">
          <button class="btn btn-ghost" @click="copyViewed">{{ viewCopied ? '✓ Copied' : 'Copy' }}</button>
          <button class="btn btn-ghost btn-sm" @click="downloadKubeconfig(viewTarget!)">
            <svg width="13" height="13" viewBox="0 0 20 20" fill="currentColor"><path fill-rule="evenodd" d="M3 17a1 1 0 011-1h12a1 1 0 110 2H4a1 1 0 01-1-1zm3.293-7.707a1 1 0 011.414 0L9 10.586V3a1 1 0 112 0v7.586l1.293-1.293a1 1 0 111.414 1.414l-3 3a1 1 0 01-1.414 0l-3-3a1 1 0 010-1.414z" clip-rule="evenodd"/></svg>
            Download
          </button>
          <button class="btn btn-ghost" @click="viewTarget = null">Close</button>
        </div>
      </div>
    </div>

    <!-- Rules viewer modal -->
    <div v-if="rulesTarget" class="modal-overlay" @click.self="rulesTarget = null">
      <div class="modal" style="max-width:680px">
        <div class="modal-header">
          Rules — <strong class="font-mono">{{ rulesTarget.name }}</strong>
          <span class="badge badge-gray" style="margin-left:8px;font-size:11px">
            {{ rulesTarget.namespace ? rulesTarget.namespace : 'cluster-wide' }}
          </span>
        </div>
        <div class="modal-body" style="padding:0">
          <table style="width:100%;border-collapse:collapse;font-size:13px">
            <thead>
              <tr style="background:var(--surface-alt,#f8fafc)">
                <th style="padding:10px 16px;text-align:left;font-weight:600;border-bottom:1px solid var(--border)">API Groups</th>
                <th style="padding:10px 16px;text-align:left;font-weight:600;border-bottom:1px solid var(--border)">Resources</th>
                <th style="padding:10px 16px;text-align:left;font-weight:600;border-bottom:1px solid var(--border)">Verbs</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="(rule, i) in rulesTarget.rules" :key="i"
                style="border-bottom:1px solid var(--border)">
                <td style="padding:9px 16px;font-family:monospace;font-size:12px;vertical-align:top">
                  <span v-if="rule.apiGroups.join('') === ''" class="text-muted">(core)</span>
                  <span v-else>{{ rule.apiGroups.join(', ') }}</span>
                </td>
                <td style="padding:9px 16px;font-family:monospace;font-size:12px;vertical-align:top">
                  {{ rule.resources.join(', ') }}
                </td>
                <td style="padding:9px 16px;vertical-align:top">
                  <span v-for="v in rule.verbs" :key="v" class="badge badge-info" style="margin:2px 2px 2px 0;font-size:11px">{{ v }}</span>
                </td>
              </tr>
              <tr v-if="!rulesTarget.rules?.length">
                <td colspan="3" style="padding:16px;text-align:center;color:var(--text-muted)">No rules</td>
              </tr>
            </tbody>
          </table>
        </div>
        <div class="modal-footer">
          <button class="btn btn-ghost" @click="rulesTarget = null">Close</button>
        </div>
      </div>
    </div>

    <!-- Delete confirmation modal -->
    <div v-if="deleteTarget" class="modal-overlay" @click.self="deleteTarget = null">
      <div class="modal" style="max-width:400px">
        <div class="modal-header">Confirm deletion</div>
        <div class="modal-body">
          <p>Delete user <strong class="font-mono">{{ deleteTarget }}</strong>?</p>
          <p class="text-muted text-sm" style="margin-top:8px">
            This removes the CSR, private key Secret and all RBAC bindings. This cannot be undone.
          </p>
        </div>
        <div class="modal-footer">
          <button class="btn btn-ghost" @click="deleteTarget = null">Cancel</button>
          <button class="btn btn-danger" @click="doDelete" :disabled="!!deleting">
            <span v-if="deleting" class="spinner" />
            <span v-else>Delete</span>
          </button>
        </div>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { RouterLink } from 'vue-router'
import AppLayout from '@/components/AppLayout.vue'
import { listUsers, deleteUser, type User } from '@/api/users'
import { client } from '@/api/client'

const users       = ref<User[]>([])
const loading     = ref(true)
const error       = ref('')
const deleting    = ref('')
const downloading = ref('')
const viewing     = ref('')
const deleteTarget = ref<string | null>(null)
const viewTarget  = ref<string | null>(null)
const viewContent = ref('')
const viewCopied  = ref(false)
const rulesTarget = ref<User | null>(null)

async function load() {
  loading.value = true
  error.value   = ''
  try {
    users.value = await listUsers()
  } catch (e: any) {
    error.value = e.response?.data?.error ?? 'Failed to load users'
  } finally {
    loading.value = false
  }
}

async function viewKubeconfig(name: string) {
  viewing.value = name
  error.value = ''
  try {
    const res = await client.get(`/users/${name}/kubeconfig`, { responseType: 'blob' })
    viewContent.value = await res.data.text()
    viewTarget.value = name
  } catch (e: any) {
    if (e.response?.data instanceof Blob) {
      try {
        const text = await e.response.data.text()
        error.value = JSON.parse(text).error ?? 'Failed to load kubeconfig'
      } catch {
        error.value = 'Failed to load kubeconfig'
      }
    } else {
      error.value = e.response?.data?.error ?? 'Failed to load kubeconfig'
    }
  } finally {
    viewing.value = ''
  }
}

async function copyViewed() {
  try {
    await navigator.clipboard.writeText(viewContent.value)
  } catch {
    const ta = document.createElement('textarea')
    ta.value = viewContent.value
    ta.style.position = 'fixed'
    ta.style.opacity = '0'
    document.body.appendChild(ta)
    ta.focus()
    ta.select()
    document.execCommand('copy')
    document.body.removeChild(ta)
  }
  viewCopied.value = true
  setTimeout(() => { viewCopied.value = false }, 2000)
}

async function downloadKubeconfig(name: string) {
  downloading.value = name
  try {
    const res = await client.get(`/users/${name}/kubeconfig`, { responseType: 'blob' })
    const url = URL.createObjectURL(new Blob([res.data], { type: 'application/x-yaml' }))
    const a = document.createElement('a')
    a.href = url
    a.download = `${name}.kubeconfig`
    a.click()
    URL.revokeObjectURL(url)
  } catch (e: any) {
    if (e.response?.data instanceof Blob) {
      try {
        const text = await e.response.data.text()
        error.value = JSON.parse(text).error ?? 'Failed to download kubeconfig'
      } catch {
        error.value = 'Failed to download kubeconfig'
      }
    } else {
      error.value = e.response?.data?.error ?? 'Failed to download kubeconfig'
    }
  } finally {
    downloading.value = ''
  }
}

function confirmDelete(name: string) {
  deleteTarget.value = name
}

async function doDelete() {
  if (!deleteTarget.value) return
  deleting.value = deleteTarget.value
  try {
    await deleteUser(deleteTarget.value)
    deleteTarget.value = null
    await load()
  } catch (e: any) {
    error.value = e.response?.data?.error ?? 'Delete failed'
  } finally {
    deleting.value = ''
  }
}

function statusClass(status: string) {
  return {
    Active:  'badge-success',
    Pending: 'badge-warning',
    Denied:  'badge-danger',
    Failed:  'badge-danger',
  }[status] ?? 'badge-gray'
}

function formatDate(iso: string) {
  return new Date(iso).toLocaleString()
}

onMounted(load)
</script>
