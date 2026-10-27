export interface HealthResponse {
  status: 'ok' | 'degraded' | 'error'
  service: string
  version: string
}
