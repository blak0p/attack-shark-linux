import { expect, it } from "vitest";
import { startTake, captureInput, stopTake } from "./recorder";

const key = (code: string, action: "down" | "up") => ({ type: "keyboard" as const, key_code: code, action });
it("measures only between downs with after-event orientation, never releases or stop", () => {
  let take = startTake("recorded");
  take = captureInput(take, key("KeyA", "down"), 0);
  take = captureInput(take, key("KeyA", "up"), 30);
  take = captureInput(take, key("KeyB", "down"), 120);
  take = captureInput(take, key("KeyB", "up"), 180);
  expect(stopTake(take)).toEqual([
    { ...key("KeyA", "down"), delay_ms: 120 }, { ...key("KeyA", "up"), delay_ms: 0 },
    { ...key("KeyB", "down"), delay_ms: 0 }, { ...key("KeyB", "up"), delay_ms: 0 },
  ]);
});
it.each([["minimum", 0], ["fixed", 10]] as const)("%s applies to every event including safe auto-release", (policy, delay_ms) => {
  let take = startTake(policy);
  take = captureInput(take, key("UnknownPhysical", "down"), 0);
  expect(stopTake(take)).toEqual([{ ...key("UnknownPhysical", "down"), delay_ms }, { ...key("UnknownPhysical", "up"), delay_ms }]);
});
it("preserves modifiers, suppresses repeats and unmatched ups, leaves prior snapshots unchanged", () => {
  const first = captureInput(startTake("recorded"), key("ControlLeft", "down"), 0);
  const repeated = captureInput(first, key("ControlLeft", "down"), 20);
  expect(repeated).toBe(first);
  const next = captureInput(first, key("UnknownPhysical", "down"), 100);
  expect(first.events[0].delay_ms).toBe(0);
  expect(next.events[0].delay_ms).toBe(100);
  expect(captureInput(next, key("KeyB", "up"), 200)).toBe(next);
  expect(stopTake(next).map(e => e.key_code)).toEqual(["ControlLeft", "UnknownPhysical", "ControlLeft", "UnknownPhysical"]);
});
it("records all five mouse identities in down/up order", () => {
  let take = startTake("minimum");
  const types = ["mouse_left", "mouse_middle", "mouse_right", "mouse_back", "mouse_forward"] as const;
  for (const type of types) for (const action of ["down", "up"] as const) take = captureInput(take, { type, action }, 0);
  expect(stopTake(take)).toEqual(types.flatMap(type => [{ type, action: "down", delay_ms: 0 }, { type, action: "up", delay_ms: 0 }]));
});
