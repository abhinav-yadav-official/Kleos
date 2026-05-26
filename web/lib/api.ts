const BASE = "/kleos/api";

type Tokens = { access: string; refresh: string };

const STORAGE_ACCESS = "kleos.access";
const STORAGE_REFRESH = "kleos.refresh";

let accessMem: string | null = null;

export function getAccess(): string | null {
  if (accessMem) return accessMem;
  if (typeof window === "undefined") return null;
  return window.localStorage.getItem(STORAGE_ACCESS);
}
export function getRefresh(): string | null {
  if (typeof window === "undefined") return null;
  return window.localStorage.getItem(STORAGE_REFRESH);
}
export function setTokens(t: Tokens) {
  accessMem = t.access;
  if (typeof window !== "undefined") {
    window.localStorage.setItem(STORAGE_ACCESS, t.access);
    window.localStorage.setItem(STORAGE_REFRESH, t.refresh);
  }
}
export function clearTokens() {
  accessMem = null;
  if (typeof window !== "undefined") {
    window.localStorage.removeItem(STORAGE_ACCESS);
    window.localStorage.removeItem(STORAGE_REFRESH);
  }
}

export type ApiError = { code: string; message: string; details?: unknown };

export class ApiException extends Error {
  status: number;
  body: ApiError;
  constructor(status: number, body: ApiError) {
    super(body?.message || `HTTP ${status}`);
    this.status = status;
    this.body = body;
  }
}

async function rawFetch(path: string, init: RequestInit, auth: boolean): Promise<Response> {
  const headers = new Headers(init.headers);
  if (!headers.has("Content-Type") && init.body && !(init.body instanceof FormData)) {
    headers.set("Content-Type", "application/json");
  }
  if (auth) {
    const tok = getAccess();
    if (tok) headers.set("Authorization", `Bearer ${tok}`);
  }
  return fetch(`${BASE}${path}`, { ...init, headers });
}

async function refreshOnce(): Promise<boolean> {
  const refresh = getRefresh();
  if (!refresh) return false;
  const res = await rawFetch("/auth/refresh", {
    method: "POST",
    body: JSON.stringify({ refresh }),
  }, false);
  if (!res.ok) {
    clearTokens();
    return false;
  }
  const body = (await res.json()) as Tokens;
  setTokens(body);
  return true;
}

export async function apiFetch<T = unknown>(
  path: string,
  init: RequestInit = {},
  opts: { auth?: boolean; rawText?: boolean } = {},
): Promise<T> {
  const auth = opts.auth !== false;
  let res = await rawFetch(path, init, auth);
  if (res.status === 401 && auth) {
    const ok = await refreshOnce();
    if (ok) res = await rawFetch(path, init, true);
  }
  if (!res.ok) {
    let err: ApiError = { code: "unknown", message: res.statusText };
    try {
      const body = await res.json();
      if (body?.error) err = body.error;
    } catch {}
    throw new ApiException(res.status, err);
  }
  if (opts.rawText) return (await res.text()) as unknown as T;
  if (res.status === 204) return undefined as unknown as T;
  return (await res.json()) as T;
}

export type User = { id: string; email: string; name: string; is_admin?: boolean };
export type AuthResponse = { user: User; access: string; refresh: string };

export async function signup(email: string, password: string, name: string) {
  const out = await apiFetch<AuthResponse>("/auth/signup", {
    method: "POST",
    body: JSON.stringify({ email, password, name }),
  }, { auth: false });
  setTokens({ access: out.access, refresh: out.refresh });
  return out.user;
}

export async function login(email: string, password: string) {
  const out = await apiFetch<AuthResponse>("/auth/login", {
    method: "POST",
    body: JSON.stringify({ email, password }),
  }, { auth: false });
  setTokens({ access: out.access, refresh: out.refresh });
  return out.user;
}

export async function logout() {
  try {
    const refresh = getRefresh();
    if (refresh) {
      await apiFetch("/auth/logout", { method: "POST", body: JSON.stringify({ refresh }) }, { auth: true });
    }
  } finally {
    clearTokens();
  }
}

export async function me(): Promise<User> {
  const body = await apiFetch<{ user: User }>("/auth/me");
  return body.user;
}

export type Smtp = {
  id: string;
  label: string;
  host: string;
  port: number;
  username: string;
  from_email: string;
  from_name?: string;
  use_tls: boolean;
  is_primary: boolean;
  verified_at: string | null;
  last_error: string | null;
  created_at: string;
};
export async function listSmtp(): Promise<Smtp[]> {
  const body = await apiFetch<{ smtp: Smtp[] }>("/smtp");
  return body.smtp || [];
}
export async function createSmtp(input: {
  label: string;
  host: string;
  port: number;
  username: string;
  password: string;
  from_email: string;
  from_name?: string;
  use_tls: boolean;
}): Promise<Smtp> {
  const body = await apiFetch<{ smtp: Smtp }>("/smtp", { method: "POST", body: JSON.stringify(input) });
  return body.smtp;
}
export async function verifySmtp(id: string): Promise<{ ok: boolean; detail: string }> {
  return apiFetch(`/smtp/${id}/verify`, { method: "POST" });
}
export async function setPrimarySmtp(id: string): Promise<Smtp> {
  const body = await apiFetch<{ smtp: Smtp }>(`/smtp/${id}/primary`, { method: "POST" });
  return body.smtp;
}
export async function deleteSmtp(id: string): Promise<void> {
  await apiFetch(`/smtp/${id}`, { method: "DELETE" });
}

