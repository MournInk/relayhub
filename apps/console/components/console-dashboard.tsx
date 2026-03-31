"use client";

import { useEffect, useMemo, useState } from "react";
import {
  Activity,
  ArrowRight,
  Bot,
  Cable,
  ChartNoAxesCombined,
  Coins,
  Database,
  GitBranch,
  Globe,
  KeyRound,
  LayoutDashboard,
  Link2,
  Network,
  RefreshCcw,
  Route,
  ScrollText,
  ServerCog,
  ShieldCheck,
  Sparkles,
  TimerReset,
  TriangleAlert,
  Waypoints,
} from "lucide-react";

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
import { cn } from "../lib/utils";
import { Button } from "./ui/button";
import { Input } from "./ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "./ui/select";
import { Separator } from "./ui/separator";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "./ui/tabs";

type LoadState = {
  overview?: OverviewResponse;
  providers?: ProviderItem[];
  models?: ModelItem[];
  rules?: RuleItem[];
  requests?: RequestItem[];
  sessions?: SessionItem[];
  usage?: OverviewResponse["summary"];
};

type SectionKey =
  | "dashboard"
  | "providers"
  | "models"
  | "rules"
  | "requests"
  | "sessions";

type ActivityMode = "status" | "strategy";
type EndpointPreset = "desktop-local" | "web-local" | "custom";

const DEFAULT_WEB_API_BASE = "http://127.0.0.1:8080";
const DEFAULT_DESKTOP_API_BASE = "http://127.0.0.1:4317";
const DEFAULT_ADMIN_TOKEN = "relayhub-admin";

const EMPTY_SUMMARY: OverviewResponse["summary"] = {
  requests: 0,
  successes: 0,
  failures: 0,
  input_tokens: 0,
  output_tokens: 0,
  logical_cost: 0,
  physical_cost: 0,
  race_extra_cost: 0,
  active_sessions: 0,
};

const NAV_ITEMS: Array<{
  value: SectionKey;
  label: string;
  description: string;
  icon: typeof LayoutDashboard;
}> = [
  {
    value: "dashboard",
    label: "总览",
    description: "路由与经营指标",
    icon: LayoutDashboard,
  },
  {
    value: "providers",
    label: "Provider",
    description: "供应侧健康与成本",
    icon: ServerCog,
  },
  {
    value: "models",
    label: "模型",
    description: "逻辑模型拓扑",
    icon: Bot,
  },
  {
    value: "rules",
    label: "规则",
    description: "策略与命中条件",
    icon: Route,
  },
  {
    value: "requests",
    label: "请求",
    description: "近期链路结果",
    icon: ScrollText,
  },
  {
    value: "sessions",
    label: "会话",
    description: "粘性绑定状态",
    icon: Link2,
  },
];

const compactCountFormatter = new Intl.NumberFormat("zh-CN", {
  notation: "compact",
  maximumFractionDigits: 1,
});

function detectDefaultApiBase() {
  if (typeof window === "undefined") {
    return DEFAULT_WEB_API_BASE;
  }
  if ("__TAURI_INTERNALS__" in window || "__TAURI__" in window) {
    return DEFAULT_DESKTOP_API_BASE;
  }
  return DEFAULT_WEB_API_BASE;
}

function detectDefaultState(): RelayState {
  return {
    apiBase: detectDefaultApiBase(),
    adminToken: DEFAULT_ADMIN_TOKEN,
  };
}

function matchEndpointPreset(apiBase: string): EndpointPreset {
  if (apiBase === DEFAULT_DESKTOP_API_BASE) {
    return "desktop-local";
  }
  if (apiBase === DEFAULT_WEB_API_BASE) {
    return "web-local";
  }
  return "custom";
}

function formatCompact(value: number) {
  return compactCountFormatter.format(value);
}

function formatMoney(value: number) {
  if (value >= 1000) {
    return `$${(value / 1000).toFixed(1)}k`;
  }
  if (value >= 1) {
    return `$${value.toFixed(2)}`;
  }
  return `$${value.toFixed(4)}`;
}

function formatPercent(value: number) {
  return `${Math.round(value * 100)}%`;
}

