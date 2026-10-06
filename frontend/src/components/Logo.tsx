/**
 * Logo is the app's mark, the same drawing as its icon (spdev icon): a dark
 * rounded square with a play sign split across the middle, its halves in
 * two reds. It is only decoration: the app's name always goes beside it.
 */
export function Logo({ size = 24 }: { size?: number }) {
  return (
    <svg aria-hidden width={size} height={size} viewBox="0 0 100 100" className="shrink-0">
      <rect width="100" height="100" rx="19" fill="#0C111F" />
      <polygon points="27.4,18.5 79.2,49.5 27.4,49.5" fill="#E50914" />
      <polygon points="27.4,50.5 79.2,50.5 27.4,81.5" fill="#FF6F61" />
    </svg>
  );
}
