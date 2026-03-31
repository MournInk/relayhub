"use client";

import type { ReactNode } from "react";
import { useEffect, useMemo, useState } from "react";
import { Button, Card, Chip, Spinner } from "@heroui/react";

import {
  type ModelItem,
  type OverviewResponse,
  type ProviderItem,
  type RelayState,
  type RequestItem,
  type RuleItem,
  type SessionItem,
  relayApi,
} from "../lib/api";

type LoadState = {
  overview?: OverviewResponse;
  providers?: ProviderItem[];
  models?: ModelItem[];
  rules?: RuleItem[];
  requests?: RequestItem[];
  sessions?: SessionItem[];
  usage?: OverviewResponse["summary"];
};

type SectionKey = "dashboard" | "providers" | "models" | "rules" | "requests" | "sessions";

const defaultState: RelayState = {
  apiBase: "http://127.0.0.1:8080",
  adminToken: "relayhub-admin",
};

export function ConsoleDashboard() {
  const [state, setState] = useState<RelayState>(defaultState);
  const [loadState, setLoadState] = useState<LoadState>({});
  const [section, setSection] = useState<SectionKey>("dashboard");
  const [error, setError] = useState<string>("");
  const [loading, setLoading] = useState<boolean>(true);

  useEffect(() => {
    const saved = window.localStorage.getItem("relayhub-console");
    if (saved) {
      setState(JSON.parse(saved) as RelayState);
    }
  }, []);

  useEffect(() => {
    void load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [state.apiBase, state.adminToken]);

  async function load() {
    setLoading(true);
    setError("");
    try {
      const [overview, providers, models, rules, requests, sessions, usage] = await Promise.all([
        relayApi.overview(state),
        relayApi.providers(state),
        relayApi.models(state),
        relayApi.rules(state),
        relayApi.requests(state),
        relayApi.sessions(state),
        relayApi.usage(state),
      ]);
      setLoadState({
        overview,
        providers: providers.items,
        models: models.items,
        rules: rules.items,
        requests: requests.items,
        sessions: sessions.items,
        usage,
      });
      window.localStorage.setItem("relayhub-console", JSON.stringify(state));
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "Unknown error");
    } finally {
      setLoading(false);
    }
  }

  const requestSuccessRate = useMemo(() => {
    if (!loadState.usage || loadState.usage.requests === 0) return "0%";
    return `${Math.round((loadState.usage.successes / loadState.usage.requests) * 100)}%`;
  }, [loadState.usage]);

  const renderSection = () => {
    switch (section) {
      case "providers":
        return <ProvidersTable providers={loadState.providers ?? []} />;
      case "models":
        return <ModelsPanel models={loadState.models ?? []} />;
      case "rules":
        return <RulesTable rules={loadState.rules ?? []} />;
      case "requests":
        return <RequestsTable requests={loadState.requests ?? []} />;
      case "sessions":
        return <SessionsTable sessions={loadState.sessions ?? []} />;
      default:
        return <DashboardOverview overview={loadState.overview} />;
    }
  };

  return (
    <div className="relay-shell">
      <div className="relay-grid lg:grid-cols-[360px_minmax(0,1fr)]">
        <Card className="relay-panel rounded-[28px]">
          <Card.Header className="flex-col items-start gap-3">
            <p className="relay-gradient-text text-xs font-semibold uppercase tracking-[0.35em]">
              RelayHub
            </p>
            <div>
              <h1 className="text-4xl font-semibold tracking-tight">本地统一 AI 中转</h1>
              <p className="mt-3 text-sm text-slate-400">
                多 Provider 路由、竞速、会话粘性、用量追踪和治理台。
              </p>
            </div>
          </Card.Header>
          <Card.Content className="space-y-4 p-6 pt-0">
            <LabelledField
              label="Admin API Base"
              value={state.apiBase}
              onChange={(value) => setState((current) => ({ ...current, apiBase: value }))}
            />
            <LabelledField
              label="Admin Token"
              value={state.adminToken}
              onChange={(value) => setState((current) => ({ ...current, adminToken: value }))}
            />
            <Button variant="primary" onPress={() => void load()}>
              刷新控制台
            </Button>
            <hr className="border-white/10" />
            <div className="space-y-3 text-sm text-slate-300">
              <p>
                默认代理 Key：<code className="relay-mono rounded bg-white/10 px-2 py-1">relayhub-local-key</code>
              </p>
              <p>
                默认逻辑模型：<code className="relay-mono rounded bg-white/10 px-2 py-1">smart-fast</code> /{" "}
                <code className="relay-mono rounded bg-white/10 px-2 py-1">smart-budget</code>
              </p>
              <p>
                默认 Admin Token：<code className="relay-mono rounded bg-white/10 px-2 py-1">relayhub-admin</code>
              </p>
            </div>
            <div className="rounded-[22px] border border-white/10 bg-slate-950/40 p-4">
              <p className="mb-2 text-xs uppercase tracking-[0.25em] text-slate-500">Smoke Test</p>
              <pre className="relay-mono overflow-x-auto whitespace-pre-wrap text-xs text-slate-300">
{`curl -H "Authorization: Bearer relayhub-local-key" \\
  -H "Content-Type: application/json" \\
  -d '{"model":"smart-fast","messages":[{"role":"user","content":"写一个 hello world"}]}' \\
  ${state.apiBase}/v1/chat/completions`}
              </pre>
            </div>
          </Card.Content>
        </Card>

        <div className="relay-grid gap-5">
          <div className="grid gap-4 md:grid-cols-4">
            <MetricCard title="Requests" value={String(loadState.usage?.requests ?? 0)} detail={`Success ${requestSuccessRate}`} />
            <MetricCard title="Logical Cost" value={`$${(loadState.usage?.logical_cost ?? 0).toFixed(4)}`} detail="面向用户账单" />
            <MetricCard title="Physical Cost" value={`$${(loadState.usage?.physical_cost ?? 0).toFixed(4)}`} detail="包括竞速与补发" />
            <MetricCard title="Sessions" value={String(loadState.usage?.active_sessions ?? 0)} detail="活跃粘性会话" />
          </div>

          {error ? (
            <Card className="relay-panel rounded-[28px] border border-rose-500/40">
              <Card.Content className="p-5">
                <p className="text-sm text-rose-300">{error}</p>
              </Card.Content>
            </Card>
          ) : null}

          <Card className="relay-panel rounded-[28px]">
            <Card.Content className="p-6">
              {loading ? (
                <div className="flex min-h-[360px] items-center justify-center gap-4 text-slate-300">
                  <Spinner />
                  <span>加载 RelayHub 数据...</span>
                </div>
              ) : (
                <div className="space-y-5">
                  <div className="flex flex-wrap gap-3">
                    {(
                      [
                        ["dashboard", "Dashboard"],
                        ["providers", "Providers"],
                        ["models", "Models"],
                        ["rules", "Rules"],
                        ["requests", "Requests"],
                        ["sessions", "Sessions"],
                      ] as Array<[SectionKey, string]>
                    ).map(([key, label]) => (
                      <Button
                        key={key}
                        variant={section === key ? "primary" : "secondary"}
                        onPress={() => setSection(key)}
                      >
                        {label}
                      </Button>
                    ))}
                  </div>
                  {renderSection()}
                </div>
              )}
            </Card.Content>
          </Card>
        </div>
      </div>
    </div>
  );
}

