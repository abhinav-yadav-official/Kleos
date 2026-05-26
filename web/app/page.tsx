"use client";
import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { Button, Card, ErrorBanner, Input, Label } from "@/components/ui";
import { ApiException, getAccess, login, signup } from "@/lib/api";

type Mode = "login" | "signup";

export default function LandingPage() {
  const router = useRouter();
  const [mode, setMode] = useState<Mode>("login");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [name, setName] = useState("");
  const [err, setErr] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    if (getAccess()) router.replace("/dashboard");
  }, [router]);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setErr(null);
    setBusy(true);
    try {
      if (mode === "login") await login(email, password);
      else await signup(email, password, name || email.split("@")[0]);
      router.push("/dashboard");
    } catch (e) {
      setErr(e instanceof ApiException ? e.body.message : (e as Error).message);
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="grid gap-8 md:grid-cols-2 mt-12">
      <section>
        <h1 className="text-4xl font-semibold leading-tight">
          Kleos
        </h1>
        <p className="text-muted mt-3 max-w-md">
          Automated recruiter outreach. Scrape jobs, find the right contact,
          draft a tailored email per role, send with warm-up — without losing
          your inbox.
        </p>
        <ul className="mt-6 space-y-2 text-sm text-muted">
          <li>• You bring SMTP + resume + preferences.</li>
          <li>• Kleos matches jobs, drafts emails per match, and sends one at a time.</li>
          <li>• 3-variant generation with spam self-check picks the safest send.</li>
        </ul>
      </section>

      <Card>
        <div className="flex gap-2 mb-4">
          <Button variant={mode === "login" ? "primary" : "ghost"} onClick={() => setMode("login")}>
            Log in
          </Button>
          <Button variant={mode === "signup" ? "primary" : "ghost"} onClick={() => setMode("signup")}>
            Sign up
          </Button>
        </div>
        <form onSubmit={submit} className="space-y-3">
          {mode === "signup" && (
            <div>
              <Label htmlFor="name">Name</Label>
              <Input id="name" value={name} onChange={(e) => setName(e.target.value)} autoComplete="name" />
            </div>
          )}
          <div>
            <Label htmlFor="email">Email</Label>
            <Input id="email" type="email" value={email} onChange={(e) => setEmail(e.target.value)} required autoComplete="email" />
          </div>
          <div>
            <Label htmlFor="password">Password</Label>
            <Input id="password" type="password" value={password} onChange={(e) => setPassword(e.target.value)} required autoComplete={mode === "login" ? "current-password" : "new-password"} />
          </div>
          <ErrorBanner message={err} />
          <Button type="submit" disabled={busy} className="w-full">
            {busy ? "Working…" : mode === "login" ? "Log in" : "Create account"}
          </Button>
        </form>
      </Card>
    </div>
  );
}
