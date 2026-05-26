"use client";
import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { Button, Card, ErrorBanner, Input, Label } from "@/components/ui";
import { ApiException, getAccess, login, setTokens, signup } from "@/lib/api";

type Mode = "login" | "signup";

export default function LandingPage() {
  const router = useRouter();
  const [mode, setMode] = useState<Mode>("login");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [name, setName] = useState("");
  const [tosAccepted, setTosAccepted] = useState(false);
  const [err, setErr] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    // Google OAuth callback delivers tokens via URL fragment. Consume them,
    // persist, and replace the URL before the rest of the page renders.
    if (typeof window !== "undefined" && window.location.hash) {
      const params = new URLSearchParams(window.location.hash.slice(1));
      const access = params.get("access");
      const refresh = params.get("refresh");
      if (access && refresh) {
        setTokens({ access, refresh });
        window.history.replaceState(null, "", window.location.pathname);
        router.replace("/dashboard");
        return;
      }
    }
    if (typeof window !== "undefined") {
      const qs = new URLSearchParams(window.location.search);
      const ge = qs.get("google_error");
      if (ge) setErr(`Google sign-in failed: ${ge}`);
    }
    if (getAccess()) router.replace("/dashboard");
  }, [router]);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setErr(null);
    setBusy(true);
    try {
      if (mode === "login") await login(email, password);
      else await signup(email, password, name || email.split("@")[0], tosAccepted);
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
          {mode === "signup" && (
            <label className="flex items-start gap-2 text-xs text-muted">
              <input
                type="checkbox"
                checked={tosAccepted}
                onChange={(e) => setTosAccepted(e.target.checked)}
                className="mt-0.5"
              />
              <span>
                I confirm I have a lawful basis to contact each recruiter I send mail to.
                See <a href="/kleos/privacy/" className="text-accent underline">privacy</a>.
              </span>
            </label>
          )}
          <ErrorBanner message={err} />
          <Button type="submit" disabled={busy || (mode === "signup" && !tosAccepted)} className="w-full">
            {busy ? "Working…" : mode === "login" ? "Log in" : "Create account"}
          </Button>
        </form>

        <div className="my-4 flex items-center gap-3 text-xs text-muted">
          <span className="h-px flex-1 bg-border" />
          <span>or</span>
          <span className="h-px flex-1 bg-border" />
        </div>
        <a
          href="/kleos/api/auth/google/start"
          className="flex items-center justify-center gap-2 rounded-md border border-border bg-bg hover:bg-border/40 px-4 h-9 text-sm font-medium"
        >
          <svg width="18" height="18" viewBox="0 0 18 18" aria-hidden>
            <path fill="#4285F4" d="M17.64 9.2c0-.64-.06-1.25-.16-1.84H9v3.49h4.84a4.14 4.14 0 0 1-1.8 2.72v2.26h2.92c1.71-1.57 2.68-3.88 2.68-6.63z" />
            <path fill="#34A853" d="M9 18c2.43 0 4.47-.81 5.96-2.18l-2.92-2.26c-.81.54-1.85.86-3.04.86-2.34 0-4.32-1.58-5.03-3.7H.96v2.32A9 9 0 0 0 9 18z" />
            <path fill="#FBBC05" d="M3.97 10.72A5.41 5.41 0 0 1 3.68 9c0-.6.1-1.18.29-1.72V4.96H.96A9 9 0 0 0 0 9c0 1.45.35 2.83.96 4.04l3.01-2.32z" />
            <path fill="#EA4335" d="M9 3.58c1.32 0 2.5.45 3.44 1.35l2.58-2.58C13.46.89 11.43 0 9 0A9 9 0 0 0 .96 4.96l3.01 2.32C4.68 5.16 6.66 3.58 9 3.58z" />
          </svg>
          Continue with Google
        </a>
        <p className="text-xs text-muted mt-2">
          Clicking continues with Google and accepts our{" "}
          <a href="/kleos/privacy/" className="text-accent underline">privacy</a> terms.
        </p>
      </Card>
    </div>
  );
}