function LabelledField({
  label,
  value,
  onChange,
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
}) {
  return (
    <label className="block">
      <span className="mb-2 block text-xs uppercase tracking-[0.25em] text-slate-500">{label}</span>
      <input
        className="w-full rounded-2xl border border-white/10 bg-slate-950/50 px-4 py-3 text-sm text-white outline-none transition focus:border-cyan-400/50"
        value={value}
        onChange={(event) => onChange(event.target.value)}
      />
    </label>
  );
}

function MetricCard({ title, value, detail }: { title: string; value: string; detail: string }) {
  return (
    <Card className="relay-panel rounded-[24px]">
      <Card.Content className="gap-2 p-5">
        <p className="text-xs uppercase tracking-[0.28em] text-slate-500">{title}</p>
        <p className="text-3xl font-semibold text-white">{value}</p>
        <p className="text-sm text-slate-400">{detail}</p>
      </Card.Content>
    </Card>
  );
}

function DashboardOverview({ overview }: { overview?: OverviewResponse }) {
  return (
    <div className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_360px]">
      <Card className="relay-panel rounded-[24px]">
        <Card.Header className="justify-between">
          <div>
            <p className="text-xs uppercase tracking-[0.28em] text-slate-500">Overview</p>
            <h2 className="text-xl font-semibold text-white">{overview?.instance ?? "RelayHub"}</h2>
          </div>
          <Chip color="success" variant="soft">
            {overview?.listen ?? ":8080"}
          </Chip>
        </Card.Header>
        <Card.Content className="grid gap-4 p-6 pt-0 md:grid-cols-2">
          {(overview?.providers ?? []).map((provider) => (
            <Card key={provider.id} className="rounded-[20px] border border-white/10 bg-white/5">
              <Card.Content className="gap-3 p-4">
                <div className="flex items-center justify-between">
                  <p className="font-medium text-white">{provider.name}</p>
                  <Chip color={provider.enabled ? "success" : "danger"} size="sm" variant="soft">
                    {provider.enabled ? "Enabled" : "Disabled"}
                  </Chip>
                </div>
                <div className="flex flex-wrap gap-2">
                  {(provider.tags ?? []).map((tag) => (
                    <Chip key={tag} size="sm" variant="secondary">
                      {tag}
                    </Chip>
                  ))}
                </div>
                <p className="text-sm text-slate-400">
                  health {provider.health_score} / priority {provider.priority}
                </p>
              </Card.Content>
            </Card>
          ))}
        </Card.Content>
      </Card>

      <Card className="relay-panel rounded-[24px]">
        <Card.Header>
          <div>
            <p className="text-xs uppercase tracking-[0.28em] text-slate-500">Usage Split</p>
            <h2 className="text-xl font-semibold text-white">Logical vs Physical</h2>
          </div>
        </Card.Header>
        <Card.Content className="space-y-4 p-6 pt-0">
          <CostBox title="Logical Cost" value={(overview?.summary.logical_cost ?? 0).toFixed(4)} tone="emerald" />
          <CostBox title="Physical Cost" value={(overview?.summary.physical_cost ?? 0).toFixed(4)} tone="cyan" />
          <CostBox title="Race Extra" value={(overview?.summary.race_extra_cost ?? 0).toFixed(4)} tone="amber" />
        </Card.Content>
      </Card>
    </div>
  );
}

