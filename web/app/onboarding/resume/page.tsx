"use client";
import { useEffect, useRef, useState } from "react";
import { Nav } from "@/components/Nav";
import { Badge, Button, Card, ErrorBanner } from "@/components/ui";
import { ApiException, activateResume, deleteResume, listResumes, Resume, uploadResume } from "@/lib/api";

export default function OnboardingResumePage() {
  const [items, setItems] = useState<Resume[]>([]);
  const [err, setErr] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const fileRef = useRef<HTMLInputElement>(null);

  async function reload() {
    try {
      setItems(await listResumes());
    } catch (e) {
      setErr(asMessage(e));
    }
  }
  useEffect(() => {
    reload();
  }, []);

  async function upload(e: React.ChangeEvent<HTMLInputElement>) {
    const f = e.target.files?.[0];
    if (!f) return;
    setBusy(true);
    setErr(null);
    try {
      await uploadResume(f);
      if (fileRef.current) fileRef.current.value = "";
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
      <h1 className="text-xl font-semibold mb-4">Onboarding · Resume</h1>
      <p className="text-sm text-muted mb-6 max-w-2xl">
        Upload a single-column PDF (≤10MB). Kleos runs <span className="font-mono">pdftotext</span> server-side
        and uses the extracted text in prompt context. You can keep multiple versions; one is active at a time.
      </p>

      <Card className="mb-6">
        <input ref={fileRef} type="file" accept="application/pdf" onChange={upload} disabled={busy} className="text-sm" />
        {busy && <p className="text-muted text-sm mt-2">Uploading…</p>}
        <ErrorBanner message={err} />
      </Card>

      <div className="space-y-2">
        {items.length === 0 && <p className="text-muted text-sm">No resumes uploaded yet.</p>}
        {items.map((r) => (
          <Card key={r.id}>
            <div className="flex justify-between items-start gap-3">
              <div className="min-w-0">
                <div className="font-medium truncate">{r.filename}</div>
                <div className="text-xs text-muted">{new Date(r.created_at).toLocaleString()}</div>
                {r.parsed_text_preview && (
                  <pre className="mt-2 max-h-40 overflow-auto text-xs text-muted whitespace-pre-wrap font-mono">
                    {r.parsed_text_preview}
                  </pre>
                )}
              </div>
              <div className="flex flex-col gap-2 shrink-0">
                {r.is_active ? (
                  <Badge tone="ok">active</Badge>
                ) : (
                  <Button variant="ghost" onClick={async () => { await activateResume(r.id); reload(); }}>
                    Make active
                  </Button>
                )}
                <Button variant="danger" onClick={async () => { if (confirm("Delete?")) { await deleteResume(r.id); reload(); } }}>
                  Delete
                </Button>
              </div>
            </div>
          </Card>
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
