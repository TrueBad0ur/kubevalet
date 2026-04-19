import { client } from './client'

export interface Settings {
  version: string
  clusterServer: string
  localUsersEnabled: boolean
}

export async function getSettings(clusterID: number): Promise<Settings> {
  const res = await client.get<Settings>('/settings', { params: { cluster_id: clusterID } })
  return res.data
}

export async function updateSettings(payload: { clusterServer: string }, clusterID: number): Promise<void> {
  await client.put('/settings', payload, { params: { cluster_id: clusterID } })
}

export async function changePassword(currentPassword: string, newPassword: string): Promise<void> {
  await client.put('/settings/password', { currentPassword, newPassword })
}
