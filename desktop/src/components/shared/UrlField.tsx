import { useState, useEffect, useRef } from "react";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";

const LOCAL_SCHEMES = ["tcp", "tcp4", "tcp6"];
const VNET_SCHEMES = ["vtcp"];
const BUILTIN_SCHEMES = ["socks5"];
const DEFAULT_PORT = "1000";

interface UrlParts {
  scheme: string;
  host: string;
  port: string;
  authcode: string;
}

function parseUrl(raw: string, defaultScheme: string): UrlParts {
  const parts: UrlParts = {
    scheme: defaultScheme,
    host: "",
    port: "",
    authcode: "",
  };
  if (!raw) return parts;
  try {
    // Format: scheme://host:port?authcode=xxx (host can be empty)
    const match = raw.match(/^(\w+):\/\/([^:/?]*)(?::(\d+))?(?:\?(.*))?$/);
    if (match) {
      parts.scheme = match[1];
      parts.host = match[2];
      parts.port = match[3] ?? "";
      const queryStr = match[4] ?? "";
      if (queryStr) {
        const params = new URLSearchParams(queryStr);
        parts.authcode = params.get("authcode") ?? "";
      }
    }
  } catch {
    // ignore
  }
  return parts;
}

function buildUrl(parts: UrlParts): string {
  let url = `${parts.scheme}://${parts.host}`;
  if (parts.port) url += `:${parts.port}`;
  if (parts.authcode) url += `?authcode=${parts.authcode}`;
  return url;
}

/** 从 "vhost.alias" 中拆分出 vhost 和 alias */
function splitVnetHost(host: string): { vhost: string; alias: string } {
  const dot = host.lastIndexOf(".");
  if (dot < 0) return { vhost: host, alias: "" };
  return { vhost: host.substring(0, dot), alias: host.substring(dot + 1) };
}

/** 该协议是否需要端口号 */
function schemeNeedsPort(scheme: string): boolean {
  return !BUILTIN_SCHEMES.includes(scheme);
}

interface UrlFieldProps {
  label: string;
  value: string;
  onChange: (value: string) => void;
  networks?: string[];
  localAddresses?: string[];
  linkAliases?: string[];
  /** alias → domain 映射，仅 isListen + vtcp 时用于自动填充 vhost */
  linkDomains?: Record<string, string>;
  isListen?: boolean;
}

