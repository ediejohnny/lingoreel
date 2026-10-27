/**
 * Logo is the app's mark, the same drawing as its icon (lrdev icon): a dark
 * rounded square with a play sign split across the middle, its halves a neon
 * red and a pink. It is only decoration: the app's name always goes beside it.
 */
export function Logo({ size = 24 }: { size?: number }) {
  return (
    <svg aria-hidden width={size} height={size} viewBox="0 0 100 100" className="shrink-0">
      <rect width="100" height="100" rx="19" fill="#0C111F" />
      <polygon points="27.71,15.35 84.74,49.5 27.71,49.5" fill="#FF003C" />
      <polygon points="27.71,50.5 84.74,50.5 27.71,84.65" fill="#FF5C8A" />
    </svg>
  );
}
