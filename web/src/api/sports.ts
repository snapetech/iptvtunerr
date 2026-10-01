import { api } from './client'

export interface SportsDataset {
  id: 'mlb' | 'nfl' | 'ncaa'
  name: string
  product: string
  host: string
  remote_league_id: number
  guide_block: number
}

export interface SportsTeamOption {
  dataset: string
  team: string
}

export interface SportsTeamRule {
  dataset: string
  team: string
}

export interface SportsAutomationSettings {
  version: number
  enabled: boolean
  datasets: string[]
  team_rules: SportsTeamRule[]
  past_hours: number
  lookahead_days: number
  max_channels_per_event: number
  updated_at?: string
}

export interface SportsQuota {
  daily_limit?: number
  daily_remaining?: number
  minute_limit?: number
  minute_remaining?: number
}

export interface SportsDatasetStatus {
  id: string
  cached_dates: string[]
  cached_event_count: number
  last_fetch_at?: string
  last_fetch_event_count: number
  stale_date_count: number
}

export interface SportsStatus {
  provider: string
  configured: boolean
  last_request_at?: string
  last_success_at?: string
  last_error?: string
  quota: SportsQuota
  datasets: SportsDatasetStatus[]
}

export interface SportsEvent {
  id: string
  dataset: string
  league?: string
  home_team: string
  home_team_code?: string
  home_team_id?: string
  away_team: string
  away_team_code?: string
  away_team_id?: string
  starts_at: string
  status?: string
  status_code?: string
}

export interface SportsEventChannel {
  id: string
  event_id: string
  dataset: string
  name: string
  guide_id: string
  guide_number: string
  starts_at: string
  stops_at: string
  stream_channel_id: string
  source_channel_name: string
  source_guide_number?: string
  source_programme: string
  match_confidence: 'high' | 'medium'
  play_url: string
}

export interface SportsEventReport {
  event: SportsEvent
  matched: boolean
  reason?: string
  channels?: SportsEventChannel[]
}

export interface SportsAutomationReport {
  generated_at: string
  enabled: boolean
  guide_ready: boolean
  schedule_event_count: number
  matched_event_count: number
  unmatched_event_count: number
  generated_count: number
  warnings?: string[]
  events: SportsEventReport[]
  channels: SportsEventChannel[]
}

export interface SportsAutomationView {
  generated_at: string
  configured: boolean
  settings_writable: boolean
  api_key_configured: boolean
  datasets: SportsDataset[]
  available_teams: SportsTeamOption[]
  settings: SportsAutomationSettings
  status: SportsStatus
  report: SportsAutomationReport
}

export const sportsApi = {
  automation: () => api.get<SportsAutomationView>('/api/v1/sports/automation'),
  save: (settings: SportsAutomationSettings) =>
    api.patch<{ ok: boolean; settings: SportsAutomationSettings; view: SportsAutomationView }>(
      '/api/v1/sports/automation', settings,
    ),
  refresh: () => api.post<{ ok: boolean; warning?: string; report: SportsAutomationReport; status: SportsStatus }>(
    '/api/v1/sports/refresh',
  ),
  events: () => api.get<SportsAutomationReport>('/api/v1/sports/events'),
  playlistURL: '/api/sports/live.m3u',
  guideURL: '/api/sports/guide.xml',
}
