import { afterEach, describe, expect, it, vi } from "vitest";

import {
  readCreationProfile,
  saveCreationProfile,
} from "../src/client/lib/creation-profile.ts";

describe("Host creation profile", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("defaults to invitation-only entry without a room password", () => {
    vi.stubGlobal("window", {
      localStorage: { getItem: () => null },
    });

    expect(readCreationProfile()).toEqual({
      codeEntryPolicy: "private",
      roomPassword: null,
    });
  });

  it("persists the accepted creation preferences in same-origin storage", () => {
    const values = new Map<string, string>();
    vi.stubGlobal("window", {
      localStorage: {
        getItem: (key: string) => values.get(key) ?? null,
        setItem: (key: string, value: string) => values.set(key, value),
        removeItem: (key: string) => values.delete(key),
      },
    });

    saveCreationProfile({
      codeEntryPolicy: "private",
      roomPassword: "room-password",
    });

    expect(readCreationProfile()).toEqual({
      codeEntryPolicy: "private",
      roomPassword: "room-password",
    });
    expect(values.get("piik:host-creation-profile:v1")).not.toContain(
      "SITE_ACCESS_PASSWORD",
    );

    saveCreationProfile({ codeEntryPolicy: "private", roomPassword: null });
    expect(readCreationProfile()).toEqual({
      codeEntryPolicy: "private",
      roomPassword: null,
    });
  });

  it("discards removed policy records", () => {
    const removeItem = vi.fn();
    vi.stubGlobal("window", {
      localStorage: {
        getItem: () =>
          JSON.stringify({ codeEntryPolicy: "password", roomPassword: null }),
        removeItem,
      },
    });

    expect(readCreationProfile()).toEqual({
      codeEntryPolicy: "private",
      roomPassword: null,
    });
    expect(removeItem).toHaveBeenCalledOnce();
  });

  it("keeps an explicitly saved open policy", () => {
    vi.stubGlobal("window", {
      localStorage: { getItem: () => JSON.stringify({ codeEntryPolicy: "open", roomPassword: null }) },
    });
    expect(readCreationProfile().codeEntryPolicy).toBe("open");
  });

  it("uses invitation-only entry when browser storage is unavailable", () => {
    vi.stubGlobal("window", {
      localStorage: { getItem: () => { throw new Error("storage unavailable"); } },
    });
    expect(readCreationProfile()).toEqual({ codeEntryPolicy: "private", roomPassword: null });
  });
});
