"use client";
import { useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { Nav } from "@/components/Nav";
import { Badge, Button, Card, ErrorBanner, Input } from "@/components/ui";
import { addRecipients, ApiException, getRecipientPool, PoolEntry } from "@/lib/api";

const COUNTRIES = [
  { code: "IN", label: "India" },
];

export default function RecipientPoolPage() {
  const [country, setCountry] = useState("IN");
  const [pool, setPool] = useState<PoolEntry[] | null>(null);
  const [picked, setPicked] = useState<Set<string>>(new Set());
  const [filter, setFilter] = useState("");
  const [err, setErr] = useState<string | null>(null);
  const [msg, setMsg] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    setPool(null);
    setPicked(new Set());
    getRecipientPool(country, 500).then(setPool).catch((e) => setErr(asMessage(e)));
  }, [country]);

  const visible = useMemo(() => {
    if (!pool) return [];
    const q = filter.trim().toLowerCase();
    if (!q) return pool;
    return pool.filter((p) =>
      p.email.toLowerCase().includes(q) ||
      p.company_name.toLowerCase().includes(q) ||
      p.company_slug.toLowerCase().includes(q) ||
      (p.name || "").toLowerCase().includes(q)
    );
  }, [pool, filter]);

  function toggle(id: string) {
    setPicked((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }

  function toggleAllVisible(check: boolean) {
    setPicked((prev) => {
      const next = new Set(prev);
      for (const p of visible) {
        if (check) next.add(p.recruiter_id);
        else next.delete(p.recruiter_id);
      }
      return next;
    });
  }

  async function addSelected() {
    if (!pool) return;
    setBusy(true);
    setErr(null);
    setMsg(null);
    try {
      // Group selected entries by company_slug so we can re-paste via the
      // existing /api/recipients endpoint (idempotent via ON CONFLICT).
      const bySlug = new Map<string, PoolEntry[]>();
      for (const p of pool) {
        if (!picked.has(p.recruiter_id)) continue;
        const list = bySlug.get(p.company_slug) ?? [];
        list.push(p);
        bySlug.set(p.company_slug, list);
      }
      let total = 0;
      for (const [slug, entries] of bySlug) {
        const first = entries[0];
        await addRecipients({
          company_slug: slug,
          domain: first.company_domain || undefined,
          emails: entries.map((e) => ({ email: e.email, name: e.name, title: e.title })),
        });
        total += entries.length;
      }
      setMsg(`Re-asserted ${total} recipients across ${bySlug.size} companies into your pool. Audit row written.`);
      setPicked(new Set());
    } catch (e) {
      setErr(asMessage(e));
    } finally {
      setBusy(false);
    }
  }

  const allVisibleChecked = visible.length > 0 && visible.every((p) => picked.has(p.recruiter_id));

  return (
    <>
      <Nav />
      <div className="flex items-center justify-between mb-4">
        <div>
          <h1 className="text-xl font-semibold">Recipient pool</h1>
          <p className="text-sm text-muted">
            Pre-scraped recruiter contacts at companies tagged by country. Pick the ones to add to
            your pool — campaign tick + email finder will use them when matching jobs appear.
          </p>
        </div>
        <Link href="/onboarding/recipients" className="text-accent text-sm">Manual paste →</Link>
      </div>

      <div className="flex flex-wrap items-end gap-3 mb-3">
        <label className="text-sm">
          <span className="text-xs uppercase tracking-wide text-muted block mb-1">Country</span>
          <select
            value={country}
            onChange={(e) => setCountry(e.target.value)}
            className="rounded-md border border-border bg-bg px-3 h-9 text-sm"
          >
            {COUNTRIES.map((c) => <option key={c.code} value={c.code}>{c.label}</option>)}
          </select>
        </label>
        <div className="flex-1 min-w-[200px]">
          <span className="text-xs uppercase tracking-wide text-muted block mb-1">Filter</span>
          <Input
            placeholder="email, name, or company"
            value={filter}
            onChange={(e) => setFilter(e.target.value)}
          />
        </div>
        <Button onClick={addSelected} disabled={busy || picked.size === 0}>
          {busy ? "Adding…" : `Add selected (${picked.size})`}
        </Button>
      </div>

      <ErrorBanner message={err} />
      {msg && <p className="text-ok text-sm mb-3">{msg}</p>}

      <Card className="p-0 overflow-hidden">
        <table className="w-full text-sm">
          <thead className="bg-panel text-muted text-left">
            <tr>
              <th className="px-3 py-2 w-8">
                <input
                  type="checkbox"
                  checked={allVisibleChecked}
                  onChange={(e) => toggleAllVisible(e.target.checked)}
                  aria-label="select all"
                />
              </th>
              <th className="px-3 py-2">Email</th>
              <th className="px-3 py-2">Name</th>
              <th className="px-3 py-2">Title</th>
              <th className="px-3 py-2">Company</th>
              <th className="px-3 py-2">Confidence</th>
              <th className="px-3 py-2">Source</th>
            </tr>
          </thead>
          <tbody>
            {pool === null && <tr><td colSpan={7} className="px-3 py-3 text-muted">Loading…</td></tr>}
            {pool?.length === 0 && (
              <tr>
                <td colSpan={7} className="px-3 py-4 text-muted">
                  Pool is empty for {country}. Run <span className="font-mono">worker-prefetchpool --seed seeds/india_companies.json</span> on the VPS.
                </td>
              </tr>
            )}
            {visible.map((p) => (
              <tr key={p.recruiter_id} className="border-t border-border align-top">
                <td className="px-3 py-2">
                  <input
                    type="checkbox"
                    checked={picked.has(p.recruiter_id)}
                    onChange={() => toggle(p.recruiter_id)}
                  />
                </td>
                <td className="px-3 py-2 font-mono text-xs">{p.email}</td>
                <td className="px-3 py-2">{p.name || "—"}</td>
                <td className="px-3 py-2 text-muted">{p.title || "—"}</td>
                <td className="px-3 py-2">{p.company_name}<div className="text-xs text-muted">{p.company_slug}</div></td>
                <td className="px-3 py-2"><Badge tone={confidenceTone(p.confidence)}>{p.confidence}</Badge></td>
                <td className="px-3 py-2 text-muted">{p.source}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </Card>
    </>
  );
}

function confidenceTone(c: string): "ok" | "warn" | "default" {
  if (c === "high") return "ok";
  if (c === "medium") return "warn";
  return "default";
}

function asMessage(e: unknown): string {
  if (e instanceof ApiException) return e.body.message;
  if (e instanceof Error) return e.message;
  return String(e);
}
