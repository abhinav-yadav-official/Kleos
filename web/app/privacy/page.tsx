import Link from "next/link";

export const metadata = {
  title: "Privacy — Kleos",
};

export default function PrivacyPage() {
  return (
    <div className="prose prose-invert max-w-3xl mx-auto">
      <p>
        <Link href="/" className="text-accent text-sm">← Back</Link>
      </p>
      <h1 className="text-2xl font-semibold mt-4">Privacy</h1>
      <p className="text-sm text-muted">Last updated 2026-05-26.</p>

      <h2 className="text-lg font-semibold mt-6">What Kleos stores</h2>
      <ul className="list-disc ml-5 text-sm space-y-1">
        <li>Account: email, bcrypt password hash, name, created/updated timestamps, TOS-accepted timestamp.</li>
        <li>SMTP credentials: host, port, username, AES-256-GCM-encrypted password. Plaintext password never persists.</li>
        <li>Resume: PDF file on disk + extracted text. Used only as prompt context for your campaigns.</li>
        <li>Preferences: roles, locations, keywords, tone, optional addendum.</li>
        <li>Campaigns and derived rows: matches, draft variants, sent-email metadata (message-id, smtp response).</li>
        <li>Warm-up state: per-day counters and pause status.</li>
        <li>Audit log: signup, login, smtp add/delete, send events, admin actions. Retained for security review.</li>
      </ul>

      <h2 className="text-lg font-semibold mt-6">What Kleos does not do</h2>
      <ul className="list-disc ml-5 text-sm space-y-1">
        <li>No open/click tracking pixels. Emails are plain text.</li>
        <li>No third-party analytics in the web UI.</li>
        <li>No selling, renting, or sharing of user data with advertisers.</li>
        <li>No reading of recruiter inboxes or replies. Kleos only sends; replies land in your own inbox.</li>
      </ul>

      <h2 className="text-lg font-semibold mt-6">Recruiter contact data</h2>
      <p className="text-sm">
        Recruiter emails come from three sources: public mailto links on careers pages,
        commit metadata from public GitHub repositories the company owns, and operator paste.
        Role-alias addresses (security@, abuse@, …) are filtered. By signing up you attested
        you have a lawful basis to contact each recruiter you send mail to.
      </p>

      <h2 className="text-lg font-semibold mt-6">Deletion</h2>
      <p className="text-sm">
        Use Settings → Danger zone → Delete account. Kleos removes your user row and cascades
        to refresh tokens, preferences, SMTP credentials, resumes, campaigns, matches, drafts,
        and sent-email history. Audit-log rows are retained with the user_id cleared. The
        operation is immediate and irreversible.
      </p>

      <h2 className="text-lg font-semibold mt-6">Subprocessors</h2>
      <ul className="list-disc ml-5 text-sm space-y-1">
        <li>OpenAI / Codex CLI — prompt and resume excerpt used for generation; bound by their data policy.</li>
        <li>Your SMTP provider (Gmail, ForwardEmail, etc.) — receives your sends.</li>
      </ul>

      <h2 className="text-lg font-semibold mt-6">Contact</h2>
      <p className="text-sm">
        Open an issue at <a href="https://github.com/abhinav-yadav-official/Kleos/issues" className="text-accent">github.com/abhinav-yadav-official/Kleos</a>.
      </p>
    </div>
  );
}
