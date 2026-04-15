import { client } from './client'

export interface Settings {
  version: string
  clusterServer: string
  localUsersEnabled: boolean
}

export async function getSettings(): Promise<Settings> {
  const res = await client.get<Settings>('/settings')
  return res.data
}

export async function updateSettings(payload: { clusterServer: string }): Promise<void> {
  await client.put('/settings', payload)
}

export async function changePassword(currentPassword: string, newPassword: string): Promise<void> {
  await client.put('/settings/password', { currentPassword, newPassword })
}
