import { useState, useEffect, useRef } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { StatusDot } from "@/components/shared/StatusDot";
import { InfoRow } from "@/components/shared/InfoRow";
import { useAgentStore } from "@/stores/agent-store";
import { useProfileStore } from "@/stores/profile-store";
import { useUIStore } from "@/stores/ui-store";
import { Pencil, Loader2 } from "lucide-react";
import * as api from "@/lib/api";
import type { NetworkStateDTO, PingResultDTO } from "@/lib/types";

const PING_INTERVAL = 15_000;
const COUNTDOWN_TICK = 100;
const ONLINE_DURATION_TICK = 1_000;
const MAX_MANUAL_RESULTS = 10;

function PingCountdownRing({
  durationMs,
  remainingMs,
  refreshing,
}: {
  durationMs: number;
  remainingMs: number;
  refreshing: boolean;
}) {
  const size = 14;
  const viewBoxSize = 18;
  const center = viewBoxSize / 2;
  const strokeWidth = 1.75;
  const radius = 7;
  const circumference = 2 * Math.PI * radius;
  const progress = Math.min(Math.max(remainingMs / durationMs, 0), 1);
  const dashOffset = circumference * (1 - progress);
  const seconds = Math.max(1, Math.ceil(remainingMs / 1000));

  return (
    <div
      className={`text-muted-foreground/60 transition-opacity duration-200 ${refreshing ? "animate-pulse opacity-100" : "opacity-80"}`}
      title={`距离下次刷新还有 ${seconds}s`}
    >
      <svg
        width={size}
        height={size}
        viewBox={`0 0 ${viewBoxSize} ${viewBoxSize}`}
      >
        <circle
          cx={center}
          cy={center}
          r={radius}
          fill="none"
          stroke="currentColor"
          strokeWidth={strokeWidth}
          className="opacity-20"
        />
        <circle
          cx={center}
          cy={center}
          r={radius}
          fill="none"
          stroke="currentColor"
          strokeWidth={strokeWidth}
          strokeLinecap="round"
          strokeDasharray={circumference}
          strokeDashoffset={-dashOffset}
          className="-rotate-90 origin-center transition-[stroke-dashoffset] duration-100 ease-linear"
        />
      </svg>
    </div>
  );
}

function formatOnlineDuration(ms: number): string {
  if (ms <= 0) return "-";

  const totalSeconds = Math.floor(ms / 1000);
  const hours = Math.floor(totalSeconds / 3600);
  const minutes = Math.floor((totalSeconds % 3600) / 60);
  const seconds = totalSeconds % 60;

  if (hours > 0) {
    return `${hours}h ${minutes}m ${seconds}s`;
  }

  if (minutes > 0) {
    return `${minutes}m ${seconds}s`;
  }

  return `${seconds}s`;
}

function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

/** 解析延迟字符串为毫秒数，失败返回 -1 */
function parseLatencyMs(result: string): number {
  if (result === "<1ms") return 0.5;
  const msMatch = result.match(/^(\d+)ms$/);
  if (msMatch) return parseInt(msMatch[1], 10);
  const sMatch = result.match(/^([\d.]+)s$/);
  if (sMatch) return parseFloat(sMatch[1]) * 1000;
  return -1;
}

/** 根据延迟返回颜色 class */
function latencyColor(result: string): string {
  const ms = parseLatencyMs(result);
  if (ms < 0) return "text-destructive";
  if (ms < 100) return "text-green-500";
  if (ms < 500) return "text-yellow-500";
  return "text-red-500";
}

function formatTime(date: Date): string {
  return date.toLocaleTimeString("zh-CN", { hour12: false });
}

interface ManualPingResult {
  domain: string;
  result: string;
  error?: boolean;
  time: Date;
}

