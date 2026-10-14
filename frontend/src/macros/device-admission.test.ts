import { describe, expect, it } from "vitest";
import type { MacroEvent } from "../desktop-contract";
import { deviceAdmission } from "./device-admission";

const key = (action: "down" | "up", key_code = "KeyA", delay_ms = 0): MacroEvent => ({ type: "keyboard", key_code, action, delay_ms });
const click = (type: MacroEvent["type"] = "mouse_left", delay_ms = 0): MacroEvent[] => [{ type, action: "down", delay_ms }, { type, action: "up", delay_ms }];
describe("device admission", () => {
 it.each(["KeyF", "KeyD", "KeyS", "Tab", "AltLeft", "ControlRight", "ShiftRight", "MetaRight", "NumpadDivide", "Numpad9", "ArrowUp", "ContextMenu"])("official ordinary key %s is assignable", code => expect(deviceAdmission([key("down",code,10),key("up",code,10)]).eligible).toBe(true));
 it.each(["KeyA", "ControlLeft", "ShiftLeft"])("admits exact %s", code => expect(deviceAdmission([key("down",code,100),key("up",code,100)]).eligible).toBe(true));
 it.each(["mouse_left", "mouse_right", "mouse_middle", "mouse_back", "mouse_forward"] as const)("preserves %s", type => expect(deviceAdmission(click(type),255,2).eligible).toBe(true));
 it("admits ordered overlapping mixed chord without changing events", () => {
 const events=[key("down","ShiftLeft",100),...click("mouse_right",1271),key("down"),key("up"),key("up","ShiftLeft")];const before=structuredClone(events);
 expect(deviceAdmission(events).eligible).toBe(true);expect(events).toEqual(before);
 });
 it.each([
 ["empty",[]], ["incomplete",[key("down")]], ["unmatched",[key("up"),key("down")]], ["repeated",[key("down"),key("down"),key("up")]],
 ["unsupported",[key("down","F13"),key("up","F13")]], ["padded",[key("down"," KeyA"),key("up"," KeyA")]],
 ["mouse key",[{...click()[0],key_code:"KeyA"},click()[1]]], ["missing key",[{type:"keyboard",action:"down",delay_ms:0}]],
 ["overflow",[key("down","KeyA",51200),key("up")]], ["negative",[key("down","KeyA",-1),key("up")]], ["fractional",[key("down","KeyA",1.5),key("up")]],
 ] as [string,MacroEvent[]][])("rejects %s without loss",(_,events)=>{const before=structuredClone(events);expect(deviceAdmission(events).eligible).toBe(false);expect(deviceAdmission(events).reason).toBeTruthy();expect(events).toEqual(before)});
 it.each([0,256,-1,1.5,NaN])("rejects repeat %s",repeat=>expect(deviceAdmission(click(),repeat).eligible).toBe(false));
 it.each([3,255,-1,1.5,NaN])("rejects mode %s",mode=>expect(deviceAdmission(click(),1,mode).eligible).toBe(false));
 it("counts source, extensions and output pairs independently",()=>{
 const repeated=(count:number,delay:number)=>Array.from({length:count},()=>click("mouse_left",delay)).flat();
 expect(deviceAdmission(repeated(23,0)).eligible).toBe(true);
 expect(deviceAdmission(repeated(24,0)).eligible).toBe(false);
 const ten=repeated(5,1271);expect(deviceAdmission([...ten,...click()]).eligible).toBe(true);
 expect(deviceAdmission([...ten,...click("mouse_left",1271),...click()]).eligible).toBe(false);
 const fifty=repeated(23,0);for(let i=0;i<4;i++)fifty[i].delay_ms=1271;
 expect(deviceAdmission(fifty).eligible).toBe(true);fifty[4].delay_ms=1271;expect(deviceAdmission(fifty).eligible).toBe(false);
 expect(deviceAdmission([key("down","KeyA",51199),key("up","KeyA",51199)]).eligible).toBe(true);
 expect(deviceAdmission([key("down"),key("up","KeyA",51200)]).eligible).toBe(false);
 });
});
