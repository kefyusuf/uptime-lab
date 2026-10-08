import { test } from "node:test";
import assert from "node:assert/strict";
import { verifyEvidence, verifyInventoryEvidence, verifySchedulingEvidence } from "./verify-web-evidence.mjs";
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
const resumed='dd29443e-c597-4e7b-9202-a4762e0e04c0';
const schedulingCapture={...inventoryCapture,scheduling:{monitorId:id,pausedState:'paused',resumedState:'active',checkIdsBeforePause:[a,b],checkIdsWhilePaused:[a,b],checkIdAfterResume:resumed,restarts:{api:{beforeStartedAt:'2026-10-08T12:00:00.123456789Z',afterStartedAt:'2026-10-08T12:00:00.123456790Z'},checker:{beforeStartedAt:'2026-10-08T12:00:01Z',afterStartedAt:'2026-10-08T12:00:02Z'}}}};
const schedulingRows=[...rows,{...rows[0],id:resumed}];
const schedulingMonitors=monitors.map(row=>({...row,paused:false}));
test('scheduling links durable resumed claim and actual later restart timestamps',()=>assert.doesNotThrow(()=>verifySchedulingEvidence(schedulingCapture,schedulingRows,schedulingMonitors)));
for(const mutation of [
 c=>{delete c.scheduling},c=>{delete c.scheduling.restarts.api},c=>{delete c.scheduling.restarts.checker.afterStartedAt},
 c=>{c.scheduling.monitorId=a},c=>{c.scheduling.pausedState='active'},c=>{c.scheduling.resumedState='paused'},
 c=>{c.scheduling.checkIdsWhilePaused.push(resumed)},c=>{c.scheduling.checkIdAfterResume=a},c=>{c.scheduling.checkIdAfterResume='invalid'},
 c=>{c.scheduling.restarts.api.afterStartedAt=c.scheduling.restarts.api.beforeStartedAt},
 c=>{c.scheduling.restarts.checker.afterStartedAt='2026-10-08T12:00:00Z'},c=>{c.scheduling.extra=true},c=>{c.scheduling.restarts.api.containerId='same'},
])test('scheduling rejects mutated acceptance evidence '+mutation.toString(),()=>{const mutated=structuredClone(schedulingCapture);mutation(mutated);assert.throws(()=>verifySchedulingEvidence(mutated,schedulingRows,schedulingMonitors))});
test('scheduling rejects resumed claim without durable linkage',()=>{
 assert.throws(()=>verifySchedulingEvidence(schedulingCapture,rows,schedulingMonitors));
 assert.throws(()=>verifySchedulingEvidence(schedulingCapture,[...rows,{...schedulingRows.at(-1),monitor_id:a}],schedulingMonitors));
 assert.throws(()=>verifySchedulingEvidence(schedulingCapture,schedulingRows,schedulingMonitors.map(row=>({...row,paused:true}))));
});
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
