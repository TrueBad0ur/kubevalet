<template>
  <AppLayout title="Groups">
    <template #actions>
      <button v-if="isAdmin" class="btn btn-primary" @click="openCreate">+ New Group</button>
    </template>

    <Teleport to="body">
      <div v-if="loadError" class="toast toast-error">{{ loadError }}</div>
      <div v-if="syncMsg"   class="toast toast-success">{{ syncMsg }}</div>
    </Teleport>

    <div class="card">
      <div class="card-header">
        <h2>Groups <span class="badge badge-gray" style="margin-left:6px">{{ groups.length }}</span></h2>
      </div>
      <div v-if="loading" style="padding:40px;text-align:center"><span class="spinner" style="width:22px;height:22px"/></div>
      <div v-else-if="groups.length === 0" class="empty-state">
        <h3>No groups yet</h3>
        <p>Create a group to manage k8s RBAC for multiple users at once.</p>
        <button v-if="isAdmin" class="btn btn-primary" @click="openCreate">+ New Group</button>
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
            <tr v-for="g in groups" :key="g.name">
              <td class="font-mono" style="font-weight:600">{{ g.name }}</td>
              <td class="text-muted">{{ g.description || '—' }}</td>
              <td>
                <span v-if="g.clusterRole" class="badge" :class="roleClass(g.clusterRole)">{{ g.clusterRole }}</span>
                <span v-else-if="g.customRole && !g.namespaceBindings?.length" class="badge badge-purple">custom cluster</span>
                <span v-else-if="g.namespaceBindings?.length" class="badge badge-info">{{ g.namespaceBindings.length }} namespace{{ g.namespaceBindings.length !== 1 ? 's' : '' }}</span>
                <span v-else class="text-muted text-sm">none</span>
              </td>
              <td class="text-muted text-sm">{{ fmtDate(g.createdAt) }}</td>
              <td v-if="isAdmin" style="text-align:right;white-space:nowrap">
                <button class="btn btn-ghost btn-sm" style="margin-right:4px" @click="openEdit(g)">Edit</button>
                <button v-if="g.clusterRole || g.customRole || g.namespaceBindings?.length"
                  class="btn btn-ghost btn-sm" style="margin-right:4px" @click="doSync(g.name)"
                  :disabled="syncing === g.name" title="Recreate missing k8s RBAC objects from database">
                  <span v-if="syncing === g.name" class="spinner" />
                  <span v-else>Sync</span>
                </button>
                <button class="btn btn-danger btn-sm" @click="confirmDelete(g)">Delete</button>
              </td>
              <td v-else></td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- Create / Edit modal -->
    <div v-if="modal.open" class="modal-overlay" @mousedown.self="closeModal">
      <div class="modal" style="max-width:640px">
        <div class="modal-header">
          <span>{{ modal.editing ? 'Edit Group' : 'New Group' }}</span>
          <button class="btn btn-ghost btn-sm btn-icon" @click="closeModal">✕</button>
        </div>
        <div class="modal-body">
          <div v-if="modal.error" class="alert alert-error">{{ modal.error }}</div>

          <!-- Name (create only) -->
          <div v-if="!modal.editing" class="form-group">
            <label class="form-label">Name <span class="required">*</span></label>
            <input v-model="modal.name" type="text" class="form-input"
              placeholder="e.g. backend-devs" pattern="[a-z0-9][a-z0-9\-]*" required />
            <p class="form-hint">Lowercase letters, numbers and hyphens. Used as k8s Group subject.</p>
          </div>

          <div class="form-group">
            <label class="form-label">Description</label>
            <input v-model="modal.description" type="text" class="form-input" placeholder="Optional description" />
          </div>

          <!-- RBAC scope -->
          <div class="form-group">
            <label class="form-label">Access scope</label>
            <div class="radio-group">
              <label class="radio-option">
                <input type="radio" v-model="modal.bindingType" value="none" />
                None (define later)
              </label>
              <label class="radio-option">
                <input type="radio" v-model="modal.bindingType" value="cluster" @change="modal.advanced = false" />
                Cluster-wide
              </label>
              <label class="radio-option">
                <input type="radio" v-model="modal.bindingType" value="namespace" @change="modal.advanced = false" />
                Namespace-scoped
              </label>
            </div>
          </div>

          <!-- Cluster role / advanced -->
          <div v-if="modal.bindingType === 'cluster'" class="form-group">
            <div style="display:flex;align-items:center;justify-content:space-between;margin-bottom:6px">
              <label class="form-label" style="margin:0">
                {{ modal.advanced ? 'Custom rules' : 'Cluster Role' }}
              </label>
              <button type="button" class="btn btn-ghost btn-sm" style="font-size:12px;padding:2px 8px"
                @click="modal.advanced = !modal.advanced; if(modal.advanced){ modal.clusterRole=''; modal.rules=[emptyRule()] }">
                {{ modal.advanced ? '← Simple' : 'Advanced →' }}
              </button>
            </div>
            <template v-if="!modal.advanced">
              <select v-model="modal.clusterRole" class="form-select">
                <option value="">— select —</option>
                <option value="cluster-admin">cluster-admin</option>
                <option value="admin">admin</option>
                <option value="edit">edit</option>
                <option value="view">view</option>
              </select>
            </template>
            <template v-else>
              <div v-for="(rule, i) in modal.rules" :key="i" class="rule-card">
                <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:10px">
                  <span style="font-size:12px;font-weight:600;color:var(--text-muted)">Rule {{ i + 1 }}</span>
                  <button type="button" class="btn btn-ghost btn-sm" style="padding:2px 6px;color:var(--danger)"
                    @click="modal.rules.splice(i,1)" v-if="modal.rules.length > 1">×</button>
                </div>
                <div class="form-group" style="margin-bottom:8px">
                  <label class="form-label" style="font-size:11px">API Groups</label>
                  <input v-model="rule.apiGroups" type="text" class="form-input" style="font-size:12px;font-family:monospace" placeholder='e.g. apps' />
                </div>
                <div class="form-group" style="margin-bottom:8px">
                  <label class="form-label" style="font-size:11px">Resources</label>
                  <input v-model="rule.resources" type="text" class="form-input" style="font-size:12px;font-family:monospace" placeholder="e.g. pods, deployments" />
                </div>
                <div class="form-group" style="margin-bottom:0">
                  <label class="form-label" style="font-size:11px">Verbs</label>
                  <div style="display:flex;flex-wrap:wrap;gap:6px;margin-bottom:6px">
                    <label v-for="v in COMMON_VERBS" :key="v" style="display:flex;align-items:center;gap:4px;font-size:12px;cursor:pointer;user-select:none">
                      <input type="checkbox" :checked="rule.verbs.includes(v)" @change="toggleVerb(rule, v)" />{{ v }}
                    </label>
                  </div>
                  <input v-model="rule.verbCustom" type="text" class="form-input" style="font-size:12px;font-family:monospace" placeholder="Extra verbs (comma-separated)" />
                </div>
              </div>
              <button type="button" class="btn btn-ghost btn-sm" style="margin-top:8px;font-size:12px" @click="modal.rules.push(emptyRule())">+ Add rule</button>
            </template>
          </div>

          <!-- Namespace bindings -->
          <template v-if="modal.bindingType === 'namespace'">
            <div v-for="(nb, ni) in modal.nsBindings" :key="ni" class="rule-card">
              <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:10px">
                <span style="font-size:12px;font-weight:600;color:var(--text-muted)">Namespace binding {{ ni + 1 }}</span>
                <button type="button" class="btn btn-ghost btn-sm" style="padding:2px 6px;color:var(--danger)"
                  @click="modal.nsBindings.splice(ni,1)" v-if="modal.nsBindings.length > 1">×</button>
              </div>
              <div class="form-group" style="margin-bottom:8px">
                <label class="form-label" style="font-size:11px">Namespace <span class="required">*</span></label>
                <input v-model="nb.namespace" type="text" class="form-input" placeholder="e.g. default" />
              </div>
              <div class="form-group" style="margin-bottom:0">
                <div style="display:flex;align-items:center;justify-content:space-between;margin-bottom:6px">
                  <label class="form-label" style="font-size:11px;margin:0">{{ nb.advanced ? 'Custom rules' : 'Role' }}</label>
                  <button type="button" class="btn btn-ghost btn-sm" style="font-size:11px;padding:2px 6px"
                    @click="nb.advanced = !nb.advanced">
                    {{ nb.advanced ? '← Simple' : 'Advanced →' }}
                  </button>
                </div>
                <select v-if="!nb.advanced" v-model="nb.role" class="form-select">
                  <option value="">— select —</option>
                  <option value="admin">admin</option>
                  <option value="edit">edit</option>
                  <option value="view">view</option>
                </select>
                <template v-else>
                  <div v-for="(rule, ri) in nb.rules" :key="ri" class="rule-card" style="margin-bottom:8px;background:var(--bg)">
                    <div style="display:flex;justify-content:space-between;margin-bottom:8px">
                      <span style="font-size:11px;font-weight:600;color:var(--text-muted)">Rule {{ ri + 1 }}</span>
                      <button type="button" class="btn btn-ghost btn-sm" style="padding:1px 5px;color:var(--danger)"
                        @click="nb.rules.splice(ri,1)" v-if="nb.rules.length > 1">×</button>
                    </div>
                    <div class="form-group" style="margin-bottom:6px">
                      <label class="form-label" style="font-size:11px">API Groups</label>
                      <input v-model="rule.apiGroups" type="text" class="form-input" style="font-size:12px;font-family:monospace" placeholder='e.g. apps' />
                    </div>
                    <div class="form-group" style="margin-bottom:6px">
                      <label class="form-label" style="font-size:11px">Resources</label>
                      <input v-model="rule.resources" type="text" class="form-input" style="font-size:12px;font-family:monospace" placeholder="e.g. pods, deployments" />
                    </div>
                    <div class="form-group" style="margin-bottom:0">
                      <label class="form-label" style="font-size:11px">Verbs</label>
                      <div style="display:flex;flex-wrap:wrap;gap:6px;margin-bottom:6px">
                        <label v-for="v in COMMON_VERBS" :key="v" style="display:flex;align-items:center;gap:4px;font-size:12px;cursor:pointer">
                          <input type="checkbox" :checked="rule.verbs.includes(v)" @change="toggleVerb(rule, v)" />{{ v }}
                        </label>
                      </div>
                      <input v-model="rule.verbCustom" type="text" class="form-input" style="font-size:12px;font-family:monospace" placeholder="Extra verbs (comma-separated)" />
                    </div>
                  </div>
                  <button type="button" class="btn btn-ghost btn-sm" style="font-size:11px" @click="nb.rules.push(emptyRule())">+ Add rule</button>
                </template>
              </div>
            </div>
            <button type="button" class="btn btn-ghost btn-sm" style="margin-bottom:8px;font-size:12px"
              @click="modal.nsBindings.push(emptyNsBinding())">+ Add namespace binding</button>
          </template>
        </div>
        <div class="modal-footer">
          <button class="btn btn-ghost" @click="closeModal">Cancel</button>
          <button class="btn btn-primary" :disabled="modal.saving" @click="saveModal">
            <span v-if="modal.saving" class="spinner" />
            <span v-else>{{ modal.editing ? 'Save' : 'Create' }}</span>
          </button>
        </div>
      </div>
    </div>

    <!-- Delete confirm modal -->
    <div v-if="delTarget" class="modal-overlay" @mousedown.self="delTarget = null">
      <div class="modal" style="max-width:420px">
        <div class="modal-header">Delete group</div>
        <div class="modal-body">
          <p style="font-size:13.5px">Delete group <strong class="font-mono">{{ delTarget.name }}</strong>?
          All k8s bindings for this group will be removed. Users with this group in their cert will lose those permissions.</p>
          <div v-if="delError" class="alert alert-error" style="margin-top:12px">{{ delError }}</div>
        </div>
        <div class="modal-footer">
          <button class="btn btn-ghost" @click="delTarget = null; delError = ''">Cancel</button>
          <button class="btn btn-danger" :disabled="deleting" @click="doDelete">
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
import { useAuth } from '@/composables/useAuth'
import { useCluster } from '@/composables/useCluster'
import { listGroups, createGroup, updateGroup, deleteGroup, syncGroup, type Group } from '@/api/groups'
import type { NamespaceBinding, PolicyRule } from '@/api/users'

