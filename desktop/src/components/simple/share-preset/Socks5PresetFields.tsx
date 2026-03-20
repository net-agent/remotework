import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

export function Socks5PresetFields({
  resolvedAlias,
  name,
  onNameChange,
  username,
  onUsernameChange,
  password,
  onPasswordChange,
}: {
  resolvedAlias: string | null;
  name: string;
  onNameChange: (value: string) => void;
  username: string;
  onUsernameChange: (value: string) => void;
  password: string;
  onPasswordChange: (value: string) => void;
}) {
  return (
    <>
      <div>
        <Label htmlFor="simple-share-name">开放名称</Label>
        <Input
          id="simple-share-name"
          value={name}
          onChange={(event) => onNameChange(event.target.value)}
          placeholder="请输入名称"
          className="mt-1.5"
        />
      </div>

      <div>
        <Label>当前连接信息</Label>
        <div className="mt-1.5 rounded-md border bg-muted/40 px-3 py-2 text-sm">
          {resolvedAlias ?? "未选择"}
        </div>
      </div>

      <div className="rounded-md border bg-muted/30 px-3 py-3 text-xs text-muted-foreground">
        用户名和密码都可以留空，此时将以匿名方式开放本地网络。若需要认证，请两项同时填写。
      </div>
      <div>
        <Label htmlFor="simple-share-username">用户名</Label>
        <Input
          id="simple-share-username"
          value={username}
          onChange={(event) => onUsernameChange(event.target.value)}
          placeholder="可留空"
          className="mt-1.5"
        />
      </div>
      <div>
        <Label htmlFor="simple-share-password">密码</Label>
        <Input
          id="simple-share-password"
          type="password"
          value={password}
          onChange={(event) => onPasswordChange(event.target.value)}
          placeholder="可留空"
          className="mt-1.5"
        />
      </div>
    </>
  );
}