function formatDateTime(value?: string) {
  if (!value) {
    return "--";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return date.toLocaleString("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  });
}

function shortId(value: string) {
  if (value.length <= 12) {
    return value;
  }
  return `${value.slice(0, 8)}...${value.slice(-4)}`;
}

function statusBadgeClass(status: string) {
  if (status === "succeeded") {
    return "border-emerald-200/80 bg-emerald-100/80 text-emerald-800";
  }
  return "border-rose-200/80 bg-rose-100/80 text-rose-800";
}

function strategyBadgeClass(strategy: string) {
  if (strategy === "race") {
    return "border-emerald-200/80 bg-emerald-100/80 text-emerald-800";
  }
  if (strategy === "failover") {
    return "border-amber-200/80 bg-amber-100/80 text-amber-800";
  }
  return "border-stone-200/80 bg-stone-100/80 text-stone-700";
}

function squareToneFromRequest(
  request: RequestItem | undefined,
  mode: ActivityMode,
): string {
  if (!request) {
    return "bg-[#d9d7cf]";
  }
  if (mode === "status") {
    return request.status === "succeeded" ? "bg-[#7ea875]" : "bg-[#d77d69]";
  }
  if (request.route_strategy === "race") {
    return "bg-[#74a86e]";
  }
  if (request.route_strategy === "failover") {
    return "bg-[#d1a861]";
  }
  return "bg-[#8fa1ad]";
}

function SurfaceCard({
  className,
  children,
}: {
  className?: string;
  children: React.ReactNode;
}) {
  return <section className={cn("rh-surface p-6 md:p-7", className)}>{children}</section>;
}

function SectionHeader({
  eyebrow,
  title,
  description,
  action,
}: {
  eyebrow?: string;
  title: string;
  description?: string;
  action?: React.ReactNode;
}) {
  return (
    <div className="flex flex-col gap-4 md:flex-row md:items-start md:justify-between">
      <div className="space-y-2">
        {eyebrow ? <p className="rh-label">{eyebrow}</p> : null}
        <div className="space-y-1">
          <h2 className="text-2xl font-semibold tracking-[-0.04em] text-foreground">
            {title}
          </h2>
          {description ? (
            <p className="max-w-3xl text-sm leading-6 text-muted-foreground">
              {description}
            </p>
          ) : null}
        </div>
      </div>
      {action ? <div className="shrink-0">{action}</div> : null}
    </div>
  );
}

function BrandGlyph({ className }: { className?: string }) {
  return (
    <svg
      viewBox="0 0 64 64"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
      className={className}
      aria-hidden="true"
    >
      <circle cx="14" cy="18" r="5.5" className="fill-primary/12 stroke-primary" strokeWidth="2.5" />
      <circle cx="49" cy="16" r="5.5" className="fill-primary/12 stroke-primary" strokeWidth="2.5" />
      <circle cx="20" cy="46" r="5.5" className="fill-primary/12 stroke-primary" strokeWidth="2.5" />
      <circle cx="48" cy="45" r="5.5" className="fill-primary/12 stroke-primary" strokeWidth="2.5" />
      <path
        d="M18 21L28 30.5C30.2 32.6 33.8 32.6 36 30.5L45 22"
        className="stroke-primary"
        strokeWidth="2.5"
        strokeLinecap="round"
      />
      <path
        d="M24.5 42L30 36.5C31.2 35.3 32.8 35.3 34 36.5L43 45.5"
        className="stroke-primary"
        strokeWidth="2.5"
        strokeLinecap="round"
      />
    </svg>
  );
}

function StatusChip({
  tone,
  children,
}: {
  tone: "success" | "warning" | "neutral" | "danger";
  children: React.ReactNode;
}) {
  const toneClass =
    tone === "success"
      ? "border-emerald-200/80 bg-emerald-100/80 text-emerald-800"
      : tone === "warning"
        ? "border-amber-200/80 bg-amber-100/80 text-amber-800"
        : tone === "danger"
          ? "border-rose-200/80 bg-rose-100/80 text-rose-800"
          : "border-stone-200/80 bg-stone-100/80 text-stone-700";
  return (
    <span
      className={cn(
        "inline-flex items-center gap-2 rounded-full border px-3 py-1.5 text-xs font-semibold",
        toneClass,
      )}
    >
      {children}
    </span>
  );
}

function MiniLabel({ children }: { children: React.ReactNode }) {
  return <p className="rh-label">{children}</p>;
}

function MetricCard({
  icon: Icon,
  label,
  value,
  detail,
}: {
  icon: typeof Activity;
  label: string;
  value: string;
  detail: string;
}) {
  return (
    <div className="rh-surface-soft p-5">
      <div className="flex items-start justify-between gap-4">
        <div className="flex h-12 w-12 items-center justify-center rounded-[18px] bg-primary/10 text-primary">
          <Icon className="h-5 w-5" />
        </div>
        <MiniLabel>{label}</MiniLabel>
      </div>
      <div className="mt-6 space-y-1">
        <p className="text-3xl font-semibold tracking-[-0.05em] text-foreground">{value}</p>
        <p className="text-sm text-muted-foreground">{detail}</p>
      </div>
    </div>
  );
}

function TableShell({
  headers,
  rows,
  emptyText,
}: {
  headers: string[];
  rows: Array<Array<React.ReactNode>>;
  emptyText: string;
}) {
  if (rows.length === 0) {
    return (
      <div className="flex min-h-[220px] items-center justify-center rounded-[26px] border border-dashed border-border/80 bg-muted/20 px-6 text-sm text-muted-foreground">
        {emptyText}
      </div>
    );
  }

  return (
    <div className="overflow-hidden rounded-[26px] border border-border/80 bg-white/55">
      <div className="overflow-x-auto">
        <table className="min-w-full border-collapse">
          <thead className="bg-muted/55">
            <tr>
              {headers.map((header) => (
                <th
                  key={header}
                  className="px-4 py-3 text-left text-[11px] font-semibold uppercase tracking-[0.24em] text-muted-foreground"
                >
                  {header}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {rows.map((row, rowIndex) => (
              <tr key={`row-${rowIndex}`} className="border-t border-border/70">
                {row.map((cell, cellIndex) => (
                  <td key={`cell-${rowIndex}-${cellIndex}`} className="px-4 py-4 align-top text-sm text-foreground">
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

export function ConsoleDashboard() {
  const [activeTab, setActiveTab] = useState<SectionKey>("dashboard");
  const [activityMode, setActivityMode] = useState<ActivityMode>("status");
  const [draftState, setDraftState] = useState<RelayState>({
    apiBase: DEFAULT_WEB_API_BASE,
    adminToken: DEFAULT_ADMIN_TOKEN,
  });
  const [endpointPreset, setEndpointPreset] = useState<EndpointPreset>("web-local");
  const [activeState, setActiveState] = useState<RelayState>({
    apiBase: DEFAULT_WEB_API_BASE,
    adminToken: DEFAULT_ADMIN_TOKEN,
  });
  const [loadState, setLoadState] = useState<LoadState>({});
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);
  const [initialized, setInitialized] = useState(false);
  const [lastSyncedAt, setLastSyncedAt] = useState<string>("");

  useEffect(() => {
    const fallback = detectDefaultState();
    let initialState = fallback;

    if (typeof window !== "undefined") {
      const saved = window.localStorage.getItem("relayhub-console");
      if (saved) {
        try {
          initialState = JSON.parse(saved) as RelayState;
        } catch {
          initialState = fallback;
        }
      }
    }

    setDraftState(initialState);
    setEndpointPreset(matchEndpointPreset(initialState.apiBase));
    setActiveState(initialState);
    setInitialized(true);
    void load(initialState);
  }, []);

  async function load(nextState: RelayState) {
    setLoading(true);
    setError("");

    try {
      const [overview, providers, models, rules, requests, sessions, usage] =
        await Promise.all([
          relayApi.overview(nextState),
          relayApi.providers(nextState),
          relayApi.models(nextState),
          relayApi.rules(nextState),
          relayApi.requests(nextState),
          relayApi.sessions(nextState),
          relayApi.usage(nextState),
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
      setLastSyncedAt(new Date().toISOString());
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "Unknown error");
    } finally {
      setLoading(false);
    }
  }

  async function connectAndRefresh() {
    setActiveState(draftState);
    if (typeof window !== "undefined") {
      window.localStorage.setItem("relayhub-console", JSON.stringify(draftState));
    }
    await load(draftState);
  }

  async function refreshActive() {
    await load(activeState);
  }

  const summary = loadState.usage ?? loadState.overview?.summary ?? EMPTY_SUMMARY;
  const providers = loadState.providers ?? [];
  const models = loadState.models ?? [];
  const rules = loadState.rules ?? [];
  const requests = loadState.requests ?? [];
  const sessions = loadState.sessions ?? [];

  const successRate = summary.requests > 0 ? summary.successes / summary.requests : 0;
  const avgLogicalCost = summary.requests > 0 ? summary.logical_cost / summary.requests : 0;
  const avgPhysicalCost = summary.requests > 0 ? summary.physical_cost / summary.requests : 0;

  const providerHealth = useMemo(
    () =>
      [...providers].sort((left, right) => {
        if (right.health_score !== left.health_score) {
          return right.health_score - left.health_score;
        }
        return right.priority - left.priority;
      }),
    [providers],
  );

  const winnerDistribution = useMemo(() => {
    const counts = new Map<string, number>();
    for (const item of requests) {
      counts.set(item.final_provider_id, (counts.get(item.final_provider_id) ?? 0) + 1);
    }
    return [...counts.entries()]
      .map(([providerID, count]) => ({
        providerID,
        count,
      }))
      .sort((left, right) => right.count - left.count)
      .slice(0, 4);
  }, [requests]);

  const strategyStats = useMemo(() => {
    const counts = new Map<string, number>();
    for (const item of requests) {
      counts.set(item.route_strategy, (counts.get(item.route_strategy) ?? 0) + 1);
    }
    return [...counts.entries()].sort((left, right) => right[1] - left[1]);
  }, [requests]);

  const activitySquares = useMemo(() => {
    const total = 120;
    const latest = [...requests].slice(0, total).reverse();
    return Array.from({ length: total }, (_, index) => {
      const request = latest[index];
      return {
        key: request ? `${request.request_id}-${index}` : `empty-${index}`,
        tone: squareToneFromRequest(request, activityMode),
        label: request
          ? `${request.logical_model} / ${request.route_strategy} / ${request.status}`
          : "No request yet",
      };
    });
  }, [activityMode, requests]);

  const sectionMeta = useMemo(
    () =>
      NAV_ITEMS.find((item) => item.value === activeTab) ?? NAV_ITEMS[0],
    [activeTab],
  );

  const overviewRows = requests.slice(0, 6).map((request) => [
    <div key={`${request.request_id}-head`} className="space-y-1">
      <p className="font-semibold text-foreground">{request.logical_model}</p>
      <p className="font-mono text-xs text-muted-foreground">{shortId(request.request_id)}</p>
    </div>,
    <div key={`${request.request_id}-strategy`} className="space-y-2">
      <span
        className={cn(
          "inline-flex rounded-full border px-3 py-1 text-xs font-semibold",
          strategyBadgeClass(request.route_strategy),
        )}
      >
        {request.route_strategy}
      </span>
      <p className="text-xs text-muted-foreground">{request.final_provider_id}</p>
    </div>,
    <div key={`${request.request_id}-response`} className="max-w-[420px] text-sm leading-6 text-muted-foreground">
      {request.response_preview}
    </div>,
    <div key={`${request.request_id}-cost`} className="space-y-1">
      <p className="font-semibold text-foreground">{formatMoney(request.physical_cost)}</p>
      <p className="text-xs text-muted-foreground">
        logic {formatMoney(request.logical_usage.cost)}
      </p>
    </div>,
  ]);

  const requestTableRows = requests.map((request) => [
    <div key={`${request.request_id}-main`} className="space-y-1">
      <p className="font-semibold text-foreground">{request.logical_model}</p>
      <p className="font-mono text-xs text-muted-foreground">{request.request_id}</p>
    </div>,
    <span
      key={`${request.request_id}-state`}
      className={cn(
        "inline-flex rounded-full border px-3 py-1 text-xs font-semibold",
        statusBadgeClass(request.status),
      )}
    >
      {request.status}
    </span>,
    <span
      key={`${request.request_id}-route`}
      className={cn(
        "inline-flex rounded-full border px-3 py-1 text-xs font-semibold",
        strategyBadgeClass(request.route_strategy),
      )}
    >
      {request.route_strategy}
    </span>,
    <div key={`${request.request_id}-provider`} className="space-y-1">
      <p className="font-semibold text-foreground">{request.final_provider_id}</p>
      <p className="text-xs text-muted-foreground">{formatDateTime(request.started_at)}</p>
    </div>,
    <div key={`${request.request_id}-money`} className="space-y-1">
      <p className="font-semibold text-foreground">{formatMoney(request.physical_cost)}</p>
      <p className="text-xs text-muted-foreground">
        logical {formatMoney(request.logical_usage.cost)}
      </p>
    </div>,
    <p key={`${request.request_id}-preview`} className="max-w-[420px] text-sm leading-6 text-muted-foreground">
      {request.response_preview}
    </p>,
  ]);

  const sessionTableRows = sessions.map((session) => [
    <div key={`${session.session_key}-session`} className="space-y-1">
      <p className="font-semibold text-foreground">{session.project_id}</p>
      <p className="font-mono text-xs text-muted-foreground">{session.session_key}</p>
    </div>,
    <div key={`${session.session_key}-provider`} className="space-y-1">
      <p className="font-semibold text-foreground">{session.provider_id}</p>
      <p className="text-xs text-muted-foreground">{session.provider_model}</p>
    </div>,
    <p key={`${session.session_key}-seen`} className="text-sm text-muted-foreground">
      {formatDateTime(session.last_seen_at)}
    </p>,
  ]);

  const isConnected = Boolean(loadState.overview) && !error;

  return (
    <div className="rh-shell">
      <Tabs
        orientation="vertical"
        value={activeTab}
        onValueChange={(value) => setActiveTab(value as SectionKey)}
        className="flex flex-col gap-5 md:flex-row"
      >
        <aside className="md:sticky md:top-8 md:self-start">
          <div className="rh-surface flex flex-col gap-3 p-3 md:w-[98px]">
            <div className="hidden items-center justify-center rounded-[26px] bg-primary/10 py-5 text-primary md:flex">
              <BrandGlyph className="h-10 w-10" />
            </div>
            <TabsList className="flex gap-2 overflow-x-auto md:flex-col md:overflow-visible">
              {NAV_ITEMS.map(({ value, label, icon: Icon }) => (
                <TabsTrigger
                  key={value}
                  value={value}
                  className="min-w-[104px] flex-1 px-3 py-3 md:min-w-0 md:flex-none md:rounded-[24px] md:px-0 md:py-4"
                  title={label}
                >
                  <div className="flex items-center gap-2 md:flex-col md:gap-2">
                    <Icon className="h-4 w-4 md:h-5 md:w-5" />
                    <span className="text-xs font-semibold tracking-[0.08em] md:hidden">
                      {label}
                    </span>
                  </div>
                </TabsTrigger>
              ))}
            </TabsList>
            <div className="hidden rounded-[24px] border border-border/70 bg-white/60 p-3 text-center md:block">
              <p className="text-[10px] font-semibold uppercase tracking-[0.26em] text-muted-foreground">
                RelayHub
              </p>
              <p className="mt-2 text-sm font-semibold text-foreground">Control</p>
            </div>
          </div>
        </aside>

        <div className="min-w-0 flex-1 space-y-5">
          <header className="grid gap-5 xl:grid-cols-[minmax(0,1fr)_420px]">
            <SurfaceCard className="overflow-hidden">
              <div className="flex flex-col gap-10">
                <div className="flex items-start gap-4">
                  <div className="flex h-16 w-16 items-center justify-center rounded-[24px] bg-primary/10 text-primary shadow-[inset_0_0_0_1px_rgba(121,152,112,0.16)]">
                    <BrandGlyph className="h-9 w-9" />
                  </div>
                  <div className="space-y-3">
                    <p className="rh-label">RelayHub Control Plane</p>
                    <h1 className="rh-title">{sectionMeta.label}</h1>
                    <p className="max-w-3xl text-sm leading-7 text-muted-foreground md:text-base">
                      {sectionMeta.description}。整体视觉基于暖白企业中控台重构，借鉴
                      Octopus 的悬浮导航、大圆角面板与柔和绿系层次，但完全按 RelayHub
                      的多来源路由、竞速与用量治理语义重新组织。
                    </p>
                  </div>
                </div>

                <div className="grid gap-4 md:grid-cols-3">
                  <div className="rh-surface-soft p-5">
                    <MiniLabel>Control Scope</MiniLabel>
                    <div className="mt-4 flex items-center justify-between">
                      <div>
                        <p className="text-2xl font-semibold tracking-[-0.04em]">
                          {loadState.overview?.instance ?? "RelayHub"}
                        </p>
                        <p className="mt-2 text-sm text-muted-foreground">
                          {loadState.overview?.listen ?? activeState.apiBase}
                        </p>
                      </div>
                      <Network className="h-6 w-6 text-primary" />
                    </div>
                  </div>

                  <div className="rh-surface-soft p-5">
                    <MiniLabel>Routing Fabric</MiniLabel>
                    <div className="mt-4 flex items-end justify-between gap-4">
                      <div>
                        <p className="text-2xl font-semibold tracking-[-0.04em]">
                          {providers.length} / {models.length}
                        </p>
                        <p className="mt-2 text-sm text-muted-foreground">
                          Providers / Logical Models
                        </p>
                      </div>
                      <Waypoints className="h-6 w-6 text-primary" />
                    </div>
                  </div>

                  <div className="rh-surface-soft p-5">
                    <MiniLabel>Governance Status</MiniLabel>
                    <div className="mt-4 flex items-end justify-between gap-4">
                      <div>
                        <p className="text-2xl font-semibold tracking-[-0.04em]">
                          {rules.length} 条
                        </p>
                        <p className="mt-2 text-sm text-muted-foreground">
                          路由规则，活跃会话 {summary.active_sessions}
                        </p>
                      </div>
                      <ShieldCheck className="h-6 w-6 text-primary" />
                    </div>
                  </div>
                </div>
              </div>
            </SurfaceCard>

            <SurfaceCard>
              <SectionHeader
                eyebrow="Connection"
                title="控制链路"
                description="桌面模式默认接入本地 4317 sidecar，浏览器模式默认接入本地 8080。"
              />

              <div className="mt-6 space-y-4">
                <div className="flex flex-wrap items-center gap-3">
                  <StatusChip tone={isConnected ? "success" : error ? "danger" : "neutral"}>
                    <span
                      className={cn(
                        "h-2.5 w-2.5 rounded-full",
                        isConnected
                          ? "bg-emerald-600"
                          : error
                            ? "bg-rose-500"
                            : "bg-stone-400",
                      )}
                    />
                    {isConnected ? "连接正常" : error ? "连接异常" : "等待初始化"}
                  </StatusChip>
                  <p className="text-xs text-muted-foreground">
                    最后同步 {lastSyncedAt ? formatDateTime(lastSyncedAt) : "--"}
                  </p>
                </div>

                <label className="block space-y-2">
                  <MiniLabel>Endpoint Preset</MiniLabel>
                  <Select
                    value={endpointPreset}
                    onValueChange={(value) => {
                      if (value === "desktop-local") {
                        setEndpointPreset("desktop-local");
                        setDraftState((current) => ({
                          ...current,
                          apiBase: DEFAULT_DESKTOP_API_BASE,
                        }));
                        return;
                      }
                      if (value === "web-local") {
                        setEndpointPreset("web-local");
                        setDraftState((current) => ({
                          ...current,
                          apiBase: DEFAULT_WEB_API_BASE,
                        }));
                        return;
                      }
                      setEndpointPreset("custom");
                    }}
                  >
                    <SelectTrigger>
                      <SelectValue placeholder="选择接口入口" />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="desktop-local">Desktop Local · 4317</SelectItem>
                      <SelectItem value="web-local">Web Local · 8080</SelectItem>
                      <SelectItem value="custom">Custom Endpoint</SelectItem>
                    </SelectContent>
                  </Select>
                </label>

                <label className="block space-y-2">
                  <MiniLabel>Admin API Base</MiniLabel>
                  <Input
                    value={draftState.apiBase}
                    onChange={(event) => {
                      const nextApiBase = event.target.value;
                      setDraftState((current) => ({
                        ...current,
                        apiBase: nextApiBase,
                      }));
                      setEndpointPreset(matchEndpointPreset(nextApiBase));
                    }}
                    placeholder="http://127.0.0.1:4317"
                  />
                </label>

                <label className="block space-y-2">
                  <MiniLabel>Admin Token</MiniLabel>
                  <Input
                    value={draftState.adminToken}
                    onChange={(event) =>
                      setDraftState((current) => ({
                        ...current,
                        adminToken: event.target.value,
                      }))
                    }
                    placeholder="relayhub-admin"
                  />
                </label>

                <div className="flex flex-wrap gap-3 pt-2">
                  <Button onClick={() => void connectAndRefresh()}>
                    <Cable className="h-4 w-4" />
                    连接并刷新
                  </Button>
                  <Button variant="secondary" onClick={() => void refreshActive()}>
                    <RefreshCcw className="h-4 w-4" />
                    仅刷新
                  </Button>
                </div>

                <Separator className="my-2" />

                <div className="space-y-2 text-sm text-muted-foreground">
                  <p className="flex items-center gap-2">
                    <KeyRound className="h-4 w-4 text-primary" />
                    默认代理 Key：<span className="font-mono text-foreground">relayhub-local-key</span>
                  </p>
                  <p className="flex items-center gap-2">
                    <Globe className="h-4 w-4 text-primary" />
                    当前控制面目标：<span className="font-mono text-foreground">{activeState.apiBase}</span>
                  </p>
                </div>
              </div>
            </SurfaceCard>
          </header>

          {loading && initialized ? (
            <div className="rh-surface-soft flex items-center gap-3 px-5 py-4 text-sm text-muted-foreground">
              <span className="h-3 w-3 animate-spin rounded-full border-2 border-primary/25 border-t-primary" />
              正在同步控制面数据
            </div>
          ) : null}

          {error ? (
            <div className="rh-surface flex items-start gap-4 border-rose-200/80 bg-rose-50/80 p-5 text-rose-900 shadow-none">
              <TriangleAlert className="mt-0.5 h-5 w-5 shrink-0 text-rose-600" />
              <div className="space-y-1">
                <p className="text-sm font-semibold">控制面请求失败</p>
                <p className="text-sm leading-6 text-rose-800">{error}</p>
              </div>
            </div>
          ) : null}

          <TabsContent value="dashboard" className="space-y-5">
            <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
              <MetricCard
                icon={Activity}
                label="Requests"
                value={formatCompact(summary.requests)}
                detail={`成功率 ${formatPercent(successRate)}`}
              />
              <MetricCard
                icon={Coins}
                label="Logical Cost"
                value={formatMoney(summary.logical_cost)}
                detail={`平均单次 ${formatMoney(avgLogicalCost)}`}
              />
              <MetricCard
                icon={ChartNoAxesCombined}
                label="Physical Cost"
                value={formatMoney(summary.physical_cost)}
                detail={`平均单次 ${formatMoney(avgPhysicalCost)}`}
              />
              <MetricCard
                icon={Sparkles}
                label="Race Extra"
                value={formatMoney(summary.race_extra_cost)}
                detail="竞速额外消耗"
              />
              <MetricCard
                icon={Link2}
                label="Sticky Sessions"
                value={String(summary.active_sessions)}
                detail="活跃绑定中的会话"
              />
              <MetricCard
                icon={Database}
                label="Token Throughput"
                value={formatCompact(summary.input_tokens + summary.output_tokens)}
                detail={`输入 ${formatCompact(summary.input_tokens)} · 输出 ${formatCompact(summary.output_tokens)}`}
              />
            </div>

            <div className="grid gap-5 xl:grid-cols-[minmax(0,1.45fr)_390px]">
              <div className="space-y-5">
                <SurfaceCard>
                  <SectionHeader
                    eyebrow="Routing Pulse"
                    title="请求活跃度矩阵"
                    description="基于最近请求生成的信号格，用来快速感知链路稳定性或策略分布。"
                    action={
                      <div className="w-[180px]">
                        <Select
                          value={activityMode}
                          onValueChange={(value) => setActivityMode(value as ActivityMode)}
                        >
                          <SelectTrigger className="h-10">
                            <SelectValue placeholder="选择观察方式" />
                          </SelectTrigger>
                          <SelectContent>
                            <SelectItem value="status">按状态着色</SelectItem>
                            <SelectItem value="strategy">按策略着色</SelectItem>
                          </SelectContent>
                        </Select>
                      </div>
                    }
                  />

                  <div className="mt-6 space-y-4">
                    <div className="rh-data-grid rounded-[28px] border border-border/70 bg-white/55 p-5">
                      <div className="grid grid-cols-10 gap-2 sm:grid-cols-12 lg:grid-cols-[repeat(15,minmax(0,1fr))] xl:grid-cols-[repeat(20,minmax(0,1fr))]">
                        {activitySquares.map((cell) => (
                          <div
                            key={cell.key}
                            className={cn("h-5 rounded-md transition-transform hover:scale-110", cell.tone)}
                            title={cell.label}
                          />
                        ))}
                      </div>
                    </div>

                    <div className="flex flex-wrap gap-3 text-xs font-medium text-muted-foreground">
                      {activityMode === "status" ? (
                        <>
                          <span className="inline-flex items-center gap-2">
                            <span className="h-3 w-3 rounded bg-[#7ea875]" />
                            Succeeded
                          </span>
                          <span className="inline-flex items-center gap-2">
                            <span className="h-3 w-3 rounded bg-[#d77d69]" />
                            Failed
                          </span>
                          <span className="inline-flex items-center gap-2">
                            <span className="h-3 w-3 rounded bg-[#d9d7cf]" />
                            Empty Slot
                          </span>
                        </>
                      ) : (
                        <>
                          <span className="inline-flex items-center gap-2">
                            <span className="h-3 w-3 rounded bg-[#74a86e]" />
                            Race
                          </span>
                          <span className="inline-flex items-center gap-2">
                            <span className="h-3 w-3 rounded bg-[#d1a861]" />
                            Failover
                          </span>
                          <span className="inline-flex items-center gap-2">
                            <span className="h-3 w-3 rounded bg-[#8fa1ad]" />
                            Single
                          </span>
                        </>
                      )}
                    </div>
                  </div>
                </SurfaceCard>

                <SurfaceCard>
                  <SectionHeader
                    eyebrow="Routing Fabric"
                    title="逻辑模型拓扑"
                    description="每个逻辑模型对应一组目标 Provider 与治理策略，形成统一入口后的真实执行蓝图。"
                  />
                  <div className="mt-6 grid gap-4 lg:grid-cols-3">
                    {models.map((model) => {
                      const attachedRule = rules.find(
                        (rule) => rule.match.logical_model === model.id,
                      );
                      return (
                        <div
                          key={model.id}
                          className="rh-surface-soft flex flex-col gap-4 p-5"
                        >
                          <div className="flex items-start justify-between gap-3">
                            <div className="space-y-1">
                              <p className="text-lg font-semibold tracking-[-0.03em] text-foreground">
                                {model.name}
                              </p>
                              <p className="text-sm leading-6 text-muted-foreground">
                                {model.description}
                              </p>
                            </div>
                            <span className="rounded-full bg-primary/10 px-3 py-1 text-xs font-semibold text-primary">
                              {model.id}
                            </span>
                          </div>

                          <div className="flex flex-wrap gap-2">
                            {(model.tags ?? []).map((tag) => (
                              <span
                                key={`${model.id}-${tag}`}
                                className="rounded-full border border-border/80 bg-white/70 px-3 py-1 text-xs font-medium text-muted-foreground"
                              >
                                {tag}
                              </span>
                            ))}
                          </div>

                          <div className="rounded-[22px] border border-border/70 bg-white/68 p-4">
                            <div className="flex items-center justify-between gap-3">
                              <MiniLabel>Policy</MiniLabel>
                              {attachedRule ? (
                                <span
                                  className={cn(
                                    "rounded-full border px-3 py-1 text-xs font-semibold",
                                    strategyBadgeClass(attachedRule.policy.strategy),
                                  )}
                                >
                                  {attachedRule.policy.strategy}
                                </span>
                              ) : (
                                <StatusChip tone="neutral">未绑定规则</StatusChip>
                              )}
                            </div>
                            <div className="mt-3 space-y-3">
                              {model.targets.map((target, index) => (
                                <div
                                  key={`${model.id}-${target.provider_id}`}
                                  className="flex items-center justify-between gap-3"
                                >
                                  <div className="flex items-center gap-3">
                                    <span className="flex h-8 w-8 items-center justify-center rounded-full bg-primary/10 text-xs font-semibold text-primary">
                                      {index + 1}
                                    </span>
                                    <div>
                                      <p className="font-semibold text-foreground">
                                        {target.provider_id}
                                      </p>
                                      <p className="text-xs text-muted-foreground">
                                        {target.model}
                                      </p>
                                    </div>
                                  </div>
                                  <div className="text-right text-xs text-muted-foreground">
                                    <p>priority {target.priority}</p>
                                    <p>weight {target.weight}</p>
                                  </div>
                                </div>
                              ))}
                            </div>
                          </div>
                        </div>
                      );
                    })}
                  </div>
                </SurfaceCard>

                <SurfaceCard>
                  <SectionHeader
                    eyebrow="Operational Feed"
                    title="最新请求切片"
                    description="聚焦最近完成的请求，验证策略命中、最终落点和成本是否符合预期。"
                  />
                  <div className="mt-6">
                    <TableShell
                      headers={["Logical Model", "Route", "Response", "Cost"]}
                      rows={overviewRows}
                      emptyText="暂时还没有请求记录。"
                    />
                  </div>
                </SurfaceCard>
              </div>

              <div className="space-y-5">
                <SurfaceCard>
                  <SectionHeader
                    eyebrow="Supply Health"
                    title="Provider 健康度"
                    description="综合优先级、健康分与基础成本，快速判断主力落点是否合理。"
                  />
                  <div className="mt-6 space-y-4">
                    {providerHealth.map((provider) => (
                      <div
                        key={provider.id}
                        className="rounded-[24px] border border-border/70 bg-white/70 p-4"
                      >
                        <div className="flex items-start justify-between gap-3">
                          <div className="space-y-1">
                            <p className="font-semibold text-foreground">{provider.name}</p>
                            <p className="text-xs text-muted-foreground">
                              {provider.type} · {provider.latency_ms} ms
                            </p>
                          </div>
                          <StatusChip tone={provider.enabled ? "success" : "danger"}>
                            {provider.enabled ? "Enabled" : "Disabled"}
                          </StatusChip>
                        </div>
                        <div className="mt-4 space-y-3">
                          <div className="h-2 rounded-full bg-muted">
                            <div
                              className="h-2 rounded-full bg-primary"
                              style={{ width: `${Math.max(provider.health_score, 6)}%` }}
                            />
                          </div>
                          <div className="flex items-center justify-between text-xs text-muted-foreground">
                            <span>health {provider.health_score}</span>
                            <span>priority {provider.priority}</span>
                          </div>
                        </div>
                      </div>
                    ))}
                  </div>
                </SurfaceCard>

                <SurfaceCard>
                  <SectionHeader
                    eyebrow="Winner Share"
                    title="赢家分布"
                    description="最近请求最终落点的集中度。"
                  />
                  <div className="mt-6 space-y-4">
                    {winnerDistribution.length === 0 ? (
                      <p className="text-sm text-muted-foreground">暂无赢家分布数据。</p>
                    ) : (
                      winnerDistribution.map((winner, index) => (
                        <div
                          key={winner.providerID}
                          className="flex items-center justify-between gap-4 rounded-[22px] border border-border/70 bg-white/68 px-4 py-3"
                        >
                          <div className="flex items-center gap-3">
                            <span className="flex h-9 w-9 items-center justify-center rounded-full bg-primary/10 text-sm font-semibold text-primary">
                              {index + 1}
                            </span>
                            <div>
                              <p className="font-semibold text-foreground">{winner.providerID}</p>
                              <p className="text-xs text-muted-foreground">
                                {winner.count} requests
                              </p>
                            </div>
                          </div>
                          <ArrowRight className="h-4 w-4 text-muted-foreground" />
                        </div>
                      ))
                    )}
                  </div>
                </SurfaceCard>

                <SurfaceCard>
                  <SectionHeader
                    eyebrow="Strategy Mix"
                    title="策略占比"
                    description="观察最近的主策略是否偏离预期。"
                  />
                  <div className="mt-6 space-y-4">
                    {strategyStats.length === 0 ? (
                      <p className="text-sm text-muted-foreground">暂无策略数据。</p>
                    ) : (
                      strategyStats.map(([strategy, count]) => {
                        const ratio = requests.length > 0 ? count / requests.length : 0;
                        return (
                          <div key={strategy} className="space-y-2">
                            <div className="flex items-center justify-between text-sm">
                              <span className="font-semibold text-foreground">{strategy}</span>
                              <span className="text-muted-foreground">
                                {count} · {formatPercent(ratio)}
                              </span>
                            </div>
                            <div className="h-2 rounded-full bg-muted">
                              <div
                                className={cn(
                                  "h-2 rounded-full",
                                  strategy === "race"
                                    ? "bg-[#74a86e]"
                                    : strategy === "failover"
                                      ? "bg-[#d1a861]"
                                      : "bg-[#8fa1ad]",
                                )}
                                style={{ width: `${Math.max(ratio * 100, 4)}%` }}
                              />
                            </div>
                          </div>
                        );
                      })
                    )}
                  </div>
                </SurfaceCard>
              </div>
            </div>
          </TabsContent>

          <TabsContent value="providers" className="space-y-5">
            <SurfaceCard>
              <SectionHeader
                eyebrow="Supply"
                title="Provider Portfolio"
                description="从健康度、延迟、成本和能力标签四个维度重新排布 Provider，全局看供应侧是否稳定。"
              />

              <div className="mt-6 grid gap-4 lg:grid-cols-2 xl:grid-cols-3">
                {providerHealth.map((provider) => (
                  <div key={provider.id} className="rh-surface-soft space-y-5 p-5">
                    <div className="flex items-start justify-between gap-3">
                      <div className="space-y-1">
                        <p className="text-lg font-semibold text-foreground">{provider.name}</p>
                        <p className="text-sm text-muted-foreground">{provider.id}</p>
                      </div>
                      <StatusChip tone={provider.enabled ? "success" : "danger"}>
                        {provider.enabled ? "Live" : "Offline"}
                      </StatusChip>
                    </div>
                    <div className="grid grid-cols-2 gap-4 text-sm">
                      <InfoPair label="Latency" value={`${provider.latency_ms} ms`} />
                      <InfoPair label="Priority" value={String(provider.priority)} />
                      <InfoPair label="Input Cost" value={formatMoney(provider.cost_per_1k_input)} />
                      <InfoPair label="Output Cost" value={formatMoney(provider.cost_per_1k_output)} />
                    </div>
                    <div className="space-y-3">
                      <MiniLabel>Capabilities</MiniLabel>
                      <div className="flex flex-wrap gap-2">
                        {provider.capabilities.map((capability) => (
                          <span
                            key={`${provider.id}-${capability}`}
                            className="rounded-full border border-border/80 bg-white/80 px-3 py-1 text-xs font-medium text-muted-foreground"
                          >
                            {capability}
                          </span>
                        ))}
                      </div>
                    </div>
                  </div>
                ))}
              </div>
            </SurfaceCard>
          </TabsContent>

          <TabsContent value="models" className="space-y-5">
            <SurfaceCard>
              <SectionHeader
                eyebrow="Logical Models"
                title="模型路由编排"
                description="逻辑模型是统一入口的抽象层，每个模型背后可以绑定不同 Provider 组合与容灾策略。"
              />
              <div className="mt-6 grid gap-4 lg:grid-cols-2">
                {models.map((model) => (
                  <div key={model.id} className="rh-surface-soft p-5">
                    <div className="flex items-start justify-between gap-3">
                      <div className="space-y-1">
                        <p className="text-xl font-semibold tracking-[-0.03em] text-foreground">
                          {model.name}
                        </p>
                        <p className="text-sm leading-6 text-muted-foreground">
                          {model.description}
                        </p>
                      </div>
                      <span className="rounded-full bg-primary/10 px-3 py-1 text-xs font-semibold text-primary">
                        {model.task_type}
                      </span>
                    </div>
                    <div className="mt-5 grid gap-3">
                      {model.targets.map((target) => (
                        <div
                          key={`${model.id}-${target.provider_id}`}
                          className="flex items-center justify-between gap-4 rounded-[22px] border border-border/75 bg-white/75 px-4 py-3"
                        >
                          <div className="space-y-1">
                            <p className="font-semibold text-foreground">{target.provider_id}</p>
                            <p className="text-xs text-muted-foreground">{target.model}</p>
                          </div>
                          <div className="text-right text-xs text-muted-foreground">
                            <p>priority {target.priority}</p>
                            <p>weight {target.weight}</p>
                          </div>
                        </div>
                      ))}
                    </div>
                  </div>
                ))}
              </div>
            </SurfaceCard>
          </TabsContent>

          <TabsContent value="rules" className="space-y-5">
            <SurfaceCard>
              <SectionHeader
                eyebrow="Policy"
                title="路由规则中枢"
                description="策略层决定是否竞速、是否自动 failover、是否优先沿用现有会话绑定。"
              />
              <div className="mt-6 grid gap-4 lg:grid-cols-2 xl:grid-cols-3">
                {rules.map((rule) => (
                  <div key={rule.id} className="rh-surface-soft flex flex-col gap-4 p-5">
                    <div className="flex items-start justify-between gap-3">
                      <div className="space-y-1">
                        <p className="text-lg font-semibold text-foreground">{rule.name}</p>
                        <p className="text-xs text-muted-foreground">{rule.id}</p>
                      </div>
                      <StatusChip tone={rule.enabled ? "success" : "neutral"}>
                        {rule.enabled ? "Enabled" : "Disabled"}
                      </StatusChip>
                    </div>

                    <div className="flex flex-wrap gap-2">
                      <span
                        className={cn(
                          "rounded-full border px-3 py-1 text-xs font-semibold",
                          strategyBadgeClass(rule.policy.strategy),
                        )}
                      >
                        {rule.policy.strategy}
                      </span>
                      <span className="rounded-full border border-border/80 bg-white/80 px-3 py-1 text-xs font-medium text-muted-foreground">
                        winner {rule.policy.winner}
                      </span>
                    </div>

                    <Separator />

                    <div className="grid grid-cols-2 gap-4 text-sm">
                      <InfoPair label="Logical Model" value={rule.match.logical_model || "all"} />
                      <InfoPair label="Priority" value={String(rule.priority)} />
                      <InfoPair
                        label="Candidates"
                        value={String(rule.policy.max_candidates)}
                      />
                      <InfoPair
                        label="Session Affinity"
                        value={rule.policy.prefer_session_bound ? "On" : "Off"}
                      />
                    </div>
                  </div>
                ))}
              </div>
            </SurfaceCard>
          </TabsContent>

          <TabsContent value="requests" className="space-y-5">
            <SurfaceCard>
              <SectionHeader
                eyebrow="Request Ledger"
                title="近期请求明细"
                description="保留逻辑模型、最终落点、响应摘要与真实成本，方便做策略回溯。"
              />
              <div className="mt-6">
                <TableShell
                  headers={["Request", "Status", "Strategy", "Winner", "Cost", "Preview"]}
                  rows={requestTableRows}
                  emptyText="当前没有请求记录。"
                />
              </div>
            </SurfaceCard>
          </TabsContent>

          <TabsContent value="sessions" className="space-y-5">
            <SurfaceCard>
              <SectionHeader
                eyebrow="Session Affinity"
                title="会话绑定视图"
                description="这里展示项目级会话如何持续粘到某个 Provider 与具体模型，帮助检查 sticky routing 的实际效果。"
              />
              <div className="mt-6 grid gap-5 xl:grid-cols-[minmax(0,1fr)_320px]">
                <TableShell
                  headers={["Project / Session", "Binding", "Last Seen"]}
                  rows={sessionTableRows}
                  emptyText="当前没有活跃会话。"
                />

                <div className="rh-surface-soft p-5">
                  <MiniLabel>Session Notes</MiniLabel>
                  <div className="mt-5 space-y-4">
                    <SessionFact
                      icon={Link2}
                      label="Active Bindings"
                      value={String(summary.active_sessions)}
                    />
                    <SessionFact
                      icon={TimerReset}
                      label="Latest Sync"
                      value={lastSyncedAt ? formatDateTime(lastSyncedAt) : "--"}
                    />
                    <SessionFact
                      icon={GitBranch}
                      label="Rule Count"
                      value={String(rules.length)}
                    />
                  </div>
                </div>
              </div>
            </SurfaceCard>
          </TabsContent>
        </div>
      </Tabs>
    </div>
  );
}

function InfoPair({ label, value }: { label: string; value: string }) {
  return (
    <div className="space-y-1">
      <p className="rh-label">{label}</p>
      <p className="text-base font-semibold text-foreground">{value}</p>
    </div>
  );
}

function SessionFact({
  icon: Icon,
  label,
  value,
}: {
  icon: typeof Activity;
  label: string;
  value: string;
}) {
  return (
    <div className="flex items-center gap-3 rounded-[22px] border border-border/75 bg-white/72 px-4 py-3">
      <div className="flex h-11 w-11 items-center justify-center rounded-[16px] bg-primary/10 text-primary">
        <Icon className="h-5 w-5" />
      </div>
      <div>
        <p className="text-xs font-semibold uppercase tracking-[0.18em] text-muted-foreground">
          {label}
        </p>
        <p className="mt-1 text-base font-semibold text-foreground">{value}</p>
      </div>
    </div>
  );
}
