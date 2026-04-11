import { client } from './client'

export interface PolicyRule {
  apiGroups: string[]
  resources: string[]
  verbs: string[]
}

export interface NamespaceBinding {
  namespace: string
  role?: string
  customRole?: boolean
  rules?: PolicyRule[]
}

export interface User {
  name: string
  groups?: string[]
  clusterRole?: string
  customRole?: boolean
  rules?: PolicyRule[]
  namespaceBindings?: NamespaceBinding[]
  status: string
  createdAt: string
}

export interface CreateUserRequest {
  name: string
  groups: string[]
  clusterRole?: string
  rules?: PolicyRule[]
  namespaceBindings?: NamespaceBinding[]
}

export interface UpdateRBACRequest {
  groups?: string[]
  clusterRole?: string
  rules?: PolicyRule[]
  namespaceBindings?: NamespaceBinding[]
}

export interface UpdateRBACResponse {
  kubeconfig?: string
}

export interface CreateUserResponse {
  user: User
  kubeconfig: string
}

export async function listUsers(): Promise<User[]> {
  const res = await client.get<{ users: User[]; total: number }>('/users')
  return res.data.users ?? []
}

export async function createUser(req: CreateUserRequest): Promise<CreateUserResponse> {
  const res = await client.post<CreateUserResponse>('/users', req)
  return res.data
}

export async function updateUserRBAC(name: string, req: UpdateRBACRequest): Promise<UpdateRBACResponse> {
  const res = await client.put<UpdateRBACResponse>(`/users/${name}/rbac`, req)
  return res.data
}

export async function deleteUser(name: string): Promise<void> {
  await client.delete(`/users/${name}`)
}

export function kubeconfigUrl(name: string): string {
  return `/api/v1/users/${name}/kubeconfig`
}
