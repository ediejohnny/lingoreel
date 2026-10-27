import { describe, expect, it } from "vitest";
import { stepAfter, welcomeSteps } from "./welcome";

describe("stepAfter", () => {
  it("walks the steps in order and stops at both ends", () => {
    expect(welcomeSteps).toEqual(["language", "languages", "mode", "tools"]);
    expect(stepAfter("language", "next")).toBe("languages");
    expect(stepAfter("mode", "next")).toBe("tools");
    expect(stepAfter("tools", "next")).toBe("tools");
    expect(stepAfter("languages", "back")).toBe("language");
    expect(stepAfter("language", "back")).toBe("language");
  });
});