function CostBox({ title, value, tone }: { title: string; value: string; tone: "emerald" | "cyan" | "amber" }) {
  const toneClass =
    tone === "emerald"
      ? "border-emerald-400/20 bg-emerald-500/10 text-emerald-200"
      : tone === "cyan"
        ? "border-cyan-400/20 bg-cyan-500/10 text-cyan-100"
        : "border-amber-400/20 bg-amber-500/10 text-amber-100";
  return (
    <div className={`rounded-[20px] border p-4 ${toneClass}`}>
      <p className="text-xs uppercase tracking-[0.24em] opacity-70">{title}</p>
      <p className="mt-2 text-3xl font-semibold">${value}</p>
    </div>
  );
}

function ProvidersTable({ providers }: { providers: ProviderItem[] }) {
  return <SimpleTable headers={["ID", "Name", "Type", "Priority", "Tags"]} rows={providers.map((provider) => [
    <code key={`${provider.id}-code`} className="relay-mono text-xs">{provider.id}</code>,
    provider.name,
    provider.type,
    String(provider.priority),
    provider.tags.join(", "),
  ])} emptyText="No providers" />;
}

function ModelsPanel({ models }: { models: ModelItem[] }) {
  return (
    <div className="grid gap-4">
      {models.map((model) => (
        <Card key={model.id} className="rounded-[22px] border border-white/10 bg-white/5">
          <Card.Content className="gap-3 p-5">
            <div className="flex flex-wrap items-center justify-between gap-2">
              <div>
                <p className="font-semibold text-white">{model.name}</p>
                <p className="text-sm text-slate-400">{model.description}</p>
              </div>
              <code className="relay-mono rounded bg-white/10 px-2 py-1 text-xs">{model.id}</code>
            </div>
            <div className="flex flex-wrap gap-2">
              {(model.tags ?? []).map((tag) => (
                <Chip key={tag} size="sm" variant="secondary">
                  {tag}
                </Chip>
              ))}
            </div>
            <div className="grid gap-2 md:grid-cols-2">
              {model.targets.map((target) => (
                <div key={`${model.id}-${target.provider_id}`} className="rounded-[18px] border border-white/10 bg-slate-950/30 p-3">
                  <p className="text-sm font-medium text-white">{target.provider_id}</p>
                  <p className="text-xs text-slate-400">{target.model}</p>
                  <p className="text-xs text-slate-500">priority {target.priority}</p>
                </div>
              ))}
            </div>
          </Card.Content>
        </Card>
      ))}
    </div>
  );
}

