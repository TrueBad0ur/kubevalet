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
  certExpiresAt?: string
}

export interface RenewCertificateResponse {
  kubeconfig: string
  certExpiresAt: string
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

function clusterParam(clusterID: number) {
  return { params: { cluster_id: clusterID } }
}

export async function listUsers(clusterID: number): Promise<User[]> {
  const res = await client.get<{ users: User[]; total: number }>('/users', clusterParam(clusterID))
  return res.data.users ?? []
}

export async function createUser(req: CreateUserRequest, clusterID: number): Promise<CreateUserResponse> {
  const res = await client.post<CreateUserResponse>('/users', req, clusterParam(clusterID))
  return res.data
}

export async function updateUserRBAC(name: string, req: UpdateRBACRequest, clusterID: number): Promise<UpdateRBACResponse> {
  const res = await client.put<UpdateRBACResponse>(`/users/${name}/rbac`, req, clusterParam(clusterID))
  return res.data
}

export async function deleteUser(name: string, clusterID: number): Promise<void> {
  await client.delete(`/users/${name}`, clusterParam(clusterID))
}

export function kubeconfigUrl(name: string, clusterID: number): string {
  return `/api/v1/users/${name}/kubeconfig?cluster_id=${clusterID}`
}

export async function syncUser(name: string, clusterID: number): Promise<{ repaired: string[] }> {
  const res = await client.post<{ repaired: string[] }>(`/users/${name}/sync`, {}, clusterParam(clusterID))
  return res.data
}

export async function renewCertificate(name: string, clusterID: number): Promise<RenewCertificateResponse> {
  const res = await client.post<RenewCertificateResponse>(`/users/${name}/renew`, {}, clusterParam(clusterID))
  return res.data
}
