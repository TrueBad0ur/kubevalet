<template>
  <AppLayout title="Local Users">
    <template #actions>
      <button v-if="enabled" class="btn btn-primary" @click="openCreate">
        <svg width="14" height="14" viewBox="0 0 20 20" fill="currentColor"><path fill-rule="evenodd" d="M10 3a1 1 0 011 1v5h5a1 1 0 110 2h-5v5a1 1 0 11-2 0v-5H4a1 1 0 110-2h5V4a1 1 0 011-1z" clip-rule="evenodd"/></svg>
        New User
      </button>
    </template>

    <Teleport to="body">
      <div v-if="toast.msg" class="toast" :class="toast.type === 'error' ? 'toast-error' : 'toast-success'">{{ toast.msg }}</div>
    </Teleport>

    <!-- Feature disabled -->
    <div v-if="!enabled" class="card" style="max-width:480px">
      <div style="padding:40px 32px;text-align:center">
        <svg width="36" height="36" viewBox="0 0 20 20" fill="currentColor" style="color:var(--text-muted);margin-bottom:16px">
          <path fill-rule="evenodd" d="M5 9V7a5 5 0 0110 0v2a2 2 0 012 2v5a2 2 0 01-2 2H5a2 2 0 01-2-2v-5a2 2 0 012-2zm8-2v2H7V7a3 3 0 016 0z" clip-rule="evenodd"/>
        </svg>
        <h3 style="margin:0 0 8px;font-size:15px;font-weight:600">Local Users disabled</h3>
        <p class="text-muted text-sm" style="margin:0">
          Enable in Helm values:<br>
          <code style="font-size:12px;background:var(--surface-alt,#f1f5f9);padding:2px 6px;border-radius:4px;display:inline-block;margin-top:8px">localUsers.enabled: true</code>
        </p>
      </div>
    </div>

    <!-- Users table -->
    <div v-else class="card">
      <div v-if="loading" style="padding:40px;text-align:center">
        <span class="spinner" style="width:24px;height:24px" />
      </div>

      <div v-else-if="users.length === 0" class="empty-state">
        <h3>No local users</h3>
        <p>Only the initial admin account exists.</p>
        <button class="btn btn-primary" @click="openCreate">Create User</button>
      </div>

      <div v-else class="table-wrap">
        <table>
          <thead>
            <tr>
              <th>Username</th>
              <th>Role</th>
              <th>Created</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="u in users" :key="u.username">
              <td class="font-mono" style="font-weight:600">
                {{ u.username }}
                <span v-if="u.username === currentUsername" class="badge badge-gray" style="margin-left:6px;font-size:10px">you</span>
              </td>
              <td>
                <span class="badge" :class="u.role === 'admin' ? 'badge-warning' : 'badge-gray'">
                  {{ u.role }}
                </span>
              </td>
              <td class="text-muted text-sm">{{ new Date(u.createdAt).toLocaleString() }}</td>
              <td style="white-space:nowrap">
                <div class="flex gap-2" style="align-items:center">
                  <!-- Role toggle: only for other users, not for 'admin' username -->
                  <button
                    v-if="u.username !== 'admin' && u.username !== currentUsername"
                    class="btn btn-ghost btn-sm"
                    :disabled="togglingRole === u.username"
                    @click="toggleRole(u)">
                    <span v-if="togglingRole === u.username" class="spinner" style="width:12px;height:12px" />
                    <span v-else>{{ u.role === 'admin' ? 'Make viewer' : 'Make admin' }}</span>
                  </button>
                  <!-- Change password: only for viewers (admins manage own password via Settings) -->
                  <button
                    v-if="u.role !== 'admin'"
                    class="btn btn-ghost btn-sm"
                    @click="openReset(u.username)">
                    Change password
                  </button>
                  <!-- Delete: not for 'admin' user, not for self -->
                  <button
                    v-if="u.username !== 'admin' && u.username !== currentUsername"
                    class="btn btn-danger btn-sm"
                    :disabled="deleting === u.username"
                    @click="confirmDelete(u.username)">
                    <span v-if="deleting === u.username" class="spinner" />
                    <span v-else>Delete</span>
                  </button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- Create modal -->
    <div v-if="createOpen" class="modal-overlay" @click.self="createOpen = false">
      <div class="modal" style="max-width:400px">
        <div class="modal-header">New local user</div>
        <div class="modal-body">
          <div v-if="createError" class="alert alert-error" style="margin-bottom:12px">{{ createError }}</div>
          <form @submit.prevent="submitCreate">
            <div class="form-group">
              <label class="form-label">Username <span class="required">*</span></label>
              <input v-model="createForm.username" type="text" class="form-input" required autocomplete="off" />
            </div>
            <div class="form-group">
              <label class="form-label">Password <span class="required">*</span></label>
              <input v-model="createForm.password" type="password" class="form-input" required autocomplete="new-password" />
            </div>
            <div class="form-group">
              <label class="form-label">Confirm password <span class="required">*</span></label>
              <input v-model="createForm.confirm" type="password" class="form-input" required autocomplete="new-password" />
            </div>
            <div class="form-group">
              <label class="form-label">Role</label>
              <div class="radio-group">
                <label class="radio-option">
                  <input type="radio" v-model="createForm.role" value="viewer" /> Viewer <span class="text-muted text-sm">(read-only)</span>
                </label>
                <label class="radio-option">
                  <input type="radio" v-model="createForm.role" value="admin" /> Admin <span class="text-muted text-sm">(full access)</span>
                </label>
              </div>
            </div>
            <div class="modal-footer" style="padding:0;border:none;margin-top:4px">
              <button type="button" class="btn btn-ghost" @click="createOpen = false">Cancel</button>
              <button type="submit" class="btn btn-primary" :disabled="createSaving">
                <span v-if="createSaving" class="spinner" />
                <span v-else>Create</span>
              </button>
            </div>
          </form>
        </div>
      </div>
    </div>

    <!-- Change password modal (viewers only) -->
    <div v-if="resetTarget" class="modal-overlay" @click.self="resetTarget = null">
      <div class="modal" style="max-width:400px">
        <div class="modal-header">Change password — <strong class="font-mono">{{ resetTarget }}</strong></div>
        <div class="modal-body">
          <div v-if="resetError" class="alert alert-error" style="margin-bottom:12px">{{ resetError }}</div>
          <form @submit.prevent="submitReset">
            <div class="form-group">
              <label class="form-label">New password <span class="required">*</span></label>
              <input v-model="resetForm.password" type="password" class="form-input" required autocomplete="new-password" />
            </div>
            <div class="form-group">
              <label class="form-label">Confirm password <span class="required">*</span></label>
              <input v-model="resetForm.confirm" type="password" class="form-input" required autocomplete="new-password" />
            </div>
            <div class="modal-footer" style="padding:0;border:none;margin-top:4px">
              <button type="button" class="btn btn-ghost" @click="resetTarget = null">Cancel</button>
              <button type="submit" class="btn btn-primary" :disabled="resetSaving">
                <span v-if="resetSaving" class="spinner" />
                <span v-else>Save</span>
              </button>
            </div>
          </form>
        </div>
      </div>
    </div>

    <!-- Delete confirmation -->
    <div v-if="deleteTarget" class="modal-overlay" @click.self="deleteTarget = null">
      <div class="modal" style="max-width:400px">
        <div class="modal-header">Confirm deletion</div>
        <div class="modal-body">
          <p>Delete local user <strong class="font-mono">{{ deleteTarget }}</strong>?</p>
          <p class="text-muted text-sm" style="margin-top:8px">This cannot be undone.</p>
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
import { ref, reactive, onMounted } from 'vue'
import AppLayout from '@/components/AppLayout.vue'
import { listLocalUsers, createLocalUser, deleteLocalUser, resetLocalUserPassword, updateLocalUserRole, type LocalUser } from '@/api/localUsers'
import { getSettings } from '@/api/settings'
import { useAuth } from '@/composables/useAuth'