function RulesTable({ rules }: { rules: RuleItem[] }) {
  return <SimpleTable headers={["ID", "Name", "Strategy", "Winner", "Candidates"]} rows={rules.map((rule) => [
    <code key={`${rule.id}-code`} className="relay-mono text-xs">{rule.id}</code>,
    rule.name,
    rule.policy.strategy,
    rule.policy.winner,
    String(rule.policy.max_candidates),
  ])} emptyText="No route rules" />;
}

function RequestsTable({ requests }: { requests: RequestItem[] }) {
  return <SimpleTable headers={["Request", "Model", "Provider", "Status", "Strategy", "Costs"]} rows={requests.map((request) => [
    <div key={`${request.request_id}-cell`}>
      <code className="relay-mono text-xs">{request.request_id.slice(0, 12)}</code>
      <p className="mt-1 max-w-[320px] text-xs text-slate-400">{request.response_preview}</p>
    </div>,
    request.logical_model,
    request.final_provider_id,
    <Chip key={`${request.request_id}-status`} color={request.status === "succeeded" ? "success" : "danger"} size="sm" variant="soft">{request.status}</Chip>,
    request.route_strategy,
    <div key={`${request.request_id}-costs`} className="text-xs">
      <div>logical ${request.logical_usage.cost.toFixed(4)}</div>
      <div>physical ${request.physical_cost.toFixed(4)}</div>
    </div>,
  ])} emptyText="No requests" />;
}

function SessionsTable({ sessions }: { sessions: SessionItem[] }) {
  return <SimpleTable headers={["Session", "Project", "Provider", "Model", "Last Seen"]} rows={sessions.map((session) => [
    <code key={`${session.session_key}-code`} className="relay-mono text-xs">{session.session_key}</code>,
    session.project_id,
    session.provider_id,
    session.provider_model,
    new Date(session.last_seen_at).toLocaleString(),
  ])} emptyText="No active sessions" />;
}

function SimpleTable({
  headers,
  rows,
  emptyText,
}: {
  headers: string[];
  rows: Array<Array<ReactNode>>;
  emptyText: string;
}) {
  if (rows.length === 0) {
    return <p className="text-sm text-slate-400">{emptyText}</p>;
  }
  return (
    <div className="overflow-hidden rounded-[24px] border border-white/10">
      <div className="overflow-x-auto">
        <table className="min-w-full border-collapse">
          <thead className="bg-white/5">
            <tr>
              {headers.map((header) => (
                <th key={header} className="px-4 py-3 text-left text-xs uppercase tracking-[0.2em] text-slate-500">
                  {header}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {rows.map((row, rowIndex) => (
              <tr key={`row-${rowIndex}`} className="border-t border-white/10">
                {row.map((cell, cellIndex) => (
                  <td key={`cell-${rowIndex}-${cellIndex}`} className="px-4 py-4 align-top text-sm text-slate-200">
                    {cell}
                  </td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
