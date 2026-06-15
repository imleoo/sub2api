import { describe, expect, it } from "vitest";
import { normalizeSupportedModelScopesForPlatform } from "../groupsSupportedModelScopes";

describe("normalizeSupportedModelScopesForPlatform", () => {
  it("drops hidden model scopes for OpenAI groups", () => {
    expect(
      normalizeSupportedModelScopesForPlatform("openai", [
        "claude",
        "gemini_text",
        "gemini_image",
      ]),
    ).toEqual([]);
  });

  it("drops hidden model scopes for Anthropic groups", () => {
    expect(normalizeSupportedModelScopesForPlatform("anthropic", ["claude"])).toEqual([]);
  });

  it("returns an empty array when no scopes are provided", () => {
    expect(normalizeSupportedModelScopesForPlatform("gemini", undefined)).toEqual([]);
  });
});
