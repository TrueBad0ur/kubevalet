import { client } from './client'

export interface LocalUser {
  id: number
  username: string
  createdAt: string
}

export async function listLocalUsers(): Promise<LocalUser[]> {
  const res = await client.get<{ users: LocalUser[]; total: number }>('/local-users')
  return res.data.users ?? []
}

export async function createLocalUser(username: string, password: string): Promise<LocalUser> {
  const res = await client.post<LocalUser>('/local-users', { username, password })
  return res.data
}

export async function deleteLocalUser(username: string): Promise<void> {
  await client.delete(`/local-users/${username}`)
}

export async function resetLocalUserPassword(username: string, password: string): Promise<void> {
  await client.put(`/local-users/${username}/password`, { password })
}
