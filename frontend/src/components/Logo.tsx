/**
 * Logo is the app's mark, the same drawing as its icon (spdev icon): a dark
 * rounded square with a play sign split across the middle, its halves in
 * two reds. It is only decoration: the app's name always goes beside it.
 */
export function Logo({ size = 24 }: { size?: number }) {
  return (
    <svg aria-hidden width={size} height={size} viewBox="0 0 100 100" className="shrink-0">
      <rect width="100" height="100" rx="19" fill="#0C111F" />
      <polygon points="24.77,15.35 81.8,49.5 24.77,49.5" fill="#E50914" />
      <polygon points="24.77,50.5 81.8,50.5 24.77,84.65" fill="#FF6F61" />
    </svg>
  );
}
