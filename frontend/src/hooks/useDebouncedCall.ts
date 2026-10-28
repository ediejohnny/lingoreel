import { useCallback, useEffect, useRef } from "react";
import { after } from "../timing";

/**
 * useDebouncedCall returns a function that calls fn wait milliseconds after it stops being called, with the last
 * arguments: a slider that sends many values keeps only where it stopped. A call still waiting when the screen goes
 * runs at once, so the last choice is never lost.
 */
export function useDebouncedCall<A extends unknown[]>(fn: (...args: A) => void, wait: number): (...args: A) => void {
  const latest = useRef(fn);
  const waiting = useRef<{ run: () => void; cancel: () => void } | null>(null);
  useEffect(() => {
    latest.current = fn;
  });
  useEffect(
    () => () => {
      const call = waiting.current;
      call?.cancel();
      call?.run();
    },
    [],
  );
  return useCallback(
    (...args: A) => {
      waiting.current?.cancel();
      const run = () => {
        waiting.current = null;
        latest.current(...args);
      };
      waiting.current = { run, cancel: after(wait, run) };
    },
    [wait],
  );
}
