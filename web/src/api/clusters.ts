import { client } from './client'

export interface Cluster {
  id: number
  name: string
  description?: string
  apiServer: string
  clusterName: string
  createdAt: string
}

export interface CreateClusterRequest {
  name: string
  description?: string
  kubeconfig: string
  apiServer: string
  clusterName?: string
}

export async function listClusters(): Promise<Cluster[]> {
  const res = await client.get<{ clusters: Cluster[] }>('/clusters')
  return res.data.clusters ?? []
}

export async function createCluster(req: CreateClusterRequest): Promise<Cluster> {
  const res = await client.post<Cluster>('/clusters', req)
  return res.data
}

export async function deleteCluster(id: number): Promise<void> {
  await client.delete(`/clusters/${id}`)
}
