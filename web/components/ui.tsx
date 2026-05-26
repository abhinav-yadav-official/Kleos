"use client";
import { ButtonHTMLAttributes, InputHTMLAttributes, ReactNode, TextareaHTMLAttributes } from "react";

export function Card({ children, className = "" }: { children: ReactNode; className?: string }) {
  return (
    <div className={`rounded-lg border border-border bg-panel p-5 ${className}`}>{children}</div>
  );
}

export function Button({
  children,
  variant = "primary",
  className = "",
  ...rest
}: ButtonHTMLAttributes<HTMLButtonElement> & { variant?: "primary" | "ghost" | "danger" }) {
  const base = "inline-flex items-center justify-center rounded-md text-sm font-medium h-9 px-4 transition disabled:opacity-50 disabled:cursor-not-allowed";
  const styles: Record<string, string> = {
    primary: "bg-accent text-bg hover:opacity-90",
    ghost: "border border-border text-text hover:bg-border/40",
    danger: "bg-err text-bg hover:opacity-90",
  };
  return (
    <button {...rest} className={`${base} ${styles[variant]} ${className}`}>
      {children}
    </button>
  );
}

export function Input(props: InputHTMLAttributes<HTMLInputElement>) {
  return (
    <input
      {...props}
      className={`block w-full rounded-md border border-border bg-bg px-3 h-9 text-sm focus:outline-none focus:border-accent ${props.className ?? ""}`}
    />
  );
}

export function Textarea(props: TextareaHTMLAttributes<HTMLTextAreaElement>) {
  return (
    <textarea
      {...props}
      className={`block w-full rounded-md border border-border bg-bg px-3 py-2 text-sm focus:outline-none focus:border-accent ${props.className ?? ""}`}
    />
  );
}

export function Label({ children, htmlFor }: { children: ReactNode; htmlFor?: string }) {
  return (
    <label htmlFor={htmlFor} className="text-xs uppercase tracking-wide text-muted">
      {children}
    </label>
  );
}

export function Badge({ children, tone = "default" }: { children: ReactNode; tone?: "default" | "ok" | "warn" | "err" }) {
  const tones: Record<string, string> = {
    default: "border-border text-muted",
    ok: "border-ok/50 text-ok",
    warn: "border-warn/50 text-warn",
    err: "border-err/50 text-err",
  };
  return (
    <span className={`inline-flex items-center rounded-full border px-2 py-0.5 text-xs ${tones[tone]}`}>
      {children}
    </span>
  );
}

export function ErrorBanner({ message }: { message: string | null }) {
  if (!message) return null;
  return (
    <div className="rounded-md border border-err/50 bg-err/10 text-err text-sm px-3 py-2">
      {message}
    </div>
  );
}
