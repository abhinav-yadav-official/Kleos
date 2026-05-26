"use client";
import { Suspense, useEffect, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { Nav } from "@/components/Nav";
import { Badge, Button, Card, ErrorBanner } from "@/components/ui";
import {
  ApiException,
  CampaignWithCounts,
  DraftRow,
  getCampaign,
  listDrafts,
  listMatches,
  listSent,
  MatchRow,
  Resume,
  SentRow,
  Smtp,
  setCampaignStatus,
} from "@/lib/api";

type Tab = "matches" | "drafts" | "sent" | "settings";

export default function CampaignDetailPage() {
  return (
    <Suspense fallback={null}>
      <CampaignDetail />
    </Suspense>
  );
}

function CampaignDetail() {
  const params = useSearchParams();
  const router = useRouter();
  const id = params.get("id") || "";
  const [tab, setTab] = useState<Tab>("matches");
  const [campaign, setCampaign] = useState<CampaignWithCounts | null>(null);
  const [err, setErr] = useState<string | null>(null);

  useEffect(() => {
    if (!id) {
      router.replace("/campaigns");
      return;
    }
    getCampaign(id).then(setCampaign).catch((e) => setErr(asMessage(e)));
  }, [id, router]);

  if (!campaign) {
    return (
      <>
        <Nav />
        <p className="text-muted">Loading…</p>
        <ErrorBanner message={err} />
      </>
    );
  }

  return (
    <>
      <Nav />
      <div className="flex items-center justify-between mb-4">
        <div>
          <h1 className="text-xl font-semibold">{campaign.name}</h1>
          <div className="text-xs text-muted mt-1">id {campaign.id}</div>
        </div>
        <Badge tone={campaign.status === "active" ? "ok" : campaign.status === "paused" ? "warn" : "default"}>{campaign.status}</Badge>
      </div>

      <div className="flex gap-2 mb-4 text-sm">
        {(["matches", "drafts", "sent", "settings"] as Tab[]).map((t) => (
          <button
            key={t}
            onClick={() => setTab(t)}
            className={`rounded px-3 py-1 ${tab === t ? "bg-border/60 text-text" : "text-muted hover:text-text"}`}
          >
            {t}
          </button>
        ))}
      </div>

      {tab === "matches" && <MatchesPane campaignId={campaign.id} />}
      {tab === "drafts" && <DraftsPane campaignId={campaign.id} />}
      {tab === "sent" && <SentPane campaignId={campaign.id} />}
      {tab === "settings" && (
        <SettingsPane
          campaign={campaign}
          onChange={(c) => setCampaign({ ...campaign, ...c })}
        />
      )}
    </>
  );
}

function MatchesPane({ campaignId }: { campaignId: string }) {
  const [rows, setRows] = useState<MatchRow[] | null>(null);
  const [state, setState] = useState<string>("");
  const [err, setErr] = useState<string | null>(null);

  useEffect(() => {
    listMatches(campaignId, state || undefined, 100).then(setRows).catch((e) => setErr(asMessage(e)));
  }, [campaignId, state]);

  const states = ["", "new", "finding_email", "email_found", "email_missing", "generating", "generated", "queued", "sent", "failed", "skipped"];

  return (
    <>
      <div className="flex flex-wrap gap-1 mb-3">
        {states.map((s) => (
          <button
            key={s || "all"}
            onClick={() => setState(s)}
            className={`text-xs rounded-full border px-2 py-0.5 ${state === s ? "border-accent text-accent" : "border-border text-muted hover:text-text"}`}
          >
            {s || "all"}
          </button>
        ))}
      </div>
      <ErrorBanner message={err} />
      <div className="border border-border rounded-md overflow-hidden">
        <table className="w-full text-sm">
          <thead className="bg-panel text-muted text-left">
            <tr><th className="px-3 py-2">Score</th><th className="px-3 py-2">State</th><th className="px-3 py-2">Title</th><th className="px-3 py-2">Company</th><th className="px-3 py-2">Loc</th></tr>
          </thead>
          <tbody>
            {rows?.length === 0 && <tr><td colSpan={5} className="px-3 py-3 text-muted">No matches.</td></tr>}
            {rows?.map((r) => (
              <tr key={r.id} className="border-t border-border">
                <td className="px-3 py-2 font-mono">{r.match_score.toFixed(2)}</td>
                <td className="px-3 py-2"><Badge tone={stateTone(r.state)}>{r.state}</Badge></td>
                <td className="px-3 py-2">
                  <a href={r.job_url} target="_blank" rel="noreferrer" className="hover:text-accent">{r.job_title}</a>
                </td>
                <td className="px-3 py-2 text-muted">{r.company_name}</td>
                <td className="px-3 py-2 text-muted">{r.job_remote ? "remote" : r.job_location}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </>
  );
}

function DraftsPane({ campaignId }: { campaignId: string }) {
  const [rows, setRows] = useState<DraftRow[] | null>(null);
  const [err, setErr] = useState<string | null>(null);

  useEffect(() => {
    listDrafts(campaignId, 60).then(setRows).catch((e) => setErr(asMessage(e)));
  }, [campaignId]);

  // group by match_id
  const grouped: Record<string, DraftRow[]> = {};
  for (const r of rows ?? []) {
    (grouped[r.match_id] ||= []).push(r);
  }

  return (
    <>
      <ErrorBanner message={err} />
      <div className="space-y-3">
        {Object.values(grouped).length === 0 && <p className="text-muted text-sm">No drafts yet. Run worker-contentgen.</p>}
        {Object.values(grouped).map((variants) => {
          const chosen = variants.find((v) => v.chosen) ?? variants[0];
          return (
            <Card key={chosen.match_id}>
              <div className="text-xs text-muted">{chosen.company_name} · {chosen.job_title} → {chosen.recruiter_email}</div>
              <div className="font-medium mt-1">{chosen.subject}</div>
              <pre className="mt-2 whitespace-pre-wrap text-sm">{chosen.body_text}</pre>
              <div className="text-xs text-muted mt-2">chosen · spam_score {chosen.spam_score.toFixed(2)}</div>
              {variants.length > 1 && (
                <details className="mt-3">
                  <summary className="text-xs text-muted cursor-pointer">Show {variants.length - 1} alternates</summary>
                  <div className="space-y-3 mt-2">
                    {variants.filter((v) => !v.chosen).map((v) => (
                      <div key={v.id} className="border-t border-border pt-2">
                        <div className="font-medium text-sm">{v.subject}</div>
                        <pre className="mt-1 whitespace-pre-wrap text-xs text-muted">{v.body_text}</pre>
                        <div className="text-xs text-muted mt-1">spam_score {v.spam_score.toFixed(2)}</div>
                      </div>
                    ))}
                  </div>
                </details>
              )}
            </Card>
          );
        })}
      </div>
    </>
  );
}

function SentPane({ campaignId }: { campaignId: string }) {
  const [rows, setRows] = useState<SentRow[] | null>(null);
  const [err, setErr] = useState<string | null>(null);

  useEffect(() => {
    listSent(campaignId, 100).then(setRows).catch((e) => setErr(asMessage(e)));
  }, [campaignId]);

  return (
    <>
      <ErrorBanner message={err} />
      <div className="border border-border rounded-md overflow-hidden">
        <table className="w-full text-sm">
          <thead className="bg-panel text-muted text-left">
            <tr><th className="px-3 py-2">Sent</th><th className="px-3 py-2">Status</th><th className="px-3 py-2">Recipient</th><th className="px-3 py-2">Job</th><th className="px-3 py-2">Response</th></tr>
          </thead>
          <tbody>
            {rows?.length === 0 && <tr><td colSpan={5} className="px-3 py-3 text-muted">No sends yet.</td></tr>}
            {rows?.map((r) => (
              <tr key={r.id} className="border-t border-border align-top">
                <td className="px-3 py-2 text-muted whitespace-nowrap">{new Date(r.sent_at).toLocaleString()}</td>
                <td className="px-3 py-2"><Badge tone={r.status === "sent" ? "ok" : r.status === "bounced" ? "err" : "warn"}>{r.status}</Badge></td>
                <td className="px-3 py-2 font-mono text-xs">{r.recruiter_email}</td>
                <td className="px-3 py-2">{r.job_title}<div className="text-xs text-muted">{r.company_name}</div></td>
                <td className="px-3 py-2 font-mono text-xs whitespace-pre-wrap">{r.smtp_response}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </>
  );
}

function SettingsPane({ campaign, onChange }: { campaign: CampaignWithCounts; onChange: (c: Partial<CampaignWithCounts>) => void }) {
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  async function call(action: "pause" | "resume" | "archive") {
    setBusy(true);
    setErr(null);
    try {
      const c = await setCampaignStatus(campaign.id, action);
      onChange({ status: c.status });
    } catch (e) {
      setErr(asMessage(e));
    } finally {
      setBusy(false);
    }
  }

  return (
    <Card className="max-w-xl">
      <div className="space-y-3">
        <div className="text-sm text-muted">
          Resume: <span className="font-mono">{campaign.resume_id}</span>
        </div>
        <div className="text-sm text-muted">
          SMTP: <span className="font-mono">{campaign.smtp_id}</span>
        </div>
        <div className="flex gap-2 pt-2">
          {campaign.status !== "active" && <Button onClick={() => call("resume")} disabled={busy}>Resume</Button>}
          {campaign.status === "active" && <Button variant="ghost" onClick={() => call("pause")} disabled={busy}>Pause</Button>}
          {campaign.status !== "archived" && <Button variant="danger" onClick={() => call("archive")} disabled={busy}>Archive</Button>}
        </div>
        <ErrorBanner message={err} />
      </div>
    </Card>
  );
}

function stateTone(state: string): "default" | "ok" | "warn" | "err" {
  if (state === "sent") return "ok";
  if (state === "failed") return "err";
  if (state === "skipped" || state === "email_missing") return "warn";
  return "default";
}

function asMessage(e: unknown): string {
  if (e instanceof ApiException) return e.body.message;
  if (e instanceof Error) return e.message;
  return String(e);
}