const COMMON_VERBS = ['get', 'list', 'watch', 'create', 'update', 'patch', 'delete']

interface RuleDraft { apiGroups: string; resources: string; verbs: string[]; verbCustom: string }
interface NsBindingDraft { namespace: string; role: string; advanced: boolean; rules: RuleDraft[] }

function emptyRule(): RuleDraft { return { apiGroups: '', resources: '', verbs: [], verbCustom: '' } }
function emptyNsBinding(): NsBindingDraft { return { namespace: '', role: '', advanced: false, rules: [emptyRule()] } }

function draftToRule(r: RuleDraft): PolicyRule {
  const apiGroups = r.apiGroups ? r.apiGroups.split(',').map(s => s.trim()) : ['']
  const resources = r.resources.split(',').map(s => s.trim()).filter(Boolean)
  const customVerbs = r.verbCustom.split(',').map(s => s.trim()).filter(Boolean)
  return { apiGroups, resources, verbs: [...new Set([...r.verbs, ...customVerbs])] }
}

function toggleVerb(rule: RuleDraft, verb: string) {
  const idx = rule.verbs.indexOf(verb)
  if (idx === -1) rule.verbs.push(verb)
  else rule.verbs.splice(idx, 1)
}

function ruleToDraft(r: PolicyRule): RuleDraft {
  return {
    apiGroups: r.apiGroups.join(', '),
    resources: r.resources.join(', '),
    verbs: r.verbs,
    verbCustom: '',
  }
}

