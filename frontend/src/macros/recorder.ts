import type { Macro } from "../desktop-contract";

export type DelayPolicy = "minimum" | "fixed" | "recorded";
type Input = Omit<Macro["events"][number], "delay_ms">;
export type Take = {
  policy: DelayPolicy;
  events: Macro["events"];
  held: Input[];
  previousDown?: { index: number; time: number };
};
export const startTake = (policy: DelayPolicy): Take => ({ policy, events: [], held: [] });
const sameInput = (a: Input, b: Input) => a.type === b.type && a.key_code === b.key_code;

// Immutable take snapshots; timing updates never reach saved or earlier draft events.
export function captureInput(take: Take, input: Input, time: number): Take {
  const held = take.held.some(item => sameInput(item, input));
  if ((input.action === "down" && held) || (input.action === "up" && !held)) return take;
  const events = take.events.map(event => ({ ...event }));
  if (input.action === "down" && take.policy === "recorded" && take.previousDown) {
    events[take.previousDown.index].delay_ms = Math.max(0, Math.round(time - take.previousDown.time));
  }
  const previousDown = input.action === "down" ? { index: events.length, time } : take.previousDown;
  events.push({ ...input, delay_ms: take.policy === "fixed" ? 10 : 0 });
  return { ...take, events, previousDown, held: input.action === "down"
    ? [...take.held, { ...input }] : take.held.filter(item => !sameInput(item, input)) };
}

// Release remaining held identities in press order without sampling a clock.
export function stopTake(take: Take): Macro["events"] {
  return [...take.events.map(event => ({ ...event })), ...take.held.map(input => ({
    ...input, action: "up" as const, delay_ms: take.policy === "fixed" ? 10 : 0,
  }))];
}
