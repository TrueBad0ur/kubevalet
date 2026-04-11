<template>
  <AppLayout title="New User">
    <template #actions>
      <RouterLink to="/" class="btn btn-ghost">← Back</RouterLink>
    </template>

    <div style="max-width:560px">
      <div class="card">
        <div class="card-body">
          <div v-if="error" class="alert alert-error">{{ error }}</div>

          <form @submit.prevent="submit">
            <!-- Name -->
            <div class="form-group">
              <label class="form-label">Username <span class="required">*</span></label>
              <input v-model="form.name" type="text" class="form-input"
                placeholder="e.g. john-doe" required pattern="[a-z0-9][a-z0-9\-]*" />
              <p class="form-hint">Lowercase letters, numbers and hyphens. Maps to CN in the x509 cert.</p>
            </div>

            <!-- Groups -->
            <div class="form-group" style="position:relative">
              <label class="form-label">Groups</label>
              <div class="tags-input" @click="focusGroupInput">
                <span v-for="g in form.groups" :key="g" class="tag">
                  {{ g }}
                  <button type="button" @click.stop="removeGroup(g)">×</button>
                </span>
                <input
                  ref="groupInput"
                  v-model="groupDraft"
                  type="text"
                  placeholder="Type or select…"
                  autocomplete="off"
                  @keydown.enter.prevent="dropdownActive >= 0 ? pickSuggestion(groupSuggestions[dropdownActive]) : addGroup()"
                  @keydown.tab.prevent="dropdownActive >= 0 ? pickSuggestion(groupSuggestions[dropdownActive]) : addGroup()"
                  @keydown.backspace="onBackspace"
                  @keydown.comma.prevent="addGroup"
                  @keydown.arrow-down.prevent="dropdownActive = Math.min(dropdownActive + 1, groupSuggestions.length - 1)"
                  @keydown.arrow-up.prevent="dropdownActive = Math.max(dropdownActive - 1, 0)"
                  @keydown.escape="groupDraft = ''"
                  @blur="onGroupBlur"
                  @focus="groupFocused = true; dropdownActive = -1"
                />
              </div>
              <!-- suggestions dropdown -->
              <div v-if="groupSuggestions.length" class="group-suggestions">
                <button
                  v-for="(g, i) in groupSuggestions" :key="g.name"
                  type="button"
                  class="group-suggestion-item"
                  :class="{ active: i === dropdownActive }"
                  @mousedown.prevent="pickSuggestion(g)"
                >
                  <span class="font-mono" style="font-size:13px;font-weight:600">{{ g.name }}</span>
                  <span v-if="g.description" class="text-muted text-sm" style="margin-left:8px">{{ g.description }}</span>
                  <span v-if="form.groups.includes(g.name)" class="badge badge-success" style="margin-left:auto;font-size:10px">added</span>
                </button>
              </div>
              <p class="form-hint">Maps to O in the cert. Used in RBAC group bindings.</p>
            </div>

            <!-- Binding type -->
            <div class="form-group">
              <label class="form-label">Access scope <span class="required">*</span></label>
              <div class="radio-group">
                <label class="radio-option">
                  <input type="radio" v-model="bindingType" value="cluster" @change="advanced = false" />
                  Cluster-wide
                </label>
                <label class="radio-option">
                  <input type="radio" v-model="bindingType" value="namespace" @change="advanced = false" />
                  Namespace-scoped
                </label>
              </div>
            </div>

            <!-- Cluster role / advanced -->
            <div v-if="bindingType === 'cluster'" class="form-group">
              <div style="display:flex;align-items:center;justify-content:space-between;margin-bottom:6px">
                <label class="form-label" style="margin:0">
                  {{ advanced ? 'Custom rules' : 'Cluster Role' }}
                  <span v-if="!advanced" class="required">*</span>
                </label>
                <button type="button" class="btn btn-ghost btn-sm" style="font-size:12px;padding:2px 8px"
                  @click="toggleAdvanced">
                  {{ advanced ? '← Simple' : 'Advanced →' }}
                </button>
              </div>
              <template v-if="!advanced">
                <select v-model="form.clusterRole" class="form-select" required>
                  <option value="">— select —</option>
                  <option value="cluster-admin">cluster-admin</option>
                  <option value="admin">admin</option>
                  <option value="edit">edit</option>
                  <option value="view">view</option>
                </select>
              </template>

              <!-- Advanced rule builder -->
              <template v-else>
                <div v-for="(rule, i) in rules" :key="i" class="rule-card">
                  <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:10px">
                    <span style="font-size:12px;font-weight:600;color:var(--text-muted)">Rule {{ i + 1 }}</span>
                    <button type="button" class="btn btn-ghost btn-sm" style="padding:2px 6px;color:var(--danger)"
                      @click="removeRule(i)" v-if="rules.length > 1">×</button>
                  </div>
                  <div class="form-group" style="margin-bottom:8px">
                    <label class="form-label" style="font-size:11px">API Groups <span style="color:var(--text-muted);font-weight:400">(comma-separated; empty = core)</span></label>
                    <input v-model="rule.apiGroups" type="text" class="form-input" style="font-size:12px;font-family:monospace"
                      placeholder='e.g. apps, "" for core' />
                  </div>
                  <div class="form-group" style="margin-bottom:8px">
                    <label class="form-label" style="font-size:11px">Resources <span style="color:var(--text-muted);font-weight:400">(comma-separated)</span></label>
                    <input v-model="rule.resources" type="text" class="form-input" style="font-size:12px;font-family:monospace"
                      placeholder="e.g. pods, deployments, secrets" required />
                  </div>
                  <div class="form-group" style="margin-bottom:0">
                    <label class="form-label" style="font-size:11px">Verbs</label>
                    <div style="display:flex;flex-wrap:wrap;gap:6px;margin-bottom:6px">
                      <label v-for="v in COMMON_VERBS" :key="v" style="display:flex;align-items:center;gap:4px;font-size:12px;cursor:pointer;user-select:none">
                        <input type="checkbox" :checked="rule.verbs.includes(v)" @change="toggleVerb(rule, v)" />
                        {{ v }}
                      </label>
                    </div>
                    <input v-model="rule.verbCustom" type="text" class="form-input" style="font-size:12px;font-family:monospace"
                      placeholder="Extra verbs (comma-separated, e.g. watch, deletecollection)" />
                  </div>
                </div>
                <button type="button" class="btn btn-ghost btn-sm" style="margin-top:8px;font-size:12px" @click="addRule">
                  + Add rule
                </button>
              </template>
            </div>

            <!-- Namespace bindings (multi) -->
            <template v-if="bindingType === 'namespace'">
              <div v-for="(nb, ni) in nsBindings" :key="ni" class="rule-card">
                <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:10px">
                  <span style="font-size:12px;font-weight:600;color:var(--text-muted)">Namespace binding {{ ni + 1 }}</span>
                  <button type="button" class="btn btn-ghost btn-sm" style="padding:2px 6px;color:var(--danger)"
                    @click="nsBindings.splice(ni,1)" v-if="nsBindings.length > 1">×</button>
                </div>
                <div class="form-group" style="margin-bottom:8px">
                  <label class="form-label" style="font-size:11px">Namespace <span class="required">*</span></label>
                  <input v-model="nb.namespace" type="text" class="form-input" placeholder="e.g. default" required />
                </div>
                <div class="form-group" style="margin-bottom:0">
                  <div style="display:flex;align-items:center;justify-content:space-between;margin-bottom:6px">
                    <label class="form-label" style="font-size:11px;margin:0">
                      {{ nb.advanced ? 'Custom rules' : 'Role' }}
                      <span v-if="!nb.advanced" class="required">*</span>
                    </label>
                    <button type="button" class="btn btn-ghost btn-sm" style="font-size:11px;padding:2px 6px"
                      @click="nb.advanced = !nb.advanced">
                      {{ nb.advanced ? '← Simple' : 'Advanced →' }}
                    </button>
                  </div>
                  <select v-if="!nb.advanced" v-model="nb.role" class="form-select" required>
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
                        <input v-model="rule.resources" type="text" class="form-input" style="font-size:12px;font-family:monospace" placeholder="e.g. pods, deployments" required />
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
              <button type="button" class="btn btn-ghost btn-sm" style="margin-bottom:16px;font-size:12px"
                @click="nsBindings.push(emptyNsBinding())">+ Add namespace binding</button>
            </template>

            <button type="submit" class="btn btn-primary" :disabled="loading">
              <span v-if="loading" class="spinner" />
              <span v-else>Create User</span>
            </button>
          </form>
        </div>
      </div>
    </div>

    <!-- Kubeconfig result modal -->
    <div v-if="result" class="modal-overlay">
      <div class="modal">
        <div class="modal-header">
          <span>User <strong class="font-mono">{{ result.user.name }}</strong> created</span>
        </div>
        <div class="modal-body">
          <p style="margin-bottom:12px;font-size:13.5px">
            Copy or download the kubeconfig. The private key is stored in a cluster Secret —
            you can re-download from the users list anytime.
          </p>
          <textarea readonly class="code-block" style="width:100%;resize:none;border:none;outline:none;cursor:text"
            :rows="result.kubeconfig.split('\n').length">{{ result.kubeconfig }}</textarea>
        </div>
        <div class="modal-footer">
          <button class="btn btn-ghost" @click="copyKubeconfig">
            {{ copied ? '✓ Copied' : 'Copy' }}
          </button>
          <a :href="kubeconfigDownloadHref" :download="`${result.user.name}.kubeconfig`"
             class="btn btn-primary">
            Download
          </a>
          <RouterLink to="/" class="btn btn-ghost">Close</RouterLink>
        </div>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { reactive, ref, computed, onMounted } from 'vue'
