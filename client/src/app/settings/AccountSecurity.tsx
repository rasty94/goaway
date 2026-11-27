import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogTrigger
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { createPasskey, passkeysSupported } from "@/lib/passkeys";
import { DeleteRequest, GetRequest, PostRequest } from "@/util";
import { QRCodeSVG } from "qrcode.react";
import { useEffect, useState } from "react";
import { toast } from "sonner";
import { SettingRow } from "./SettingsRow";

type Setup = { secret: string; otpauthUrl: string };

export function TwoFactorRow() {
  const [enabled, setEnabled] = useState<boolean | null>(null);
  const [open, setOpen] = useState(false);
  const [setup, setSetup] = useState<Setup | null>(null);
  const [code, setCode] = useState("");
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    GetRequest("auth/2fa/status", true).then(([status, data]) => {
      if (status === 200) setEnabled(Boolean(data?.enabled));
    });
  }, []);

  const reset = (next: boolean) => {
    setOpen(next);
    if (!next) {
      setSetup(null);
      setCode("");
    }
  };

  const startSetup = async () => {
    setBusy(true);
    const [status, data] = await PostRequest("auth/2fa/setup", {});
    setBusy(false);
    if (status === 200) setSetup(data);
  };

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy(true);
    const [status] = await PostRequest(
      enabled ? "auth/2fa/disable" : "auth/2fa/enable",
      { code: code.trim() }
    );
    setBusy(false);
    if (status !== 200) return;
    toast.success(enabled ? "Two-factor authentication disabled" : "Two-factor authentication enabled");
    setEnabled(!enabled);
    reset(false);
  };

  // Hidden until the status is known (and on servers without a database).
  if (enabled === null) return null;

  const codeForm = (
    <form onSubmit={submit} className="space-y-3">
      <Label htmlFor="totp-code">6-digit code</Label>
      <Input
        id="totp-code"
        inputMode="numeric"
        autoComplete="one-time-code"
        maxLength={6}
        value={code}
        onChange={(e) => setCode(e.target.value)}
        placeholder="123456"
        autoFocus
      />
      <Button type="submit" className="w-full" disabled={busy || code.trim().length !== 6}>
        {enabled ? "Disable 2FA" : "Verify and enable"}
      </Button>
    </form>
  );

  return (
    <SettingRow
      title="Two-factor authentication"
      description={
        enabled
          ? "Enabled. Logins require a code from your authenticator app."
          : "Require a code from an authenticator app when logging in."
      }
      action={
        <Dialog open={open} onOpenChange={reset}>
          <DialogTrigger asChild>
            <Button variant="outline">{enabled ? "Disable 2FA" : "Enable 2FA"}</Button>
          </DialogTrigger>
          <DialogContent className="sm:max-w-md">
            <DialogHeader>
              <DialogTitle>{enabled ? "Disable two-factor authentication" : "Enable two-factor authentication"}</DialogTitle>
              <DialogDescription>
                {enabled
                  ? "Enter a current code from your authenticator app to turn 2FA off."
                  : setup
                    ? "Scan the QR code, or enter the key manually, then confirm with a code."
                    : "Use an authenticator app such as Aegis, 1Password or Google Authenticator."}
              </DialogDescription>
            </DialogHeader>
            {enabled ? (
              codeForm
            ) : setup ? (
              <div className="space-y-4">
                <div className="flex justify-center rounded-md bg-white p-3">
                  <QRCodeSVG value={setup.otpauthUrl} size={176} title="2FA QR code" />
                </div>
                <div className="space-y-1">
                  <p className="text-xs text-muted-foreground">Setup key</p>
                  <code className="block break-all rounded bg-muted px-2 py-1 text-sm" data-testid="totp-secret">
                    {setup.secret}
                  </code>
                </div>
                {codeForm}
              </div>
            ) : (
              <Button onClick={startSetup} disabled={busy} className="w-full">
                Generate setup key
              </Button>
            )}
          </DialogContent>
        </Dialog>
      }
    />
  );
}

type Passkey = { id: number; name: string; createdAt: string };

export function PasskeysRow() {
  const [available, setAvailable] = useState(false);
  const [keys, setKeys] = useState<Passkey[]>([]);
  const [name, setName] = useState("");
  const [busy, setBusy] = useState(false);

  const load = async () => {
    const [status, data] = await GetRequest("auth/webauthn/credentials");
    if (status === 200) setKeys(data ?? []);
  };

  useEffect(() => {
    GetRequest("auth/webauthn/status", true).then(([status, data]) => {
      setAvailable(status === 200 && Boolean(data?.enabled) && passkeysSupported());
    });
  }, []);

  if (!available) return null;

  const add = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy(true);
    try {
      const created = await createPasskey(name.trim() || "Passkey");
      if (created) {
        toast.success(`Passkey "${created.name}" added`);
        setName("");
        load();
      }
    } finally {
      setBusy(false);
    }
  };

  const remove = async (key: Passkey) => {
    const [status] = await DeleteRequest(`auth/webauthn/credentials/${key.id}`, null);
    if (status === 200) setKeys((prev) => prev.filter((k) => k.id !== key.id));
  };

  return (
    <SettingRow
      title="Passkeys"
      description="Sign in with Touch ID, Windows Hello or a security key instead of a password."
      action={
        <Dialog onOpenChange={(open) => open && load()}>
          <DialogTrigger asChild>
            <Button variant="outline">Manage Passkeys</Button>
          </DialogTrigger>
          <DialogContent className="sm:max-w-md">
            <DialogHeader>
              <DialogTitle>Passkeys</DialogTitle>
              <DialogDescription>Passkeys registered for your account on this server.</DialogDescription>
            </DialogHeader>
            <ul className="divide-y rounded-md border">
              {keys.length === 0 && (
                <li className="p-3 text-sm italic text-muted-foreground">No passkeys yet.</li>
              )}
              {keys.map((key) => (
                <li key={key.id} className="flex items-center justify-between gap-2 p-3">
                  <div>
                    <p className="text-sm font-medium">{key.name}</p>
                    <p className="text-xs text-muted-foreground">
                      Added {new Date(key.createdAt).toLocaleDateString()}
                    </p>
                  </div>
                  <Button variant="ghost" size="sm" onClick={() => remove(key)} aria-label={`Remove ${key.name}`}>
                    Remove
                  </Button>
                </li>
              ))}
            </ul>
            <form onSubmit={add} className="flex gap-2">
              <Input
                aria-label="Passkey name"
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="e.g. MacBook"
                maxLength={128}
              />
              <Button type="submit" disabled={busy}>
                Add passkey
              </Button>
            </form>
          </DialogContent>
        </Dialog>
      }
    />
  );
}
