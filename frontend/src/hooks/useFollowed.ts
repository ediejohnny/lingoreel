import { useEffect, useState } from "react";

/**
 * useFollowed is a value the Go side tells again at each change: it is read when the screen shows, since a change may
 * have been told before, and then followed. A change heard before the first reading answers wins over that answer,
 * which is older. Until the reading answers, or when it fails, the value is null.
 */
export function useFollowed<T>(read: () => Promise<T>, follow: (listener: (value: T) => void) => () => void): T | null {
  const [value, setValue] = useState<T | null>(null);
  useEffect(() => {
    let alive = true;
    let heard = false;
    read()
      .then((first) => alive && !heard && setValue(first))
      .catch(() => undefined);
    const stop = follow((next) => {
      heard = true;
      setValue(next);
    });
    return () => {
      alive = false;
      stop();
    };
  }, [read, follow]);
  return value;
}