const { username: currentUsername } = useAuth()

const enabled      = ref(false)
const users        = ref<LocalUser[]>([])
const loading      = ref(true)
const deleting     = ref('')
const togglingRole = ref('')
const toast        = reactive({ msg: '', type: 'success' })

function showToast(msg: string, type: 'success' | 'error' = 'success') {
  toast.msg  = msg
  toast.type = type
  setTimeout(() => { toast.msg = '' }, 3500)
}

// Create
const createOpen   = ref(false)
const createSaving = ref(false)
const createError  = ref('')
const createForm   = reactive({ username: '', password: '', confirm: '', role: 'viewer' })

function openCreate() {
  createForm.username = ''
  createForm.password = ''
  createForm.confirm  = ''
  createForm.role     = 'viewer'
  createError.value   = ''
  createOpen.value    = true
}

async function submitCreate() {
  createError.value = ''
  if (createForm.password !== createForm.confirm) {
    createError.value = 'Passwords do not match'
    return
  }
  createSaving.value = true
  try {
    await createLocalUser(createForm.username, createForm.password, createForm.role)
    createOpen.value = false
    showToast(`User "${createForm.username}" created`)
    await load()
  } catch (e: any) {
    createError.value = e.response?.data?.error ?? 'Failed to create user'
  } finally {
    createSaving.value = false
  }
}

// Role toggle
async function toggleRole(u: LocalUser) {
  togglingRole.value = u.username
  const newRole = u.role === 'admin' ? 'viewer' : 'admin'
  try {
    await updateLocalUserRole(u.username, newRole)
    showToast(`${u.username} is now ${newRole}`)
    await load()
  } catch (e: any) {
    showToast(e.response?.data?.error ?? 'Failed to update role', 'error')
  } finally {
    togglingRole.value = ''
  }
}

// Change password (viewers only)
const resetTarget = ref<string | null>(null)
const resetSaving = ref(false)
const resetError  = ref('')
const resetForm   = reactive({ password: '', confirm: '' })

function openReset(username: string) {
  resetTarget.value  = username
  resetForm.password = ''
  resetForm.confirm  = ''
  resetError.value   = ''
}

async function submitReset() {
  resetError.value = ''
  if (resetForm.password !== resetForm.confirm) {
    resetError.value = 'Passwords do not match'
    return
  }
  resetSaving.value = true
  try {
    await resetLocalUserPassword(resetTarget.value!, resetForm.password)
    resetTarget.value = null
    showToast('Password updated')
  } catch (e: any) {
    resetError.value = e.response?.data?.error ?? 'Failed to update password'
  } finally {
    resetSaving.value = false
  }
}

// Delete
const deleteTarget = ref<string | null>(null)

function confirmDelete(username: string) {
  deleteTarget.value = username
}

async function doDelete() {
  if (!deleteTarget.value) return
  deleting.value = deleteTarget.value
  try {
    await deleteLocalUser(deleteTarget.value)
    deleteTarget.value = null
    showToast('User deleted')
    await load()
  } catch (e: any) {
    showToast(e.response?.data?.error ?? 'Delete failed', 'error')
  } finally {
    deleting.value = ''
  }
}

async function load() {
  loading.value = true
  try {
    users.value = await listLocalUsers()
  } catch {}
  finally {
    loading.value = false
  }
}

onMounted(async () => {
  try {
    const s = await getSettings()
    enabled.value = s.localUsersEnabled
  } catch {}
  if (enabled.value) await load()
  else loading.value = false
})
</script>
