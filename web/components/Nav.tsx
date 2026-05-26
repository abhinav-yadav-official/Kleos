"use client";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { clearTokens, getAccess, logout, me, User } from "@/lib/api";

const links = [
  { href: "/dashboard", label: "Dashboard" },
  { href: "/campaigns", label: "Campaigns" },
  { href: "/onboarding/recipients", label: "Recipients" },
  { href: "/settings", label: "Settings" },
];

export function Nav() {
  const router = useRouter();
  const pathname = usePathname();
  const [user, setUser] = useState<User | null>(null);

  useEffect(() => {
    if (!getAccess()) {
      router.replace("/");
      return;
    }
    me().then(setUser).catch(() => {
      clearTokens();
      router.replace("/");
    });
  }, [router]);

  return (
    <header className="flex items-center justify-between mb-6">
      <div className="flex items-center gap-6">
        <Link href="/dashboard" className="text-lg font-semibold">Kleos</Link>
        <nav className="flex gap-2 text-sm">
          {links.map((l) => {
            const active = pathname?.startsWith(l.href);
            return (
              <Link
                key={l.href}
                href={l.href}
                className={`rounded px-2 py-1 ${active ? "bg-border/60 text-text" : "text-muted hover:text-text"}`}
              >
                {l.label}
              </Link>
            );
          })}
        </nav>
      </div>
      <div className="flex items-center gap-3 text-sm">
        {user && <span className="text-muted">{user.email}</span>}
        <button
          onClick={async () => {
            await logout();
            router.push("/");
          }}
          className="text-muted hover:text-text"
        >
          Log out
        </button>
      </div>
    </header>
  );
}