export type Resume = {
  id: string;
  user_id: string;
  filename: string;
  parsed_text_preview: string;
  is_active: boolean;
  created_at: string;
};
export async function listResumes(): Promise<Resume[]> {
  const body = await apiFetch<{ resumes: Resume[] }>("/resumes");
  return body.resumes || [];
}
export async function uploadResume(file: File): Promise<Resume> {
  const fd = new FormData();
  fd.append("file", file);
  const body = await apiFetch<{ resume: Resume }>("/resumes", { method: "POST", body: fd });
  return body.resume;
}
export async function activateResume(id: string): Promise<void> {
  await apiFetch(`/resumes/${id}/activate`, { method: "POST" });
}
export async function deleteResume(id: string): Promise<void> {
  await apiFetch(`/resumes/${id}`, { method: "DELETE" });
}

export type Preferences = {
  user_id?: string;
  job_titles: string[];
  job_functions: string[];
  experience_level: string;
  locations: string[];
  keywords_include: string[];
  keywords_exclude: string[];
  remote_only: boolean;
  tone_preset: string;
  tone_addendum: string;
  updated_at?: string;
};
export async function getPreferences(): Promise<Preferences> {
  const body = await apiFetch<{ preferences: Preferences }>("/preferences");
  return body.preferences;
}
export async function putPreferences(p: Preferences): Promise<Preferences> {
  const body = await apiFetch<{ preferences: Preferences }>("/preferences", {
    method: "PUT",
    body: JSON.stringify(p),
  });
  return body.preferences;
}

export type Campaign = {
  id: string;
  user_id: string;
  name: string;
  status: "active" | "paused" | "archived";
  resume_id: string;
  smtp_id: string;
  created_at: string;
  updated_at: string;
};
export type CampaignWithCounts = Campaign & { matches_by_state: Record<string, number> };
export async function listCampaigns(): Promise<CampaignWithCounts[]> {
  const body = await apiFetch<{ campaigns: CampaignWithCounts[] }>("/campaigns");
  return body.campaigns || [];
}
export async function getCampaign(id: string): Promise<CampaignWithCounts> {
  const body = await apiFetch<{ campaign: CampaignWithCounts }>(`/campaigns/${id}`);
  return body.campaign;
}
export async function createCampaign(input: { name: string; resume_id: string; smtp_id: string }): Promise<Campaign> {
  const body = await apiFetch<{ campaign: Campaign }>("/campaigns", { method: "POST", body: JSON.stringify(input) });
  return body.campaign;
}
export async function setCampaignStatus(id: string, action: "pause" | "resume" | "archive"): Promise<Campaign> {
  const body = await apiFetch<{ campaign: Campaign }>(`/campaigns/${id}/${action}`, { method: "POST" });
  return body.campaign;
}

export type MatchRow = {
  id: string;
  campaign_id: string;
  job_id: string;
  match_score: number;
  state: string;
  matched_at: string;
  job_title: string;
  job_url: string;
  job_location: string;
  job_remote: boolean;
  job_source: string;
  company_name: string;
};
export async function listMatches(campaignId: string, state?: string, limit = 50, offset = 0): Promise<MatchRow[]> {
  const qs = new URLSearchParams();
  if (state) qs.set("state", state);
  qs.set("limit", String(limit));
  qs.set("offset", String(offset));
  const body = await apiFetch<{ matches: MatchRow[] }>(`/campaigns/${campaignId}/matches?${qs.toString()}`);
  return body.matches || [];
}

export type DraftRow = {
  id: string;
  match_id: string;
  chosen: boolean;
  spam_score: number;
  subject: string;
  body_text: string;
  generated_at: string;
  job_title: string;
  company_name: string;
  recruiter_email: string;
};
export async function listDrafts(campaignId: string, limit = 60, offset = 0): Promise<DraftRow[]> {
  const body = await apiFetch<{ drafts: DraftRow[] }>(`/campaigns/${campaignId}/drafts?limit=${limit}&offset=${offset}`);
  return body.drafts || [];
}

export type SentRow = {
  id: string;
  match_id: string;
  recruiter_email: string;
  message_id: string;
  status: string;
  smtp_response: string;
  sent_at: string;
  job_title: string;
  company_name: string;
};
export async function listSent(campaignId: string, limit = 50, offset = 0): Promise<SentRow[]> {
  const body = await apiFetch<{ sent: SentRow[] }>(`/campaigns/${campaignId}/sent?limit=${limit}&offset=${offset}`);
  return body.sent || [];
}

export type Warmup = {
  user_id: string;
  smtp_id: string;
  start_date: string;
  current_day: number;
  todays_sent: number;
  todays_limit: number;
  last_rollover: string;
  paused: boolean;
  notes: string;
};
export async function getWarmup(): Promise<Warmup | null> {
  const body = await apiFetch<{ warmup: Warmup | null }>("/warmup");
  return body.warmup;
}
