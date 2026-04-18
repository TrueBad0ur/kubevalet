import { client as api } from './client'
import type { PolicyRule, NamespaceBinding } from './users'

export interface Group {
  id: number
  name: string
  description: string
  clusterRole?: string
  customRole?: boolean
  rules?: PolicyRule[]
  namespaceBindings?: NamespaceBinding[]
  createdAt: string
}

export interface CreateGroupPayload {
  name: string
  description?: string
  clusterRole?: string
  rules?: PolicyRule[]
  namespaceBindings?: NamespaceBinding[]
}

export interface UpdateGroupPayload {
  description?: string
  clusterRole?: string
  rules?: PolicyRule[]
  namespaceBindings?: NamespaceBinding[]
}

function cp(clusterID: number) {
  return { params: { cluster_id: clusterID } }
}

export async function listGroups(clusterID: number): Promise<Group[]> {
  const res = await api.get<{ groups: Group[] }>('/groups', cp(clusterID))
  return res.data.groups
}

export async function createGroup(payload: CreateGroupPayload, clusterID: number): Promise<Group> {
  const res = await api.post<Group>('/groups', payload, cp(clusterID))
  return res.data
}

export async function updateGroup(name: string, payload: UpdateGroupPayload, clusterID: number): Promise<void> {
  await api.put(`/groups/${name}`, payload, cp(clusterID))
}

export async function deleteGroup(name: string, clusterID: number): Promise<void> {
  await api.delete(`/groups/${name}`, cp(clusterID))
}

export async function syncGroup(name: string, clusterID: number): Promise<{ repaired: string[] }> {
  const res = await api.post<{ repaired: string[] }>(`/groups/${name}/sync`, {}, cp(clusterID))
  return res.data
}
