"use client";
import { useState } from "react";
import { Nav } from "@/components/Nav";
import { Button, Card, ErrorBanner, Input, Label, Textarea } from "@/components/ui";
import { addRecipients, ApiException, RecipientEntry } from "@/lib/api";

export default function OnboardingRecipientsPage() {
  const [slug, setSlug] = useState("");
  const [domain, setDomain] = useState("");
  const [careers, setCareers] = useState("");
  const [github, setGithub] = useState("");
  const [csv, setCSV] = useState("");
  const [err, setErr] = useState<string | null>(null);
  const [msg, setMsg] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setErr(null);
    setMsg(null);
    setBusy(true);
    try {
      const emails = parseCSV(csv);
      if (emails.length === 0) throw new Error("Paste at least one recipient");
      const out = await addRecipients({
        company_slug: slug,
        domain: domain || undefined,
        careers_url: careers || undefined,
        github_org: github || undefined,
        emails,
      });
      setMsg(`Inserted ${out.inserted}/${out.submitted} recipients into ${slug}.`);
      setCSV("");
    } catch (e) {
      setErr(asMessage(e));
    } finally {
      setBusy(false);
    }
  }

  return (
    <>
      <Nav />
      <h1 className="text-xl font-semibold mb-4">Onboarding · Recipients</h1>
      <p className="text-sm text-muted mb-6 max-w-2xl">
        Paste your own list of recruiter contacts. Rows land in the shared recruiter pool, so the
        email finder picks them up on the next campaign tick. One row per line:
        <span className="font-mono"> email[, name[, title]]</span>. Role-alias addresses
        (security@, abuse@) and denylisted addresses are filtered server-side.
      </p>

      <form onSubmit={submit} className="space-y-4 max-w-2xl">
        <Card>
          <Label>Company slug *</Label>
          <Input value={slug} onChange={(e) => setSlug(e.target.value)} placeholder="acme" required />
          <p className="text-xs text-muted mt-1">
            Used to link recipients to a company. Pick something short + URL-safe; campaigns will
            match jobs whose <span className="font-mono">company.slug</span> equals this.
          </p>
        </Card>
        <Card>
          <div className="grid gap-3 md:grid-cols-3">
            <div>
              <Label>Domain</Label>
              <Input value={domain} onChange={(e) => setDomain(e.target.value)} placeholder="acme.com" />
            </div>
            <div>
              <Label>Careers URL</Label>
              <Input value={careers} onChange={(e) => setCareers(e.target.value)} placeholder="https://acme.com/careers" />
            </div>
            <div>
              <Label>GitHub org</Label>
              <Input value={github} onChange={(e) => setGithub(e.target.value)} placeholder="acme-corp" />
            </div>
          </div>
        </Card>
        <Card>
          <Label>Recipients (one per line)</Label>
          <Textarea
            rows={8}
            value={csv}
            onChange={(e) => setCSV(e.target.value)}
            placeholder={"jane@acme.com, Jane Doe, Recruiter\nhiring@acme.com\nbob@acme.com, Bob"}
          />
        </Card>
        <ErrorBanner message={err} />
        {msg && <p className="text-ok text-sm">{msg}</p>}
        <Button type="submit" disabled={busy}>{busy ? "Submitting…" : "Add recipients"}</Button>
      </form>
    </>
  );
}

function parseCSV(text: string): RecipientEntry[] {
  const out: RecipientEntry[] = [];
  for (const raw of text.split(/\n+/)) {
    const line = raw.trim();
    if (!line || line.startsWith("#")) continue;
    const parts = line.split(/\s*,\s*/);
    const [email, name, title] = parts;
    if (!email || !/.+@.+\..+/.test(email)) continue;
    out.push({ email, name, title });
  }
  return out;
}

function asMessage(e: unknown): string {
  if (e instanceof ApiException) return e.body.message;
  if (e instanceof Error) return e.message;
  return String(e);
}
