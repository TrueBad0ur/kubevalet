import { client as api } from './client'
import type { PolicyRule, NamespaceBinding } from './users'

export interface RoleTemplate {
  id: number
  name: string
  description?: string
  clusterRole?: string
  customRole?: boolean
  rules?: PolicyRule[]
  namespaceBindings?: NamespaceBinding[]
  createdAt: string
}

export interface CreateTemplateRequest {
  name: string
  description?: string
  clusterRole?: string
  customRole?: boolean
  rules?: PolicyRule[]
  namespaceBindings?: NamespaceBinding[]
}

export async function listTemplates(): Promise<RoleTemplate[]> {
  const r = await api.get('/templates')
  return r.data.templates
}

export async function createTemplate(req: CreateTemplateRequest, overwrite = false): Promise<RoleTemplate> {
  const r = await api.post(`/templates${overwrite ? '?overwrite=true' : ''}`, req)
  return r.data
}

export async function deleteTemplate(id: number): Promise<void> {
  await api.delete(`/templates/${id}`)
}
