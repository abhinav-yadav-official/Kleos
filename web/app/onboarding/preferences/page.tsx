"use client";
import { useEffect, useState } from "react";
import { Nav } from "@/components/Nav";
import { Button, Card, ErrorBanner, Input, Label, Textarea } from "@/components/ui";
import { ApiException, getPreferences, Preferences, putPreferences } from "@/lib/api";

const TONE_PRESETS = ["warm", "direct", "technical", "casual", "formal"];
const LEVELS = ["any", "junior", "mid", "senior", "staff", "principal"];

export default function OnboardingPreferencesPage() {
  const [p, setP] = useState<Preferences | null>(null);
  const [err, setErr] = useState<string | null>(null);
  const [saved, setSaved] = useState(false);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    getPreferences().then(setP).catch((e) => setErr(asMessage(e)));
  }, []);

  if (!p) {
    return (
      <>
        <Nav />
        <p className="text-muted">Loading…</p>
        <ErrorBanner message={err} />
      </>
    );
  }

  async function save(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setErr(null);
    setSaved(false);
    try {
      const next = await putPreferences(p!);
      setP(next);
      setSaved(true);
    } catch (e) {
      setErr(asMessage(e));
    } finally {
      setBusy(false);
    }
  }

  return (
    <>
      <Nav />
      <h1 className="text-xl font-semibold mb-4">Onboarding · Preferences</h1>
      <p className="text-sm text-muted mb-6 max-w-2xl">
        Used by the scorer (§8) and the content prompt (§10). Keywords are case-insensitive.
        Comma-separated.
      </p>

      <form onSubmit={save} className="space-y-4 max-w-3xl">
        <Card>
          <Label>Job titles</Label>
          <Input value={p.job_titles.join(", ")} onChange={(e) => setP({ ...p, job_titles: splitList(e.target.value) })} placeholder="backend engineer, platform engineer" />
        </Card>
        <Card>
          <Label>Job functions</Label>
          <Input value={p.job_functions.join(", ")} onChange={(e) => setP({ ...p, job_functions: splitList(e.target.value) })} placeholder="engineering, platform, security" />
        </Card>
        <Card>
          <Label>Experience level</Label>
          <select
            value={p.experience_level}
            onChange={(e) => setP({ ...p, experience_level: e.target.value })}
            className="block w-full rounded-md border border-border bg-bg px-3 h-9 text-sm focus:outline-none focus:border-accent"
          >
            {LEVELS.map((l) => <option key={l} value={l}>{l}</option>)}
          </select>
        </Card>
        <Card>
          <Label>Locations</Label>
          <Input value={p.locations.join(", ")} onChange={(e) => setP({ ...p, locations: splitList(e.target.value) })} placeholder="remote, bangalore, sf bay" />
        </Card>
        <Card>
          <Label>Keywords include (must appear)</Label>
          <Input value={p.keywords_include.join(", ")} onChange={(e) => setP({ ...p, keywords_include: splitList(e.target.value) })} placeholder="go, postgres" />
        </Card>
        <Card>
          <Label>Keywords exclude (skip if present)</Label>
          <Input value={p.keywords_exclude.join(", ")} onChange={(e) => setP({ ...p, keywords_exclude: splitList(e.target.value) })} placeholder="php, wordpress" />
        </Card>
        <Card>
          <label className="flex items-center gap-2 text-sm">
            <input type="checkbox" checked={p.remote_only} onChange={(e) => setP({ ...p, remote_only: e.target.checked })} />
            Remote-only roles
          </label>
        </Card>
        <Card>
          <Label>Tone preset</Label>
          <select
            value={p.tone_preset}
            onChange={(e) => setP({ ...p, tone_preset: e.target.value })}
            className="block w-full rounded-md border border-border bg-bg px-3 h-9 text-sm focus:outline-none focus:border-accent"
          >
            {TONE_PRESETS.map((t) => <option key={t} value={t}>{t}</option>)}
          </select>
        </Card>
        <Card>
          <Label>Tone addendum (max 500 chars)</Label>
          <Textarea
            rows={4}
            maxLength={500}
            value={p.tone_addendum}
            onChange={(e) => setP({ ...p, tone_addendum: e.target.value })}
            placeholder="keep it concise; lead with one concrete project"
          />
          <div className="text-xs text-muted mt-1">{p.tone_addendum.length}/500</div>
        </Card>

        <ErrorBanner message={err} />
        {saved && <p className="text-ok text-sm">Saved.</p>}
        <Button type="submit" disabled={busy}>{busy ? "Saving…" : "Save preferences"}</Button>
      </form>
    </>
  );
}

function splitList(s: string): string[] {
  return s.split(",").map((x) => x.trim()).filter(Boolean);
}

function asMessage(e: unknown): string {
  if (e instanceof ApiException) return e.body.message;
  if (e instanceof Error) return e.message;
  return String(e);
}
