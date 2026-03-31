export type RelayState = {
  apiBase: string;
  adminToken: string;
};

export type OverviewResponse = {
  instance: string;
  listen: string;
  summary: {
    requests: number;
    successes: number;
    failures: number;
    input_tokens: number;
    output_tokens: number;
    logical_cost: number;
    physical_cost: number;
    race_extra_cost: number;
    active_sessions: number;
  };
  providers: Array<{
    id: string;
    name: string;
    type: string;
    enabled: boolean;
    priority: number;
    health_score: number;
    tags: string[];
  }>;
};

async function fetchJSON<T>(path: string, state: RelayState): Promise<T> {
  const response = await fetch(`${state.apiBase}${path}`, {
    headers: {
      Authorization: `Bearer ${state.adminToken}`,
    },
    cache: "no-store",
  });
  if (!response.ok) {
    throw new Error(`Request failed: ${response.status}`);
  }
  return response.json() as Promise<T>;
}

export const relayApi = {
  overview: (state: RelayState) => fetchJSON<OverviewResponse>("/api/admin/overview", state),
  providers: (state: RelayState) => fetchJSON<{ items: ProviderItem[] }>("/api/admin/providers", state),
  models: (state: RelayState) => fetchJSON<{ items: ModelItem[] }>("/api/admin/models", state),
  rules: (state: RelayState) => fetchJSON<{ items: RuleItem[] }>("/api/admin/router/rules", state),
  requests: (state: RelayState) => fetchJSON<{ items: RequestItem[] }>("/api/admin/requests?limit=20", state),
  sessions: (state: RelayState) => fetchJSON<{ items: SessionItem[] }>("/api/admin/sessions", state),
  usage: (state: RelayState) => fetchJSON<OverviewResponse["summary"]>("/api/admin/usage/summary", state),
};

export type ProviderItem = {
  id: string;
  name: string;
  type: string;
  enabled: boolean;
  priority: number;
  tags: string[];
  health_score?: number;
};

export type ModelItem = {
  id: string;
  name: string;
  task_type: string;
  description: string;
  tags: string[];
  targets: Array<{
    provider_id: string;
    model: string;
    priority: number;
  }>;
};

export type RuleItem = {
  id: string;
  name: string;
  enabled: boolean;
  priority: number;
  policy: {
    strategy: string;
    max_candidates: number;
    winner: string;
  };
};

export type RequestItem = {
  request_id: string;
  logical_model: string;
  final_provider_id: string;
  status: string;
  route_strategy: string;
  physical_cost: number;
  logical_usage: {
    input_tokens: number;
    output_tokens: number;
    cost: number;
  };
  started_at: string;
  response_preview: string;
};

export type SessionItem = {
  session_key: string;
  project_id: string;
  provider_id: string;
  provider_model: string;
  last_seen_at: string;
};