function nbToDraft(nb: NamespaceBinding): NsBindingDraft {
  return {
    namespace: nb.namespace,
    role: nb.role ?? '',
    advanced: !!nb.rules?.length,
    rules: nb.rules?.length ? nb.rules.map(ruleToDraft) : [emptyRule()],
  }
}

const groups   = ref<Group[]>([])
const { isAdmin } = useAuth()
const { currentID } = useCluster()

const loading  = ref(true)
const loadError = ref('')

onMounted(async () => {
  try {
    groups.value = await listGroups(currentID.value!)
  } catch (e: any) {
    loadError.value = e.response?.data?.error ?? 'Failed to load groups'
  } finally {
    loading.value = false
  }
})

// ── Modal ────────────────────────────────────────────────────────
const modal = reactive({
  open: false,
  editing: false,
  editingName: '',
  saving: false,
  error: '',
  name: '',
  description: '',
  bindingType: 'none' as 'none' | 'cluster' | 'namespace',
  advanced: false,
  clusterRole: '',
  rules: [emptyRule()] as RuleDraft[],
  nsBindings: [emptyNsBinding()] as NsBindingDraft[],
})

function openCreate() {
  Object.assign(modal, {
    open: true, editing: false, editingName: '', saving: false, error: '',
    name: '', description: '', bindingType: 'none', advanced: false,
    clusterRole: '', rules: [emptyRule()], nsBindings: [emptyNsBinding()],
  })
}

