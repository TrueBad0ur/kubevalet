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

export async function listGroups(): Promise<Group[]> {
  const res = await api.get<{ groups: Group[] }>('/groups')
  return res.data.groups
}

export async function createGroup(payload: CreateGroupPayload): Promise<Group> {
  const res = await api.post<Group>('/groups', payload)
  return res.data
}

export async function updateGroup(name: string, payload: UpdateGroupPayload): Promise<void> {
  await api.put(`/groups/${name}`, payload)
}

export async function deleteGroup(name: string): Promise<void> {
  await api.delete(`/groups/${name}`)
}
