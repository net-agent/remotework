import { useState, useEffect } from "react";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";

interface UrlParts {
  scheme: string;
  host: string;
  port: string;
  domain: string;
  secret: string;
  path: string;
}

function parseUrl(raw: string): UrlParts {
  const parts: UrlParts = { scheme: "vtcp", host: "", port: "", domain: "", secret: "", path: "" };
  if (!raw) return parts;
  try {
    // Format: scheme://domain:secret@host:port/path
    const match = raw.match(/^(\w+):\/\/(?:([^:@]+)(?::([^@]*))?@)?([^:/]+)(?::(\d+))?(\/.*)?$/);
    if (match) {
      parts.scheme = match[1];
      parts.domain = match[2] ?? "";
      parts.secret = match[3] ?? "";
      parts.host = match[4];
      parts.port = match[5] ?? "";
      parts.path = match[6] ?? "";
    }
  } catch {
    // ignore
  }
  return parts;
}

function buildUrl(parts: UrlParts): string {
  let url = `${parts.scheme}://`;
  if (parts.domain) {
    url += parts.domain;
    if (parts.secret) url += `:${parts.secret}`;
    url += "@";
  }
  url += parts.host;
  if (parts.port) url += `:${parts.port}`;
  if (parts.path) url += parts.path;
  return url;
}

interface UrlFieldProps {
  label: string;
  value: string;
  onChange: (value: string) => void;
  showDomain?: boolean;
  showSecret?: boolean;
  schemes?: string[];
}

export function UrlField({
  label,
  value,
  onChange,
  showDomain = false,
  showSecret = false,
  schemes = ["vtcp", "tcp", "ws"],
}: UrlFieldProps) {
  const [advanced, setAdvanced] = useState(false);
  const [parts, setParts] = useState<UrlParts>(() => parseUrl(value));

  useEffect(() => {
    setParts(parseUrl(value));
  }, [value]);

  const updatePart = (key: keyof UrlParts, val: string) => {
    const next = { ...parts, [key]: val };
    setParts(next);
    onChange(buildUrl(next));
  };

  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between">
        <Label>{label}</Label>
        <div className="flex items-center gap-1.5">
          <span className="text-xs text-muted-foreground">高级</span>
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
          placeholder="vtcp://domain:secret@host:port"
          className="font-mono text-xs"
        />
      ) : (
        <div className="grid gap-2">
          <div className="flex gap-2">
            <Select value={parts.scheme} onValueChange={(v) => updatePart("scheme", v)}>
              <SelectTrigger className="w-24 h-8">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {schemes.map((s) => (
                  <SelectItem key={s} value={s}>{s}</SelectItem>
                ))}
              </SelectContent>
            </Select>
            <Input
              value={parts.host}
              onChange={(e) => updatePart("host", e.target.value)}
              placeholder="地址"
              className="h-8 flex-1"
            />
            <Input
              value={parts.port}
              onChange={(e) => updatePart("port", e.target.value)}
              placeholder="端口"
              className="h-8 w-20"
            />
          </div>
          {showDomain && (
            <Input
              value={parts.domain}
              onChange={(e) => updatePart("domain", e.target.value)}
              placeholder="域名"
              className="h-8"
            />
          )}
          {showSecret && (
            <Input
              value={parts.secret}
              onChange={(e) => updatePart("secret", e.target.value)}
              placeholder="密码"
              type="password"
              className="h-8"
            />
          )}
          {parts.scheme === "ws" && (
            <Input
              value={parts.path}
              onChange={(e) => updatePart("path", e.target.value)}
              placeholder="路径 (例如 /ws)"
              className="h-8"
            />
          )}
        </div>
      )}
    </div>
  );
}