export function UrlField({
  label,
  value,
  onChange,
  networks = ["vtcp", "tcp", "ws"],
  localAddresses = [],
  linkAliases = [],
  linkDomains = {},
  isListen = false,
}: UrlFieldProps) {
  const [advanced, setAdvanced] = useState(false);
  const defaultScheme = networks[0] ?? "tcp";
  const [parts, setParts] = useState<UrlParts>(() =>
    parseUrl(value, defaultScheme),
  );

  // 追踪组件自身发出的最新 URL，避免 useEffect 回环覆盖本地状态
  const lastEmittedRef = useRef(value);

  useEffect(() => {
    // 仅在外部 value 变化时同步，跳过自身发出的变更
    if (value !== lastEmittedRef.current) {
      setParts(parseUrl(value, defaultScheme));
      lastEmittedRef.current = value;
    }
  }, [value, defaultScheme]);

  const isLocalScheme = LOCAL_SCHEMES.includes(parts.scheme);
  const isVnetScheme = VNET_SCHEMES.includes(parts.scheme);
  const isBuiltinScheme = BUILTIN_SCHEMES.includes(parts.scheme);
  const showAuthcode = isVnetScheme;
  const showPort = schemeNeedsPort(parts.scheme);

  const emitChange = (next: UrlParts) => {
    setParts(next);
    const url = buildUrl(next);
    lastEmittedRef.current = url;
    onChange(url);
  };

  const updatePart = (key: keyof UrlParts, val: string) => {
    emitChange({ ...parts, [key]: val });
  };

  const handleSchemeChange = (scheme: string) => {
    const next = { ...parts, scheme };
    const wasLocal = LOCAL_SCHEMES.includes(parts.scheme);
    const wasVnet = VNET_SCHEMES.includes(parts.scheme);
    const nowLocal = LOCAL_SCHEMES.includes(scheme);
    const nowVnet = VNET_SCHEMES.includes(scheme);
    const nowBuiltin = BUILTIN_SCHEMES.includes(scheme);

    if (isListen) {
      // --- Listen 端 ---
      if (nowVnet) {
        if (wasLocal || parts.host === "local") {
          next.host = "";
        }
      } else if (!nowLocal) {
        next.host = "local";
      } else if (!wasLocal) {
        next.host = localAddresses[0] ?? "0.0.0.0";
      }
    } else {
      // --- Target 端 ---
      if (nowBuiltin) {
        // socks5 等内置服务：host 固定，清端口
        next.host = "local";
        next.port = "";
        next.authcode = "";
      } else if (nowVnet && !wasVnet) {
        // 切入 vtcp：清掉不兼容的 host（如 IP 地址）
        next.host = "";
      } else if (nowLocal && !wasLocal) {
        // 切入 tcp：清掉不兼容的 host（如 vhost.alias）
        next.host = "";
      }
    }

    // 切入需要端口的协议时，如果端口为空则给默认值
    if (schemeNeedsPort(scheme) && !next.port) {
      next.port = DEFAULT_PORT;
    }

    // 切入本地协议时清 authcode
    if (nowLocal || nowBuiltin) {
      next.authcode = "";
    }

    emitChange(next);
  };

  const renderHostField = () => {
    // 内置服务（socks5）：只读
    if (isBuiltinScheme) {
      return (
        <Input
          value="内置代理"
          readOnly
          className="h-8 flex-1 bg-muted text-muted-foreground"
        />
      );
    }

    if (isListen && !isLocalScheme && !isVnetScheme) {
      return <Input value="local" readOnly className="h-8 flex-1 bg-muted" />;
    }

    if (isVnetScheme) {
      const { vhost, alias } = splitVnetHost(parts.host);

      // Listen 模式：vhost 由 link domain 自动决定，用户只需选择链路
      if (isListen && linkAliases.length > 0) {
        const handleAliasChange = (newAlias: string) => {
          const domain = linkDomains[newAlias] ?? newAlias;
          updatePart("host", `${domain}.${newAlias}`);
        };

        return (
          <div className="flex flex-1 gap-1">
            <Input
              value={vhost}
              readOnly
              placeholder="domain"
              className="h-8 flex-1 bg-muted"
              title="由所选链路的 domain 自动决定"
            />
            <span className="flex items-center text-muted-foreground">.</span>
            <Select value={alias} onValueChange={handleAliasChange}>
              <SelectTrigger className="h-8 w-28">
                <SelectValue placeholder="链路" />
              </SelectTrigger>
              <SelectContent>
                {linkAliases.map((a) => (
                  <SelectItem key={a} value={a}>
                    {a}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        );
      }

      // Target 模式：vhost 可编辑（远端 domain），alias 下拉选择
      if (linkAliases.length > 0) {
        const updateVnetHost = (newVhost: string, newAlias: string) => {
          const host = newAlias ? `${newVhost}.${newAlias}` : newVhost;
          updatePart("host", host);
        };

        return (
          <div className="flex flex-1 gap-1">
            <Input
              value={vhost}
              onChange={(e) => updateVnetHost(e.target.value, alias)}
              placeholder="远端 domain"
              className="h-8 flex-1"
            />
            <span className="flex items-center text-muted-foreground">.</span>
            <Select
              value={alias}
              onValueChange={(v) => updateVnetHost(vhost, v)}
            >
              <SelectTrigger className="h-8 w-28">
                <SelectValue placeholder="链路" />
              </SelectTrigger>
              <SelectContent>
                {linkAliases.map((a) => (
                  <SelectItem key={a} value={a}>
                    {a}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        );
      }

      // 无链路信息时回退为普通输入
      return (
        <Input
          value={parts.host}
          onChange={(e) => updatePart("host", e.target.value)}
          placeholder="vhost.alias"
          className="h-8 flex-1"
        />
      );
    }

    if (isListen && isLocalScheme && localAddresses.length > 0) {
      return (
        <Select value={parts.host} onValueChange={(v) => updatePart("host", v)}>
          <SelectTrigger className="h-8 flex-1">
            <SelectValue placeholder="地址" />
          </SelectTrigger>
          <SelectContent>
            {localAddresses.map((addr) => (
              <SelectItem key={addr} value={addr}>
                {addr}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      );
    }

    return (
      <Input
        value={parts.host}
        onChange={(e) => updatePart("host", e.target.value)}
        placeholder={isListen ? "地址" : "目标地址"}
        className="h-8 flex-1"
      />
    );
  };

  const renderAuthcodeWarning = () => {
    if (!isVnetScheme || parts.authcode) return null;
    return (
      <p className="text-xs text-destructive">
        vtcp 端点必须设置认证码 (authcode)
      </p>
    );
  };

  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between">
        <Label>{label}</Label>
        <div className="flex items-center gap-1.5">
          <span className="text-sm text-muted-foreground">高级</span>
          <Switch
            checked={advanced}
            onCheckedChange={(checked) => {
              if (!checked) {
                // 切回结构化模式时，从当前 value 重新解析 parts
                setParts(parseUrl(value, defaultScheme));
              }
              setAdvanced(checked);
            }}
            className="scale-75"
          />
        </div>
      </div>

      {advanced ? (
        <Input
          value={value}
          onChange={(e) => {
            const v = e.target.value;
            lastEmittedRef.current = v;
            onChange(v);
          }}
          placeholder="vtcp://vhost.alias:port?authcode=xxx"
          className="font-mono text-sm"
        />
      ) : (
        <div className="grid gap-2">
          <div className="flex gap-2">
            <Select value={parts.scheme} onValueChange={handleSchemeChange}>
              <SelectTrigger className="w-24 h-8">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {networks.map((s) => (
                  <SelectItem key={s} value={s}>
                    {s}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            {renderHostField()}
            {showPort && (
              <Input
                value={parts.port}
                onChange={(e) => updatePart("port", e.target.value)}
                placeholder={DEFAULT_PORT}
                className="h-8 w-20"
              />
            )}
          </div>
          {showAuthcode && (
            <Input
              value={parts.authcode}
              onChange={(e) => updatePart("authcode", e.target.value)}
              placeholder="认证码 (authcode)"
              type="password"
              className="h-8"
            />
          )}
          {renderAuthcodeWarning()}
          {value && (
            <p
              className="text-xs text-muted-foreground font-mono truncate"
              title={value}
            >
              {value}
            </p>
          )}
        </div>
      )}
    </div>
  );
}
