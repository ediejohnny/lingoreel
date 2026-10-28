// Work that waits a moment before it runs, such as a call to the Go side
// while a slider still moves.

/**
 * after runs fn once wait milliseconds pass and returns what cancels it. A cancelled fn never starts; one already
 * started learns through current() that what it brings is no longer wanted, so an older answer never replaces a newer.
 * The returned function fits an effect's cleanup.
 */
export function after(wait: number, fn: (current: () => boolean) => void): () => void {
  let current = true;
  const timer = setTimeout(() => fn(() => current), wait);
  return () => {
    current = false;
    clearTimeout(timer);
  };
}
