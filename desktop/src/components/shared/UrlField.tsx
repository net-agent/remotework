import { useState, useEffect } from "react";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";

const LOCAL_SCHEMES = ["tcp", "tcp4", "tcp6"];

interface UrlParts {
  scheme: string;
  host: string;
  port: string;
  secret: string;
}

function parseUrl(raw: string, defaultScheme: string): UrlParts {
  const parts: UrlParts = { scheme: defaultScheme, host: "", port: "", secret: "" };
  if (!raw) return parts;
  try {
    // Format: scheme://host:port?secret=xxx
    const match = raw.match(/^(\w+):\/\/([^:/?]+)(?::(\d+))?(?:\?(.*))?$/);
    if (match) {
      parts.scheme = match[1];
      parts.host = match[2];
      parts.port = match[3] ?? "";
      const queryStr = match[4] ?? "";
      if (queryStr) {
        const params = new URLSearchParams(queryStr);
        parts.secret = params.get("secret") ?? "";
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
  if (parts.secret) url += `?secret=${parts.secret}`;
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
  const [parts, setParts] = useState<UrlParts>(() => parseUrl(value, defaultScheme));

  useEffect(() => {
    setParts(parseUrl(value, defaultScheme));
  }, [value, defaultScheme]);

  const isLocalScheme = LOCAL_SCHEMES.includes(parts.scheme);
  const showSecret = !isLocalScheme;

  const updatePart = (key: keyof UrlParts, val: string) => {
    const next = { ...parts, [key]: val };
    setParts(next);
    onChange(buildUrl(next));
  };

  const handleSchemeChange = (scheme: string) => {
    const next = { ...parts, scheme };
    const wasLocal = LOCAL_SCHEMES.includes(parts.scheme);
    const nowLocal = LOCAL_SCHEMES.includes(scheme);

    if (isListen) {
      if (!nowLocal) {
        next.host = "local";
      } else if (!wasLocal) {
        next.host = localAddresses[0] ?? "0.0.0.0";
      }
    }

    // Clear secret when switching to local scheme
    if (nowLocal) {
      next.secret = "";
    }

    setParts(next);
    onChange(buildUrl(next));
  };

  const renderHostField = () => {
    if (isListen && !isLocalScheme) {
      return (
        <Input
          value="local"
          readOnly
          className="h-8 flex-1 bg-muted"
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
              <SelectItem key={addr} value={addr}>{addr}</SelectItem>
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

  const renderSecretWarning = () => {
    if (isLocalScheme || parts.secret) return null;
    if (isListen) {
      return (
        <p className="text-xs text-destructive">
          ⚠ 虚拟网络监听必须设置传输密码
        </p>
      );
    }
    return (
      <p className="text-xs text-amber-500">
        ⚠ 虚拟网络建议设置传输密码
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
          placeholder="vtcp://host:port?secret=xxx"
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
                  <SelectItem key={s} value={s}>{s}</SelectItem>
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
          {showSecret && (
            <Input
              value={parts.secret}
              onChange={(e) => updatePart("secret", e.target.value)}
              placeholder="传输密码"
              type="password"
              className="h-8"
            />
          )}
          {renderSecretWarning()}
          {value && (
            <p className="text-xs text-muted-foreground font-mono truncate" title={value}>
              {value}
            </p>
          )}
        </div>
      )}
    </div>
  );
}
