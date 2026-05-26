"use client";
import { useEffect, useState } from "react";
import Link from "next/link";
import { Nav } from "@/components/Nav";
import { Badge, Card } from "@/components/ui";
import { Campaign, CampaignWithCounts, getWarmup, listCampaigns, listResumes, listSmtp, Resume, Smtp, Warmup } from "@/lib/api";

export default function DashboardPage() {
  const [campaigns, setCampaigns] = useState<CampaignWithCounts[] | null>(null);
  const [smtp, setSmtp] = useState<Smtp[] | null>(null);
  const [resumes, setResumes] = useState<Resume[] | null>(null);
  const [warmup, setWarmup] = useState<Warmup | null>(null);
  const [err, setErr] = useState<string | null>(null);

  useEffect(() => {
    Promise.all([listCampaigns(), listSmtp(), listResumes(), getWarmup().catch(() => null)])
      .then(([cs, ss, rs, w]) => {
        setCampaigns(cs);
        setSmtp(ss);
        setResumes(rs);
        setWarmup(w);
      })
      .catch((e) => setErr(e.message));
  }, []);

  const activeCampaigns = campaigns?.filter((c) => c.status === "active") ?? [];
  const sentByState = campaigns?.reduce((acc, c) => acc + (c.matches_by_state?.sent ?? 0), 0) ?? 0;
  const verifiedSmtp = smtp?.filter((s) => s.verified_at) ?? [];
  const activeResume = resumes?.find((r) => r.is_active);

  return (
    <>
      <Nav />
      {err && <div className="text-err mb-4">{err}</div>}

      <OnboardingHints smtp={smtp} resumes={resumes} />

      <div className="grid gap-4 md:grid-cols-4">
        <Stat label="Active campaigns" value={activeCampaigns.length} />
        <Stat label="Emails sent" value={sentByState} />
        <Stat label="Verified SMTP" value={verifiedSmtp.length} />
        <Stat
          label="Warm-up"
          value={
            warmup
              ? `${warmup.todays_sent}/${warmup.todays_limit} day ${warmup.current_day}`
              : "not started"
          }
          tone={warmup?.paused ? "warn" : warmup ? "ok" : "default"}
        />
      </div>

      <h2 className="mt-8 mb-3 text-sm uppercase tracking-wide text-muted">Campaigns</h2>
      <div className="space-y-2">
        {campaigns?.length === 0 && <p className="text-muted text-sm">No campaigns yet. Finish onboarding then create one.</p>}
        {campaigns?.map((c) => <CampaignCard key={c.id} c={c} />)}
      </div>

      {activeResume && (
        <p className="mt-6 text-xs text-muted">Active resume: {activeResume.filename}</p>
      )}
    </>
  );
}

function Stat({ label, value, tone = "default" }: { label: string; value: number | string; tone?: "default" | "ok" | "warn" }) {
  const toneClass = tone === "warn" ? "text-warn" : tone === "ok" ? "text-ok" : "text-text";
  return (
    <Card>
      <div className="text-xs uppercase tracking-wide text-muted">{label}</div>
      <div className={`mt-1 text-2xl font-semibold ${toneClass}`}>{value}</div>
    </Card>
  );
}

function OnboardingHints({ smtp, resumes }: { smtp: Smtp[] | null; resumes: Resume[] | null }) {
  const needsSmtp = smtp !== null && !smtp.some((s) => s.verified_at);
  const needsResume = resumes !== null && !resumes.some((r) => r.is_active);
  if (!needsSmtp && !needsResume) return null;
  return (
    <div className="mb-6 grid gap-3 md:grid-cols-2">
      {needsSmtp && (
        <Card className="border-warn/50">
          <div className="font-medium">Connect SMTP</div>
          <p className="text-sm text-muted mt-1">Add and verify an SMTP credential to send mail.</p>
          <Link href="/onboarding/smtp" className="text-accent text-sm mt-2 inline-block">Go to SMTP →</Link>
        </Card>
      )}
      {needsResume && (
        <Card className="border-warn/50">
          <div className="font-medium">Upload resume</div>
          <p className="text-sm text-muted mt-1">Upload a PDF and activate it so generation has context.</p>
          <Link href="/onboarding/resume" className="text-accent text-sm mt-2 inline-block">Go to resume →</Link>
        </Card>
      )}
    </div>
  );
}

function CampaignCard({ c }: { c: CampaignWithCounts }) {
  const counts = c.matches_by_state || {};
  return (
    <Link href={{ pathname: "/campaigns/view", query: { id: c.id } }} className="block">
      <Card className="hover:border-accent/60">
        <div className="flex justify-between items-center">
          <div>
            <div className="font-medium">{c.name}</div>
            <div className="text-xs text-muted">
              new {counts.new ?? 0} · gen {counts.generated ?? 0} · sent {counts.sent ?? 0} · failed {counts.failed ?? 0}
            </div>
          </div>
          <Badge tone={c.status === "active" ? "ok" : c.status === "paused" ? "warn" : "default"}>{c.status}</Badge>
        </div>
      </Card>
    </Link>
  );
}
