import { readFileSync } from "node:fs";
import { pathToFileURL } from "node:url";
function instant(value) {
  const match =
    /^(\d{4}-\d\d-\d\d)[T ](\d\d:\d\d:\d\d)(?:\.(\d{1,9}))?(Z|\+00(?::00)?)$/.exec(
      value,
    );
  if (!match) throw new Error("Invalid UTC evidence time");
  return (
    BigInt(Date.parse(`${match[1]}T${match[2]}Z`)) * 1000000n +
    BigInt((match[3] || "").padEnd(9, "0"))
  );
}
export function verifyEvidence(capture, rows) {
  if (!/^[0-9a-f-]{36}$/.test(capture.id) || capture.target !== "http://web/")
    throw new Error("Invalid browser monitor evidence");
  for (const card of ["raw", "availability"]) {
    const value = capture[card],
      row = rows.find(
        (row) => row.id === value.checkId && row.monitor_id === capture.id,
      );
    if (
      !row ||
      row.result_kind !== "policy_rejected" ||
      row.http_status !== null ||
      !Number.isInteger(row.duration_ms) ||
      row.duration_ms < 0 ||
      row.duration_ms > 20000 ||
      (card === "raw" && row.duration_ms !== value.durationMs) ||
      instant(row.completed_at) !== instant(value.completedAt)
    )
      throw new Error(`Durable ${card} evidence mismatch`);
  }
}
export function verifyInventoryEvidence(capture,monitors) {
 const uuid=value=>typeof value==='string'&&/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/.test(value)&&value!=='00000000-0000-0000-0000-000000000000';
 const inventory=capture?.inventory;
 if(capture?.target!=='http://web/'||!inventory||Object.keys(inventory).length!==2||!Array.isArray(inventory.ids)||inventory.ids.length!==21||!inventory.ids.every(uuid)||new Set(inventory.ids).size!==21||!uuid(inventory.selectedId)||!inventory.ids.includes(inventory.selectedId)||!Array.isArray(monitors)) throw new Error('Invalid browser inventory evidence');
 for(const id of inventory.ids) {
  const matching=monitors.filter(row=>row?.id===id);
  if(matching.length!==1||matching[0].target_url!==capture.target) throw new Error('Durable inventory Monitor mismatch');
 }
}
export function verifySchedulingEvidence(capture, rows, monitors) {
  const uuid=value=>typeof value==='string'&&/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/.test(value)&&value!=='00000000-0000-0000-0000-000000000000';
  const closed=(value,keys)=>value!==null&&typeof value==='object'&&!Array.isArray(value)&&Object.keys(value).length===keys.length&&keys.every(key=>Object.hasOwn(value,key));
  const value=capture?.scheduling;
  if(!closed(value,['monitorId','pausedState','resumedState','checkIdsBeforePause','checkIdsWhilePaused','checkIdAfterResume','restarts']) || value.monitorId!==capture.id || !uuid(value.monitorId) || value.pausedState!=='paused' || value.resumedState!=='active' || !uuid(value.checkIdAfterResume))throw Error('Invalid scheduling evidence');
  for(const ids of [value.checkIdsBeforePause,value.checkIdsWhilePaused])if(!Array.isArray(ids)||ids.length===0||!ids.every(uuid)||new Set(ids).size!==ids.length)throw Error('Invalid scheduling CheckRun IDs');
  const before=[...value.checkIdsBeforePause].sort(),during=[...value.checkIdsWhilePaused].sort();
  if(JSON.stringify(before)!==JSON.stringify(during)||before.includes(value.checkIdAfterResume))throw Error('New CheckRun admitted while paused or missing resumed claim');
  if(!closed(value.restarts,['api','checker']))throw Error('Missing scheduling restart checkpoints');
  for(const service of ['api','checker']){
    const restart=value.restarts[service];
    if(!closed(restart,['beforeStartedAt','afterStartedAt']))throw Error('Invalid scheduling restart checkpoint');
    for(const time of Object.values(restart))if(typeof time!=='string'||!/^\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d(?:\.\d{1,9})?Z$/.test(time)||new Date(time).toISOString().slice(0,19)!==time.slice(0,19))throw Error('Invalid restart UTC time');
    if(instant(restart.afterStartedAt)<=instant(restart.beforeStartedAt))throw Error('Restart timestamp did not advance');
  }
  if(!Array.isArray(rows)||!Array.isArray(monitors))throw Error('Missing durable scheduling rows');
  for(const checkID of [...before,value.checkIdAfterResume])if(rows.filter(row=>row.id===checkID&&row.monitor_id===value.monitorId).length!==1)throw Error('Durable scheduling CheckRun linkage mismatch');
  const monitor=monitors.filter(row=>row.id===value.monitorId);
  if(monitor.length!==1||monitor[0].paused!==false)throw Error('Durable resumed Monitor state mismatch');
}
export function verifyDeletionEvidence(capture, rows, monitors) {
  const uuid=value=>typeof value==='string'&&/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/.test(value)&&value!=='00000000-0000-0000-0000-000000000000';
  const closed=(value,keys)=>value!==null&&typeof value==='object'&&!Array.isArray(value)&&Object.keys(value).length===keys.length&&keys.every(key=>Object.hasOwn(value,key));
  const value=capture?.deletion;
  if(!closed(value,['monitorId','deletedState','readsAfterDelete','inventoryExcludes','checkRunsBeforeDelete','checkRunsAfterDelete','deliveryFailures','restarts']) || !uuid(value.monitorId) || value.monitorId===capture.id || value.deletedState!=='deleted')throw Error('Invalid deletion evidence');
  const reads=closed(value.readsAfterDelete,['monitor','availability','latestResult','scheduling'])?Object.values(value.readsAfterDelete):null;
  if(!reads||reads.length!==4||reads.some(status=>status!==404))throw Error('Deleted monitor reads are not uniformly 404');
  if(value.inventoryExcludes!==true)throw Error('Inventory exclusion was not recorded');
  for(const ids of [value.checkRunsBeforeDelete,value.checkRunsAfterDelete])if(!Array.isArray(ids)||!ids.every(uuid))throw Error('Invalid deletion CheckRun IDs');
  if(value.checkRunsBeforeDelete.length===0||value.checkRunsAfterDelete.length!==0)throw Error('Deletion must record claimed work and an empty durable remainder');
  if(!closed(value.deliveryFailures,['attempts','status'])||value.deliveryFailures.attempts!==1||value.deliveryFailures.status!==404)throw Error('Delivery failure must be exactly one 404 not-found attempt');
  if(!closed(value.restarts,['api','checker']))throw Error('Missing deletion restart checkpoints');
  for(const service of ['api','checker']){
    const restart=value.restarts[service];
    if(!closed(restart,['beforeStartedAt','afterStartedAt']))throw Error('Invalid deletion restart checkpoint');
    for(const time of Object.values(restart))if(typeof time!=='string'||!/^\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d(?:\.\d{1,9})?Z$/.test(time)||new Date(time).toISOString().slice(0,19)!==time.slice(0,19))throw Error('Invalid restart UTC time');
    if(instant(restart.afterStartedAt)<=instant(restart.beforeStartedAt))throw Error('Restart timestamp did not advance');
  }
  if(!Array.isArray(rows)||!Array.isArray(monitors))throw Error('Missing durable deletion rows');
  if(rows.some(row=>row.monitor_id===value.monitorId||row.id===value.monitorId))throw Error('Orphan durable CheckRun survived deletion');
  if(monitors.some(row=>row.id===value.monitorId))throw Error('Deleted Monitor row survived');
}
if (
  process.argv[1] &&
  import.meta.url === pathToFileURL(process.argv[1]).href
) {
  verifyEvidence(
    JSON.parse(readFileSync(process.argv[2], "utf8")),
    JSON.parse(readFileSync(process.argv[3], "utf8")),
  );
  verifyInventoryEvidence(JSON.parse(readFileSync(process.argv[2],"utf8")),JSON.parse(readFileSync(process.argv[4],"utf8")));
  verifySchedulingEvidence(JSON.parse(readFileSync(process.argv[2],"utf8")),JSON.parse(readFileSync(process.argv[3],"utf8")),JSON.parse(readFileSync(process.argv[4],"utf8")));
  verifyDeletionEvidence(JSON.parse(readFileSync(process.argv[2],"utf8")),JSON.parse(readFileSync(process.argv[5],"utf8")),JSON.parse(readFileSync(process.argv[6],"utf8")));
  console.log(
    "Independent browser snapshots, inventory IDs, scheduling and deletion evidence match durable rows.",
  );
}
