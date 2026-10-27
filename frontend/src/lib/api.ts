import { HealthResponse } from '@/types/health'

const API_BASE_URL = import.meta.env.VITE_API_BASE_URL || '/api'

export async function fetchHealth(signal?: AbortSignal): Promise<HealthResponse> {
  const response = await fetch(`${API_BASE_URL}/health`, {
    method: 'GET',
    headers: {
      Accept: 'application/json',
    },
    signal,
  })

  if (!response.ok) {
    throw new Error(`Health check failed with HTTP ${response.status}`)
  }

  return response.json()
}