function openEdit(g: Group) {
  let bindingType: 'none' | 'cluster' | 'namespace' = 'none'
  let advanced = false
  let clusterRole = ''
  let rules = [emptyRule()] as RuleDraft[]
  let nsBindings = [emptyNsBinding()] as NsBindingDraft[]

  if (g.clusterRole) {
    bindingType = 'cluster'
    clusterRole = g.clusterRole
  } else if (g.customRole && !g.namespaceBindings?.length) {
    bindingType = 'cluster'
    advanced = true
    rules = g.rules?.length ? g.rules.map(ruleToDraft) : [emptyRule()]
  } else if (g.namespaceBindings?.length) {
    bindingType = 'namespace'
    nsBindings = g.namespaceBindings.map(nbToDraft)
  }

  Object.assign(modal, {
    open: true, editing: true, editingName: g.name, saving: false, error: '',
    name: g.name, description: g.description ?? '',
    bindingType, advanced, clusterRole, rules, nsBindings,
  })
}

function closeModal() {
  modal.open = false
}

async function saveModal() {
  modal.saving = true
  modal.error = ''
  try {
    const payload: any = {
      description: modal.description,
    }
    if (modal.bindingType === 'cluster') {
      if (modal.advanced) {
        payload.rules = modal.rules.map(draftToRule)
      } else {
        payload.clusterRole = modal.clusterRole
      }
    } else if (modal.bindingType === 'namespace') {
      payload.namespaceBindings = modal.nsBindings.map(nb => ({
        namespace: nb.namespace,
        ...(nb.advanced ? { rules: nb.rules.map(draftToRule) } : { role: nb.role }),
      }))
    }

    if (modal.editing) {
      await updateGroup(modal.editingName, payload, currentID.value!)
      const idx = groups.value.findIndex(g => g.name === modal.editingName)
      if (idx !== -1) {
        groups.value[idx] = { ...groups.value[idx], ...payload }
      }
    } else {
      payload.name = modal.name
      const created = await createGroup(payload, currentID.value!)
      groups.value.push(created)
    }
    closeModal()
  } catch (e: any) {
    modal.error = e.response?.data?.error ?? 'Save failed'
  } finally {
    modal.saving = false
  }
}

