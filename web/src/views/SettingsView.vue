<template>
  <AppLayout title="Settings">
    <div style="max-width:480px;display:flex;flex-direction:column;gap:24px">

      <!-- Cluster (admin-only) -->
      <div v-if="isAdmin" class="card">
        <div style="padding:16px 20px;border-bottom:1px solid var(--border)">
          <h3 style="margin:0;font-size:14px;font-weight:600">Cluster</h3>
        </div>
        <div style="padding:20px">
          <div v-if="clusterError"   class="alert alert-error"   style="margin-bottom:16px">{{ clusterError }}</div>
          <div v-if="clusterSuccess" class="alert alert-success" style="margin-bottom:16px">{{ clusterSuccess }}</div>
          <form @submit.prevent="submitCluster">
            <div class="form-group">
              <label class="form-label">Kubernetes API server address</label>
              <input v-model="clusterServer" type="text" class="form-input"
                placeholder="e.g. https://192.168.1.100:6443" />
              <p class="form-hint">Used in generated kubeconfigs. Overrides the default in-cluster address. Leave blank to use the server default.</p>
            </div>
            <button type="submit" class="btn btn-primary" :disabled="clusterSaving">
              <span v-if="clusterSaving" class="spinner" />
              <span v-else>Save</span>
            </button>
          </form>
        </div>
      </div>

      <!-- Change password -->
      <div class="card">
        <div style="padding:16px 20px;border-bottom:1px solid var(--border)">
          <h3 style="margin:0;font-size:14px;font-weight:600">Change Password</h3>
        </div>
        <div style="padding:20px">
          <div v-if="pwError"   class="alert alert-error"   style="margin-bottom:16px">{{ pwError }}</div>
          <div v-if="pwSuccess" class="alert alert-success" style="margin-bottom:16px">{{ pwSuccess }}</div>
          <form @submit.prevent="submitPassword">
            <div class="form-group">
              <label class="form-label">Current password</label>
              <input v-model="pwForm.current" type="password" class="form-input" required autocomplete="current-password" />
            </div>
            <div class="form-group">
              <label class="form-label">New password</label>
              <input v-model="pwForm.next" type="password" class="form-input" required autocomplete="new-password"
                />
            </div>
            <div class="form-group">
              <label class="form-label">Confirm new password</label>
              <input v-model="pwForm.confirm" type="password" class="form-input" required autocomplete="new-password" />
            </div>
            <button type="submit" class="btn btn-primary" :disabled="pwSaving" style="margin-top:4px">
              <span v-if="pwSaving" class="spinner" />
              <span v-else>Update password</span>
            </button>
          </form>
        </div>
      </div>

      <!-- App version -->
      <div class="card">
        <div style="padding:16px 20px;border-bottom:1px solid var(--border)">
          <h3 style="margin:0;font-size:14px;font-weight:600">About</h3>
        </div>
        <div style="padding:20px;display:flex;align-items:center;justify-content:space-between">
          <span class="text-muted text-sm">kubevalet version</span>
          <span class="badge badge-gray font-mono" style="font-size:13px">{{ version }}</span>
        </div>
      </div>

    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import AppLayout from '@/components/AppLayout.vue'
import { getSettings, updateSettings, changePassword } from '@/api/settings'
import { useAuth } from '@/composables/useAuth'

const { isAdmin } = useAuth()

const version        = ref('…')
const clusterServer  = ref('')
const clusterSaving  = ref(false)
const clusterError   = ref('')
const clusterSuccess = ref('')

const pwSaving  = ref(false)
const pwError   = ref('')
const pwSuccess = ref('')

const pwForm = reactive({ current: '', next: '', confirm: '' })

onMounted(async () => {
  try {
    const s = await getSettings()
    version.value       = s.version
    clusterServer.value = s.clusterServer ?? ''
  } catch {
    version.value = 'unknown'
  }
})

async function submitCluster() {
  clusterError.value   = ''
  clusterSuccess.value = ''
  clusterSaving.value  = true
  try {
    await updateSettings({ clusterServer: clusterServer.value })
    clusterSuccess.value = 'Saved. New kubeconfigs will use this address.'
  } catch (e: any) {
    clusterError.value = e.response?.data?.error ?? 'Failed to save'
  } finally {
    clusterSaving.value = false
  }
}

async function submitPassword() {
  pwError.value   = ''
  pwSuccess.value = ''
  if (pwForm.next !== pwForm.confirm) {
    pwError.value = 'New passwords do not match'
    return
  }
  pwSaving.value = true
  try {
    await changePassword(pwForm.current, pwForm.next)
    pwSuccess.value = 'Password updated successfully'
    pwForm.current = ''
    pwForm.next    = ''
    pwForm.confirm = ''
  } catch (e: any) {
    pwError.value = e.response?.data?.error ?? 'Failed to update password'
  } finally {
    pwSaving.value = false
  }
}
</script>
