interface KlIconProps {
  variant?: "full" | "favicon";
  className?: string;
}

export function KlIcon({ variant = "full", className = "" }: KlIconProps) {
  if (variant === "favicon") {
    return (
      <svg viewBox="0 0 32 32" className={className} aria-hidden>
        <text x="16" y="22" textAnchor="middle" fill="currentColor" fontSize="16" fontWeight="700" fontFamily="system-ui, sans-serif">KL</text>
      </svg>
    );
  }
  return (
    <svg viewBox="0 0 40 40" className={className} aria-hidden>
      <rect x="2" y="2" width="36" height="36" rx="5" fill="none" stroke="currentColor" strokeWidth="2.5" />
      <text x="20" y="26" textAnchor="middle" fill="currentColor" fontSize="17" fontWeight="700" fontFamily="system-ui, sans-serif">KL</text>
    </svg>
  );
}
