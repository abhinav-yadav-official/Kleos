"use client";
import { useEffect, useState } from "react";
import { Nav } from "@/components/Nav";
import { Badge, Button, Card, ErrorBanner, Input, Label } from "@/components/ui";
import { ApiException, createSmtp, deleteSmtp, listSmtp, setPrimarySmtp, Smtp, updateSmtp, verifySmtp } from "@/lib/api";

type FormState = {
  label: string;
  host: string;
  port: number;
  username: string;
  password: string;
  from_email: string;
  from_name: string;
  use_tls: boolean;
};

const emptyForm: FormState = {
  label: "primary",
  host: "smtp.gmail.com",
  port: 465,
  username: "",
  password: "",
  from_email: "",
  from_name: "",
  use_tls: true,
};

export default function OnboardingSmtpPage() {
  const [items, setItems] = useState<Smtp[]>([]);
  const [err, setErr] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [form, setForm] = useState<FormState>(emptyForm);

  async function reload() {
    try {
      setItems(await listSmtp());
    } catch (e) {
      setErr(asMessage(e));
    }
  }
  useEffect(() => {
    reload();
  }, []);

  function startEdit(s: Smtp) {
    setEditingId(s.id);
    setForm({
      label: s.label,
      host: s.host,
      port: s.port,
      username: s.username,
      password: "",
      from_email: s.from_email,
      from_name: s.from_name ?? "",
      use_tls: s.use_tls,
    });
  }

  function cancelEdit() {
    setEditingId(null);
    setForm(emptyForm);
    setErr(null);
  }

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setErr(null);
    setBusy(true);
    try {
      const fromEmail = form.from_email || form.username;
      if (editingId) {
        // Editing: drop the password key when blank so the server keeps the
        // existing cipher.
        const { password, ...rest } = form;
        const patch: Partial<FormState> = { ...rest, from_email: fromEmail };
        if (password.trim() !== "") patch.password = password;
        await updateSmtp(editingId, patch);
      } else {
        await createSmtp({ ...form, from_email: fromEmail });
      }
      cancelEdit();
      await reload();
    } catch (e) {
      setErr(asMessage(e));
    } finally {
      setBusy(false);
    }
  }

  async function verify(id: string) {
    setBusy(true);
    setErr(null);
    try {
      const out = await verifySmtp(id);
      if (!out.ok) setErr(`Verify failed: ${out.detail}`);
      await reload();
    } catch (e) {
      setErr(asMessage(e));
    } finally {
      setBusy(false);
    }
  }

  return (
    <>
      <Nav />
      <h1 className="text-xl font-semibold mb-4">Onboarding · SMTP</h1>
      <p className="text-sm text-muted mb-6 max-w-2xl">
        Add the SMTP credential Kleos will send from. Gmail: enable 2FA, create an app password,
        host <span className="font-mono">smtp.gmail.com</span> port <span className="font-mono">465</span>, TLS on.
        ForwardEmail/custom: use port 465 or 587. Editing an existing credential clears its verified
        status; re-verify after saving.
      </p>

      <div className="grid gap-6 md:grid-cols-2">
        <Card>
          <div className="flex items-center justify-between mb-3">
            <div className="font-medium">{editingId ? "Edit credential" : "Add credential"}</div>
            {editingId && (
              <button onClick={cancelEdit} className="text-xs text-muted hover:text-text">
                Cancel
              </button>
            )}
          </div>
          <form onSubmit={submit} className="space-y-3">
            <div>
              <Label>Label</Label>
              <Input value={form.label} onChange={(e) => setForm({ ...form, label: e.target.value })} />
            </div>
            <div className="grid grid-cols-3 gap-2">
              <div className="col-span-2">
                <Label>Host</Label>
                <Input value={form.host} onChange={(e) => setForm({ ...form, host: e.target.value })} required />
              </div>
              <div>
                <Label>Port</Label>
                <Input type="number" value={form.port} onChange={(e) => setForm({ ...form, port: Number(e.target.value) })} required />
              </div>
            </div>
            <div>
              <Label>Username</Label>
              <Input value={form.username} onChange={(e) => setForm({ ...form, username: e.target.value })} required autoComplete="username" />
            </div>
            <div>
              <Label>Password / App password{editingId && " (leave blank to keep existing)"}</Label>
              <Input
                type="password"
                value={form.password}
                onChange={(e) => setForm({ ...form, password: e.target.value })}
                required={!editingId}
                autoComplete="new-password"
              />
            </div>
            <div className="grid grid-cols-2 gap-2">
              <div>
                <Label>From email</Label>
                <Input value={form.from_email} onChange={(e) => setForm({ ...form, from_email: e.target.value })} placeholder="defaults to username" />
              </div>
              <div>
                <Label>From name</Label>
                <Input value={form.from_name} onChange={(e) => setForm({ ...form, from_name: e.target.value })} />
              </div>
            </div>
            <label className="flex items-center gap-2 text-sm">
              <input type="checkbox" checked={form.use_tls} onChange={(e) => setForm({ ...form, use_tls: e.target.checked })} />
              Use TLS (STARTTLS or implicit on 465)
            </label>
            <ErrorBanner message={err} />
            <Button type="submit" disabled={busy} className="w-full">
              {busy ? "Working…" : editingId ? "Save changes" : "Add credential"}
            </Button>
          </form>
        </Card>

        <div className="space-y-2">
          {items.length === 0 && <p className="text-muted text-sm">No credentials yet.</p>}
          {items.map((s) => (
            <Card key={s.id}>
              <div className="flex justify-between items-start gap-3">
                <div>
                  <div className="font-medium">
                    {s.label} <span className="text-muted text-xs">· {s.host}:{s.port}</span>
                  </div>
                  <div className="text-xs text-muted">{s.username}</div>
                  <div className="mt-2 flex gap-2">
                    <Badge tone={s.verified_at ? "ok" : "warn"}>{s.verified_at ? "verified" : "unverified"}</Badge>
                    {s.is_primary && <Badge tone="ok">primary</Badge>}
                  </div>
                  {s.last_error && <pre className="mt-2 text-xs text-err whitespace-pre-wrap">{s.last_error}</pre>}
                </div>
                <div className="flex flex-col gap-2 shrink-0">
                  <Button variant="ghost" onClick={() => verify(s.id)} disabled={busy}>Verify</Button>
                  <Button variant="ghost" onClick={() => startEdit(s)} disabled={busy}>Edit</Button>
                  {!s.is_primary && (
                    <Button variant="ghost" onClick={async () => { await setPrimarySmtp(s.id); reload(); }} disabled={busy}>
                      Make primary
                    </Button>
                  )}
                  <Button variant="danger" onClick={async () => { if (confirm("Delete?")) { await deleteSmtp(s.id); reload(); } }} disabled={busy}>
                    Delete
                  </Button>
                </div>
              </div>
            </Card>
          ))}
        </div>
      </div>
    </>
  );
}

function asMessage(e: unknown): string {
  if (e instanceof ApiException) return e.body.message;
  if (e instanceof Error) return e.message;
  return String(e);
}