// ── Delete ───────────────────────────────────────────────────────
const syncing   = ref('')
const syncMsg   = ref('')

async function doSync(name: string) {
  syncing.value = name
  try {
    const res = await syncGroup(name, currentID.value!)
    const repaired = res.repaired?.join(', ') || 'nothing missing'
    syncMsg.value = `Sync ${name}: ${repaired}`
    setTimeout(() => { syncMsg.value = '' }, 4000)
  } catch (e: any) {
    loadError.value = e.response?.data?.error ?? 'Sync failed'
    setTimeout(() => { loadError.value = '' }, 4000)
  } finally {
    syncing.value = ''
  }
}

const delTarget = ref<Group | null>(null)
const deleting  = ref(false)
const delError  = ref('')

function confirmDelete(g: Group) {
  delTarget.value = g
  delError.value = ''
}

async function doDelete() {
  if (!delTarget.value) return
  deleting.value = true
  delError.value = ''
  try {
    await deleteGroup(delTarget.value.name, currentID.value!)
    groups.value = groups.value.filter(g => g.name !== delTarget.value!.name)
    delTarget.value = null
  } catch (e: any) {
    delError.value = e.response?.data?.error ?? 'Delete failed'
  } finally {
    deleting.value = false
  }
}

// ── Helpers ──────────────────────────────────────────────────────
function roleClass(role: string): string {
  if (role === 'cluster-admin') return 'badge-danger'
  if (role === 'admin')         return 'badge-warning'
  if (role === 'edit')          return 'badge-primary-soft'
  if (role === 'view')          return 'badge-gray'
  return 'badge-gray'
}

function fmtDate(s: string): string {
  return new Date(s).toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric' })
}

</script>