import { RouterLink } from 'vue-router'
import AppLayout from '@/components/AppLayout.vue'
import { createUser, type CreateUserResponse, type PolicyRule, type NamespaceBinding } from '@/api/users'
import { listGroups, type Group } from '@/api/groups'



const COMMON_VERBS = ['get', 'list', 'watch', 'create', 'update', 'patch', 'delete']

interface RuleDraft {
  apiGroups: string
  resources: string
  verbs: string[]
  verbCustom: string
}

interface NsBindingDraft {
  namespace: string
  role: string
  advanced: boolean
  rules: RuleDraft[]
}

function emptyRule(): RuleDraft {
  return { apiGroups: '', resources: '', verbs: [], verbCustom: '' }
}

function emptyNsBinding(): NsBindingDraft {
  return { namespace: '', role: '', advanced: false, rules: [emptyRule()] }
}

function draftToRule(r: RuleDraft): PolicyRule {
  const apiGroups = r.apiGroups
    ? r.apiGroups.split(',').map(s => s.trim())
    : ['']
  const resources = r.resources.split(',').map(s => s.trim()).filter(Boolean)
  const customVerbs = r.verbCustom.split(',').map(s => s.trim()).filter(Boolean)
  const verbs = [...new Set([...r.verbs, ...customVerbs])]
  return { apiGroups, resources, verbs }
}

