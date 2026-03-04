import { useState, useEffect } from "react";
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
    // Format: scheme://host:port?authcode=xxx
    const match = raw.match(/^(\w+):\/\/([^:/?]+)(?::(\d+))?(?:\?(.*))?$/);
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

interface UrlFieldProps {
  label: string;
  value: string;
  onChange: (value: string) => void;
  networks?: string[];
  localAddresses?: string[];
  isListen?: boolean;
}

export function UrlField({
  label,
  value,
  onChange,
  networks = ["vtcp", "tcp", "ws"],
  localAddresses = [],
  isListen = false,
}: UrlFieldProps) {
  const [advanced, setAdvanced] = useState(false);
  const defaultScheme = networks[0] ?? "tcp";
  const [parts, setParts] = useState<UrlParts>(() =>
    parseUrl(value, defaultScheme),
  );

  useEffect(() => {
    setParts(parseUrl(value, defaultScheme));
  }, [value, defaultScheme]);

  const isLocalScheme = LOCAL_SCHEMES.includes(parts.scheme);
  const isVnetScheme = VNET_SCHEMES.includes(parts.scheme);
  const showAuthcode = isVnetScheme;

  const updatePart = (key: keyof UrlParts, val: string) => {
    const next = { ...parts, [key]: val };
    setParts(next);
    onChange(buildUrl(next));
  };

  const handleSchemeChange = (scheme: string) => {
    const next = { ...parts, scheme };
    const wasLocal = LOCAL_SCHEMES.includes(parts.scheme);
    const nowLocal = LOCAL_SCHEMES.includes(scheme);
    const nowVnet = VNET_SCHEMES.includes(scheme);

    if (isListen) {
      if (nowVnet) {
        // vtcp listen: editable vhostname.alias
        if (wasLocal || parts.host === "local") {
          next.host = "";
        }
      } else if (!nowLocal) {
        next.host = "local";
      } else if (!wasLocal) {
        next.host = localAddresses[0] ?? "0.0.0.0";
      }
    }

    // Clear authcode when switching to local scheme
    if (nowLocal) {
      next.authcode = "";
    }

    setParts(next);
    onChange(buildUrl(next));
  };

  const renderHostField = () => {
    if (isListen && !isLocalScheme && !isVnetScheme) {
      return <Input value="local" readOnly className="h-8 flex-1 bg-muted" />;
    }

    if (isVnetScheme) {
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
        placeholder="地址"
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
            onCheckedChange={setAdvanced}
            className="scale-75"
          />
        </div>
      </div>

      {advanced ? (
        <Input
          value={value}
          onChange={(e) => onChange(e.target.value)}
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
            <Input
              value={parts.port}
              onChange={(e) => updatePart("port", e.target.value)}
              placeholder="端口"
              className="h-8 w-20"
            />
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
