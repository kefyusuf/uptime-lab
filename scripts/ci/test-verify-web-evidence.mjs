import { test } from "node:test";
import assert from "node:assert/strict";
import { verifyEvidence, verifyInventoryEvidence } from "./verify-web-evidence.mjs";
const id = "aa29443e-c597-4e7b-9202-a4762e0e04c0",
  a = "bb29443e-c597-4e7b-9202-a4762e0e04c0",
  b = "cc29443e-c597-4e7b-9202-a4762e0e04c0";
const capture = {
  id,
  target: "http://web/",
  raw: {
    checkId: a,
    completedAt: "2026-10-02T12:00:00.123456Z",
    durationMs: 0,
  },
  availability: { checkId: b, completedAt: "2026-10-02T12:00:01Z" },
};
const rows = [a, b].map((id, index) => ({
  id,
  monitor_id: capture.id,
  result_kind: "policy_rejected",
  http_status: null,
  duration_ms: 0,
  completed_at: index
    ? "2026-10-02T12:00:01+00:00"
    : "2026-10-02T12:00:00.123456+00:00",
}));
const inventoryIds=[id,...Array.from({length:20},(_,i)=>`018f22d3-1d6a-7cc0-a37b-${(0x1000+i).toString(16).padStart(12,'0')}`)];
const inventoryCapture={...capture,inventory:{ids:inventoryIds,selectedId:id}};
const monitors=inventoryIds.map(id=>({id,target_url:'http://web/'}));
test('inventory captures match their durable Monitor rows',()=>assert.doesNotThrow(()=>verifyInventoryEvidence(inventoryCapture,monitors)));
for(const field of ['id','target_url']) test(`inventory rejects wrong ${field}`,()=>{
 const mutated=structuredClone(monitors);mutated[0][field]='wrong';assert.throws(()=>verifyInventoryEvidence(inventoryCapture,mutated));
});
test('inventory rejects missing durable row',()=>assert.throws(()=>verifyInventoryEvidence(inventoryCapture,monitors.slice(1))));
test('inventory rejects duplicate IDs and unknown selection',()=>{
 assert.throws(()=>verifyInventoryEvidence({...inventoryCapture,inventory:{ids:[id,...inventoryIds.slice(0,20)],selectedId:id}},monitors));
 assert.throws(()=>verifyInventoryEvidence({...inventoryCapture,inventory:{ids:inventoryIds,selectedId:a}},monitors));
});
test('inventory rejects absent or malformed capture',()=>{
 for(const value of [null,capture,{...inventoryCapture,inventory:{ids:[],selectedId:id}},{...inventoryCapture,inventory:{ids:inventoryIds,selectedId:id,extra:true}}]) assert.throws(()=>verifyInventoryEvidence(value,monitors));
});
test("independent captured rows remain valid after newer completion", () =>
  assert.doesNotThrow(() =>
    verifyEvidence(capture, [...rows, { ...rows[1], id: "later" }]),
  ));
for (const field of [
  "monitor_id",
  "result_kind",
  "completed_at",
  "http_status",
  "duration_ms",
])
  test(`rejects mismatched ${field}`, () => {
    const mutated = structuredClone(rows);
    mutated[0][field] =
      field === "http_status" ? 200 : field === "duration_ms" ? null : "wrong";
    assert.throws(() => verifyEvidence(capture, mutated));
  });