const form = reactive({
  name: '',
  groups: [] as string[],
  clusterRole: '',
  namespace: '',
  role: '',
})

const bindingType = ref<'cluster' | 'namespace'>('cluster')
const advanced    = ref(false)
const rules       = ref<RuleDraft[]>([emptyRule()])
const nsBindings  = ref<NsBindingDraft[]>([emptyNsBinding()])

function toggleAdvanced() {
  if (bindingType.value !== 'cluster') return
  advanced.value = !advanced.value
  if (advanced.value) {
    form.clusterRole = ''
    rules.value = [emptyRule()]
  }
}

function addRule() { rules.value.push(emptyRule()) }
function removeRule(i: number) { rules.value.splice(i, 1) }

function toggleVerb(rule: RuleDraft, verb: string) {
  const idx = rule.verbs.indexOf(verb)
  if (idx === -1) rule.verbs.push(verb)
  else rule.verbs.splice(idx, 1)
}
const groupDraft      = ref('')
const groupInput      = ref<HTMLInputElement | null>(null)
const loading         = ref(false)
const error           = ref('')
const result          = ref<CreateUserResponse | null>(null)
const copied          = ref(false)
const availableGroups = ref<Group[]>([])
const dropdownActive  = ref(-1)

const groupFocused = ref(false)

const groupSuggestions = computed(() => {
  if (!groupFocused.value || !availableGroups.value.length) return []
  const q = groupDraft.value.trim().toLowerCase()
  return availableGroups.value.filter(g =>
    !q || g.name.toLowerCase().includes(q) || g.description?.toLowerCase().includes(q)
  )
})

onMounted(async () => {
  try { availableGroups.value = await listGroups() } catch { /* ignore */ }
})

function focusGroupInput() { groupInput.value?.focus() }

function addGroup() {
  const val = groupDraft.value.trim()
  if (val && !form.groups.includes(val)) form.groups.push(val)
  groupDraft.value = ''
  dropdownActive.value = -1
}

function pickSuggestion(g: Group) {
  if (!form.groups.includes(g.name)) form.groups.push(g.name)
  groupDraft.value = ''
  dropdownActive.value = -1
  groupInput.value?.focus()
}

function onGroupBlur() {
  // small delay so mousedown on suggestion fires first
  setTimeout(() => {
    groupFocused.value = false
    addGroup()
  }, 150)
}

function removeGroup(g: string) {
  form.groups = form.groups.filter((x) => x !== g)
}

function onBackspace() {
  if (!groupDraft.value && form.groups.length) form.groups.pop()
}

async function submit() {
  addGroup() // flush any pending draft
  error.value = ''
  loading.value = true
  try {
    let payload
    if (bindingType.value === 'cluster') {
      payload = {
        name: form.name,
        groups: form.groups,
        ...(advanced.value
          ? { rules: rules.value.map(draftToRule) }
          : { clusterRole: form.clusterRole }),
      }
    } else {
      payload = {
        name: form.name,
        groups: form.groups,
        namespaceBindings: nsBindings.value.map(nb => ({
          namespace: nb.namespace,
          ...(nb.advanced
            ? { rules: nb.rules.map(draftToRule) }
            : { role: nb.role }),
        })) as NamespaceBinding[],
      }
    }
    result.value = await createUser(payload)
  } catch (e: any) {
    error.value = e.response?.data?.error ?? 'Failed to create user'
  } finally {
    loading.value = false
  }
}

async function copyKubeconfig() {
  if (!result.value) return
  const text = result.value.kubeconfig
  try {
    await navigator.clipboard.writeText(text)
  } catch {
    // Fallback for non-HTTPS contexts
    const ta = document.createElement('textarea')
    ta.value = text
    ta.style.position = 'fixed'
    ta.style.opacity = '0'
    document.body.appendChild(ta)
    ta.focus()
    ta.select()
    document.execCommand('copy')
    document.body.removeChild(ta)
  }
  copied.value = true
  setTimeout(() => { copied.value = false }, 2000)
}

const kubeconfigDownloadHref = computed(() => {
  if (!result.value) return '#'
  const blob = new Blob([result.value.kubeconfig], { type: 'application/x-yaml' })
  return URL.createObjectURL(blob)
})
</script>
