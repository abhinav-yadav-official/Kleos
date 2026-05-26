"use client";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { Nav } from "@/components/Nav";
import { Badge, Button, Card, ErrorBanner } from "@/components/ui";
import { ApiException, deleteAccount, getWarmup, listResumes, listSmtp, Resume, Smtp, Warmup } from "@/lib/api";

export default function SettingsPage() {
  const router = useRouter();
  const [smtp, setSmtp] = useState<Smtp[]>([]);
  const [resumes, setResumes] = useState<Resume[]>([]);
  const [warmup, setWarmup] = useState<Warmup | null>(null);
  const [err, setErr] = useState<string | null>(null);
  const [deleting, setDeleting] = useState(false);

  useEffect(() => {
    Promise.all([listSmtp(), listResumes(), getWarmup().catch(() => null)])
      .then(([s, r, w]) => {
        setSmtp(s);
        setResumes(r);
        setWarmup(w);
      })
      .catch((e) => setErr(asMessage(e)));
  }, []);

  return (
    <>
      <Nav />
      <h1 className="text-xl font-semibold mb-4">Settings</h1>
      <ErrorBanner message={err} />

      <section className="mb-8">
        <h2 className="text-sm uppercase tracking-wide text-muted mb-2">Warm-up</h2>
        <Card>
          {warmup ? (
            <div className="text-sm space-y-1">
              <div>Day <span className="font-mono">{warmup.current_day}</span> · sent today <span className="font-mono">{warmup.todays_sent}/{warmup.todays_limit}</span></div>
              <div className="text-muted">Started {new Date(warmup.start_date).toLocaleDateString()} · last rollover {new Date(warmup.last_rollover).toLocaleDateString()}</div>
              <div>{warmup.paused ? <Badge tone="warn">paused — {warmup.notes || "no notes"}</Badge> : <Badge tone="ok">active</Badge>}</div>
            </div>
          ) : (
            <p className="text-sm text-muted">Verify an SMTP credential to start warm-up.</p>
          )}
        </Card>
      </section>

      <section className="mb-8">
        <div className="flex justify-between mb-2">
          <h2 className="text-sm uppercase tracking-wide text-muted">SMTP credentials</h2>
          <Link href="/onboarding/smtp" className="text-accent text-sm">Manage →</Link>
        </div>
        <div className="space-y-2">
          {smtp.map((s) => (
            <Card key={s.id}>
              <div className="flex justify-between items-center">
                <div>
                  <div className="font-medium">{s.label} <span className="text-muted text-xs">· {s.host}:{s.port}</span></div>
                  <div className="text-xs text-muted">{s.username}</div>
                </div>
                <div className="flex gap-2">
                  <Badge tone={s.verified_at ? "ok" : "warn"}>{s.verified_at ? "verified" : "unverified"}</Badge>
                  {s.is_primary && <Badge tone="ok">primary</Badge>}
                </div>
              </div>
              {s.last_error && <pre className="mt-2 text-xs text-err whitespace-pre-wrap">{s.last_error}</pre>}
            </Card>
          ))}
          {smtp.length === 0 && <p className="text-sm text-muted">No SMTP credentials yet.</p>}
        </div>
      </section>

      <section className="mb-8">
        <div className="flex justify-between mb-2">
          <h2 className="text-sm uppercase tracking-wide text-muted">Resumes</h2>
          <Link href="/onboarding/resume" className="text-accent text-sm">Manage →</Link>
        </div>
        <div className="space-y-2">
          {resumes.map((r) => (
            <Card key={r.id}>
              <div className="flex justify-between items-center">
                <div className="min-w-0">
                  <div className="font-medium truncate">{r.filename}</div>
                  <div className="text-xs text-muted">{new Date(r.created_at).toLocaleString()}</div>
                </div>
                {r.is_active && <Badge tone="ok">active</Badge>}
              </div>
            </Card>
          ))}
          {resumes.length === 0 && <p className="text-sm text-muted">No resumes uploaded.</p>}
        </div>
      </section>

      <section className="mb-8">
        <h2 className="text-sm uppercase tracking-wide text-muted mb-2">Preferences</h2>
        <Card>
          <Link href="/onboarding/preferences" className="text-accent text-sm">Edit preferences →</Link>
        </Card>
      </section>

      <section className="mb-8">
        <h2 className="text-sm uppercase tracking-wide text-err mb-2">Danger zone</h2>
        <Card className="border-err/50">
          <p className="text-sm text-muted mb-3">
            Delete account permanently removes your user record, refresh tokens, preferences,
            SMTP credentials, resumes, campaigns, matches, drafts, and sent-email history.
            Audit-log rows are retained with user_id cleared. Action cannot be undone.
          </p>
          <Button
            variant="danger"
            disabled={deleting}
            onClick={async () => {
              if (!confirm("Permanently delete your account and all data? This cannot be undone.")) return;
              setDeleting(true);
              try {
                await deleteAccount();
                router.replace("/");
              } catch (e) {
                setErr(asMessage(e));
              } finally {
                setDeleting(false);
              }
            }}
          >
            {deleting ? "Deleting…" : "Delete account"}
          </Button>
        </Card>
      </section>
    </>
  );
}

function asMessage(e: unknown): string {
  if (e instanceof ApiException) return e.body.message;
  if (e instanceof Error) return e.message;
  return String(e);
}
