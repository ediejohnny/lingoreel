import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { after } from "./timing";

describe("after", () => {
  beforeEach(() => vi.useFakeTimers());
  afterEach(() => vi.useRealTimers());

  it("runs once the wait is over, and not before", () => {
    const fn = vi.fn();
    after(150, fn);
    vi.advanceTimersByTime(149);
    expect(fn).not.toHaveBeenCalled();
    vi.advanceTimersByTime(1);
    expect(fn).toHaveBeenCalledTimes(1);
  });

  it("never runs once cancelled while waiting", () => {
    const fn = vi.fn();
    after(150, fn)();
    vi.advanceTimersByTime(1000);
    expect(fn).not.toHaveBeenCalled();
  });

  it("tells a run already started that its answer is no longer wanted once cancelled", () => {
    let current = () => false;
    const cancel = after(150, (isCurrent) => (current = isCurrent));
    vi.advanceTimersByTime(150);
    expect(current()).toBe(true);
    cancel();
    expect(current()).toBe(false);
  });
});
