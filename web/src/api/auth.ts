import { client } from './client'

export interface MeResponse {
  username: string
}

export async function login(username: string, password: string): Promise<string> {
  const res = await client.post<{ token: string }>('/auth/login', { username, password })
  return res.data.token
}

export async function me(): Promise<MeResponse> {
  const res = await client.get<MeResponse>('/auth/me')
  return res.data
}
