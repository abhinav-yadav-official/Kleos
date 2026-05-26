"use client";
import { useEffect, useState } from "react";
import Link from "next/link";
import { Nav } from "@/components/Nav";
import { Badge, Button, Card, ErrorBanner, Input, Label } from "@/components/ui";
import { ApiException, CampaignWithCounts, createCampaign, listCampaigns, listResumes, listSmtp, Resume, Smtp } from "@/lib/api";

export default function CampaignsPage() {
  const [items, setItems] = useState<CampaignWithCounts[]>([]);
  const [smtp, setSmtp] = useState<Smtp[]>([]);
  const [resumes, setResumes] = useState<Resume[]>([]);
  const [err, setErr] = useState<string | null>(null);
  const [creating, setCreating] = useState(false);
  const [name, setName] = useState("");
  const [resumeId, setResumeId] = useState("");
  const [smtpId, setSmtpId] = useState("");
  const [busy, setBusy] = useState(false);

  async function reload() {
    try {
      const [cs, ss, rs] = await Promise.all([listCampaigns(), listSmtp(), listResumes()]);
      setItems(cs);
      setSmtp(ss);
      setResumes(rs);
      const active = rs.find((r) => r.is_active) ?? rs[0];
      const primary = ss.find((s) => s.is_primary && s.verified_at) ?? ss.find((s) => s.verified_at) ?? ss[0];
      if (active && !resumeId) setResumeId(active.id);
      if (primary && !smtpId) setSmtpId(primary.id);
    } catch (e) {
      setErr(asMessage(e));
    }
  }
  useEffect(() => {
    reload();
  }, []);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setErr(null);
    try {
      await createCampaign({ name, resume_id: resumeId, smtp_id: smtpId });
      setName("");
      setCreating(false);
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
      <div className="flex justify-between items-center mb-4">
        <h1 className="text-xl font-semibold">Campaigns</h1>
        <Button onClick={() => setCreating((x) => !x)}>{creating ? "Cancel" : "New campaign"}</Button>
      </div>

      {creating && (
        <Card className="mb-4">
          <form onSubmit={submit} className="space-y-3 max-w-xl">
            <div>
              <Label>Name</Label>
              <Input value={name} onChange={(e) => setName(e.target.value)} required />
            </div>
            <div>
              <Label>Resume</Label>
              <select className="block w-full rounded-md border border-border bg-bg px-3 h-9 text-sm" value={resumeId} onChange={(e) => setResumeId(e.target.value)} required>
                <option value="">Select…</option>
                {resumes.map((r) => <option key={r.id} value={r.id}>{r.filename}{r.is_active ? " (active)" : ""}</option>)}
              </select>
            </div>
            <div>
              <Label>SMTP credential</Label>
              <select className="block w-full rounded-md border border-border bg-bg px-3 h-9 text-sm" value={smtpId} onChange={(e) => setSmtpId(e.target.value)} required>
                <option value="">Select…</option>
                {smtp.map((s) => <option key={s.id} value={s.id} disabled={!s.verified_at}>{s.label} · {s.host}{s.verified_at ? "" : " (unverified)"}</option>)}
              </select>
            </div>
            <ErrorBanner message={err} />
            <Button type="submit" disabled={busy}>{busy ? "Creating…" : "Create"}</Button>
          </form>
        </Card>
      )}

      <div className="space-y-2">
        {items.length === 0 && <p className="text-muted text-sm">No campaigns yet.</p>}
        {items.map((c) => (
          <Link key={c.id} href={{ pathname: "/campaigns/view", query: { id: c.id } }} className="block">
            <Card className="hover:border-accent/60">
              <div className="flex justify-between items-center">
                <div>
                  <div className="font-medium">{c.name}</div>
                  <div className="text-xs text-muted">
                    new {c.matches_by_state?.new ?? 0} · email_found {c.matches_by_state?.email_found ?? 0} · generated {c.matches_by_state?.generated ?? 0} · sent {c.matches_by_state?.sent ?? 0} · failed {c.matches_by_state?.failed ?? 0}
                  </div>
                </div>
                <Badge tone={c.status === "active" ? "ok" : c.status === "paused" ? "warn" : "default"}>{c.status}</Badge>
              </div>
            </Card>
          </Link>
        ))}
      </div>
    </>
  );
}

function asMessage(e: unknown): string {
  if (e instanceof ApiException) return e.body.message;
  if (e instanceof Error) return e.message;
  return String(e);
}
