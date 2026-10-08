// The steps of the first run's welcome, in order.

/** welcomeSteps are the welcome's steps: the app's language, the languages spoken and studied, the mode, the tools. */
export const welcomeSteps = ["language", "languages", "mode", "tools"] as const;

export type WelcomeStep = (typeof welcomeSteps)[number];

/** stepAfter is the step "next" or "back" leads to; the first and the last stay where they are. */
export function stepAfter(step: WelcomeStep, move: "next" | "back"): WelcomeStep {
  const index = welcomeSteps.indexOf(step) + (move === "next" ? 1 : -1);
  return welcomeSteps[Math.min(Math.max(index, 0), welcomeSteps.length - 1)] ?? step;
}
