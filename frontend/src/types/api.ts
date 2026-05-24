export type Country = {
  code: string
  name?: string
}

export type IssuedConfigSummary = {
  code: string
  name?: string
  worker_last_updated?: string
  last_downloaded?: string
  installation_uuid?: string
}

export type AccountSummary = {
  available_countries?: Country[]
  issued_country_configs?: IssuedConfigSummary[]
  active_device_count?: number
  max_device_count?: number
  subscription_end_date?: string
  subscription_description?: string
}

export type CountryState = {
  code: string
  name?: string
  worker_last_updated?: string
  last_downloaded?: string
  installation_uuid?: string
  status?: string
  last_change?: string
  message?: string
}

export type KeyState = {
  id: string
  name: string
  last_check?: string
  status?: string
  last_error?: string
  error_count?: number
  countries?: Record<string, CountryState>
  last_account?: AccountSummary
}

export type State = {
  last_check?: string
  status?: string
  last_error?: string
  error_count?: number
  countries?: Record<string, CountryState>
  keys?: Record<string, KeyState>
  last_account?: AccountSummary
}

export type KeyConfig = {
  id: string
  name: string
  vpn_key?: string
  countries?: string[]
}

export type Config = {
  listen_addr?: string
  keys?: KeyConfig[]
  poll_interval_hours?: number
  telegram?: {
    bot_token?: string
    chat_id?: string
    endpoint?: string
  }
  amnezia?: {
    gateway_endpoint?: string
    gateway_public_key_filepath?: string
  }
}

export type SetupRequirements = {
  admin_password?: boolean
  gateway_public_keys?: boolean
  amnezia_keys?: boolean
  gateway_key_path?: string
}

export type StatusPayload = {
  config?: Config
  state?: State
  next_check?: string | null
  setup_mode?: boolean
  setup_requirements?: SetupRequirements
  gateway_public_key_status?: Record<string, unknown>
  fixture?: boolean
}

export type SettingsPatch = {
  poll_interval_hours?: number
  telegram?: {
    bot_token?: string
    chat_id?: string
  }
  amnezia?: {
    gateway_endpoint?: string
    gateway_public_key_filepath?: string
  }
  web_password?: string
  gateway_public_keys?: string
  auto_select_issued_countries?: boolean
  keys?: KeyConfig[]
}

export type SettingsResponse = {
  ok: boolean
  config?: Config
  setup_complete?: boolean
  password_changed?: boolean
}