export function NetworkDetail({ network }: { network: NetworkStateDTO }) {
  const { streams } = useAgentStore();
  const { openLinkForm } = useUIStore();
  const { currentConfig } = useProfileStore();

  const [pingDomain, setPingDomain] = useState("");
  const [pinging, setPinging] = useState(false);
  const [manualResults, setManualResults] = useState<ManualPingResult[]>([]);
  const [autoResults, setAutoResults] = useState<PingResultDTO[]>([]);
  const [autoLoading, setAutoLoading] = useState(false);
  const [remainingMs, setRemainingMs] = useState(PING_INTERVAL);
  const [onlineDurationMs, setOnlineDurationMs] = useState(network.aliveMs);
  const timerRef = useRef<ReturnType<typeof setInterval>>();
  const countdownRef = useRef<ReturnType<typeof setInterval>>();
  const onlineDurationRef = useRef<ReturnType<typeof setInterval>>();
  const nextRefreshAtRef = useRef<number>(Date.now() + PING_INTERVAL);

  const isOnline = network.state === "online";
  const isTcp =
    network.name === "tcp" ||
    network.name === "tcp4" ||
    network.name === "tcp6";
  const showPingCountdown = !isTcp && autoResults.length > 0;
  const hasLink = !isTcp && network.name in (currentConfig.links ?? {});

  const networkStreams = streams.filter(
    (s) => s.network === network.name && !s.isClosed,
  );

  useEffect(() => {
    clearInterval(onlineDurationRef.current);
    setOnlineDurationMs(network.aliveMs);

    if (!isOnline || network.aliveMs <= 0) {
      return;
    }

    onlineDurationRef.current = setInterval(() => {
      setOnlineDurationMs((prev) => prev + ONLINE_DURATION_TICK);
    }, ONLINE_DURATION_TICK);

    return () => {
      clearInterval(onlineDurationRef.current);
    };
  }, [isOnline, network.aliveMs]);

  useEffect(() => {
    if (isTcp) return;

    let active = true;
    const LOCAL_DOMAINS = new Set(["localhost", "127.0.0.1", "0.0.0.0", "::1"]);

    const fetchAutoPing = async () => {
      if (!active) return;
      setAutoLoading(true);
      try {
        const all = await api.ping();
        if (!active) return;
        setAutoResults(
          all.filter(
            (r) =>
              r.network === network.name &&
              !LOCAL_DOMAINS.has(r.domain) &&
              r.domain !== network.domain,
          ),
        );
      } catch {
        // 静默失败，保留上次结果
      } finally {
        if (!active) return;
        nextRefreshAtRef.current = Date.now() + PING_INTERVAL;
        setRemainingMs(PING_INTERVAL);
        setAutoLoading(false);
      }
    };

    nextRefreshAtRef.current = Date.now() + PING_INTERVAL;
    setRemainingMs(PING_INTERVAL);
    fetchAutoPing();
    timerRef.current = setInterval(fetchAutoPing, PING_INTERVAL);
    countdownRef.current = setInterval(() => {
      if (!active) return;
      setRemainingMs(Math.max(0, nextRefreshAtRef.current - Date.now()));
    }, COUNTDOWN_TICK);
    return () => {
      active = false;
      clearInterval(timerRef.current);
      clearInterval(countdownRef.current);
    };
  }, [isTcp, network.name, network.domain]);

  // 手动 Ping
  const handlePing = async () => {
    const domain = pingDomain.trim();
    if (!domain) return;
    setPinging(true);
    try {
      const res = await api.pingSingle(network.name, domain);
      setManualResults((prev) =>
        [{ domain, result: res.result, time: new Date() }, ...prev].slice(
          0,
          MAX_MANUAL_RESULTS,
        ),
      );
    } catch (e) {
      setManualResults((prev) =>
        [
          { domain, result: String(e), error: true, time: new Date() },
          ...prev,
        ].slice(0, MAX_MANUAL_RESULTS),
      );
    } finally {
      setPinging(false);
    }
  };

  return (
    <div className="p-4 space-y-3 overflow-auto h-full text-xs">
      <div className="flex items-center gap-2">
        <StatusDot state={network.state} />
        <span className="text-sm font-semibold">{network.name}</span>
        <span className="px-1.5 py-0.5 rounded bg-muted text-muted-foreground capitalize">
          {network.state}
        </span>
        {!isTcp && hasLink && (
          <Button
            variant="ghost"
            size="icon"
            className="h-6 w-6 ml-auto"
            onClick={() => openLinkForm(network.name)}
          >
            <Pencil className="h-3.5 w-3.5" />
          </Button>
        )}
      </div>

      {network.lastErr && (
        <div className="px-2.5 py-1.5 rounded bg-destructive/10 text-destructive">
          {network.lastErr}
        </div>
      )}

      <div className="rounded-md border border-border/60 px-3">
        <InfoRow label="协议" value={network.protocol} mono />
        <InfoRow label="地址" value={network.address} mono />
        {network.domain && <InfoRow label="域名" value={network.domain} mono />}
        <InfoRow
          label="在线时长"
          value={formatOnlineDuration(onlineDurationMs)}
        />
        <InfoRow label="监听" value={String(network.listens)} />
        <InfoRow label="拨号" value={String(network.dials)} />
      </div>

      {!isTcp && (
        <div className="rounded-md border border-border/60 px-3">
          {/* 自动 Ping 监控 */}
          <div className="flex items-center gap-1.5 py-1.5 border-b border-border/40">
            <span className="text-muted-foreground flex-1">
              Ping 监控
              {autoLoading && (
                <Loader2 className="inline h-3 w-3 animate-spin ml-1.5 align-text-bottom" />
              )}
            </span>
            {showPingCountdown && (
              <PingCountdownRing
                durationMs={PING_INTERVAL}
                remainingMs={remainingMs}
                refreshing={autoLoading}
              />
            )}
          </div>
          {autoResults.length > 0 ? (
            autoResults.map((r) => (
              <div
                key={r.domain}
                className="flex items-center gap-2 py-1.5 border-b border-border/40 last:border-0"
              >
                <span className="text-muted-foreground truncate">
                  {r.domain}
                </span>
                {r.usedServices.length > 0 && (
                  <span className="text-muted-foreground/50 truncate shrink-0 text-[10px]">
                    ({r.usedServices.join(", ")})
                  </span>
                )}
                <span
                  className={`font-mono text-right ml-auto shrink-0 tabular-nums ${latencyColor(r.result)}`}
                >
                  {r.result}
                </span>
              </div>
            ))
          ) : (
            <div className="py-1.5 text-muted-foreground/50 border-b border-border/40 last:border-0">
              暂无服务依赖此网络
            </div>
          )}

          {/* 手动测试 */}
          <div className="py-1.5 border-b border-border/40 text-muted-foreground">
            手动测试
          </div>
          <div className="flex items-center gap-1.5 py-1.5 border-b border-border/40">
            <Input
              value={pingDomain}
              onChange={(e) => setPingDomain(e.target.value)}
              onKeyDown={(e) => e.key === "Enter" && handlePing()}
              placeholder={network.domain || "输入域名..."}
              className="h-6 text-xs flex-1 min-w-0"
            />
            <Button
              size="sm"
              variant="ghost"
              className="h-6 px-2.5 shrink-0 text-xs"
              disabled={pinging || !pingDomain.trim()}
              onClick={handlePing}
            >
              {pinging ? <Loader2 className="h-3 w-3 animate-spin" /> : "Ping"}
            </Button>
          </div>
          {manualResults.length > 0 && (
            <div className="max-h-[240px] overflow-auto">
              {manualResults.map((r, i) => (
                <div
                  key={i}
                  className={`flex items-center gap-2 py-1.5 border-b border-border/40 last:border-0 ${
                    r.error ? "text-destructive" : ""
                  }`}
                >
                  <span className="text-muted-foreground/50 font-mono shrink-0 tabular-nums">
                    {formatTime(r.time)}
                  </span>
                  <span className="text-muted-foreground truncate">
                    {r.domain}
                  </span>
                  <span
                    className={`font-mono text-right ml-auto shrink-0 tabular-nums ${r.error ? "" : latencyColor(r.result)}`}
                  >
                    {r.result}
                  </span>
                </div>
              ))}
              <div className="py-1 text-right">
                <button
                  className="text-muted-foreground/50 hover:text-muted-foreground text-[10px]"
                  onClick={() => setManualResults([])}
                >
                  清除记录
                </button>
              </div>
            </div>
          )}
        </div>
      )}

      {networkStreams.length > 0 && (
        <div className="space-y-1.5">
          <span className="font-medium text-muted-foreground">
            活跃连接 ({networkStreams.length})
          </span>
          <div className="space-y-0.5">
            {networkStreams.map((s, i) => (
              <div
                key={i}
                className="flex items-center gap-2 font-mono px-2.5 py-1 rounded bg-accent/30"
              >
                <span className="truncate">{s.localAddr}</span>
                <span className="text-muted-foreground shrink-0">→</span>
                <span className="truncate">{s.remoteAddr}</span>
                <span className="ml-auto text-muted-foreground shrink-0 tabular-nums">
                  ↑{formatBytes(s.writeBytes)} ↓{formatBytes(s.readBytes)}
                </span>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}
