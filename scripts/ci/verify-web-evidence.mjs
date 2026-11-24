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
export function verifyHistoryEvidence(capture, rows) {
  const uuid=value=>typeof value==='string'&&/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/.test(value)&&value!=='00000000-0000-0000-0000-000000000000';
  const closed=(value,keys)=>value!==null&&typeof value==='object'&&!Array.isArray(value)&&Object.keys(value).length===keys.length&&keys.every(key=>Object.hasOwn(value,key));
  const value=capture?.history;
  if(!closed(value,['monitorId','firstPageIds','secondPageIds','orderConfirmed','readsAfterRestart','restarts']) || !uuid(value.monitorId) || value.monitorId===capture.id)throw Error('Invalid history evidence');
  for(const ids of [value.firstPageIds,value.secondPageIds])if(!Array.isArray(ids)||!ids.every(uuid)||new Set(ids).size!==ids.length)throw Error('Invalid history CheckRun IDs');
  if(value.firstPageIds.length!==20||value.secondPageIds.length===0)throw Error('History pages must be 20 newest rows plus a real continuation');
  if(value.orderConfirmed!==true)throw Error('History order was not confirmed');
  const reads=closed(value.readsAfterRestart,['history'])?Object.values(value.readsAfterRestart):null;
  if(!reads||reads.length!==1||reads[0]!==200)throw Error('Restarted history read must be 200');
  if(!closed(value.restarts,['api']))throw Error('Missing history restart checkpoints');
  const restart=value.restarts.api;
  if(!closed(restart,['beforeStartedAt','afterStartedAt']))throw Error('Invalid history restart checkpoint');
  for(const time of Object.values(restart))if(typeof time!=='string'||!/^[^T]+T[0-9:.]+Z$/.test(time))throw Error('Invalid restart UTC time');
  if(instant(restart.afterStartedAt)<=instant(restart.beforeStartedAt))throw Error('Restart timestamp did not advance');
  if(!Array.isArray(rows)||rows.length!==25)throw Error('Durable history rows must back the captured pages');
  const ordered=[...rows].sort((a,b)=>{const delta=instant(b.completed_at)-instant(a.completed_at);return delta===0n?(a.id<b.id?1:a.id>b.id?-1:0):delta>0n?1:-1;});
  const durableIds=ordered.map(row=>row.id);
  if(JSON.stringify(value.firstPageIds)!==JSON.stringify(durableIds.slice(0,20))||JSON.stringify(value.secondPageIds)!==JSON.stringify(durableIds.slice(20)))throw Error('Durable history page linkage mismatch');
}
export function verifyConfirmationRecoveryEvidence(capture, rows, monitors) {
  const uuid=value=>typeof value==='string'&&/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/.test(value)&&value!=='00000000-0000-0000-0000-000000000000';
  const closed=(value,keys)=>value!==null&&typeof value==='object'&&!Array.isArray(value)&&Object.keys(value).length===keys.length&&keys.every(key=>Object.hasOwn(value,key));
  const value=capture?.confirmationRecovery;
  if(!closed(value,['monitorId','transientAssessment','confirmedAssessment','recoveringAssessment','recoveredAssessment','readsAfterRestart','restarts']) || !uuid(value.monitorId) || value.monitorId===capture.id)throw Error('Invalid confirmation/recovery evidence');
  const expected={transientAssessment:['unknown','awaiting_confirmation'],confirmedAssessment:['unavailable','unexpected_http_status'],recoveringAssessment:['unavailable','recovering'],recoveredAssessment:['available','successful_response']};
  for(const key of Object.keys(expected)){
    const assessment=value[key];
    if(!closed(assessment,['status','reason','checkId','completedAt'])||assessment.status!==expected[key][0]||assessment.reason!==expected[key][1]||!uuid(assessment.checkId))throw Error('Invalid confirmation/recovery assessment');
    instant(assessment.completedAt);
  }
  const checkIds=[value.transientAssessment.checkId,value.confirmedAssessment.checkId,value.recoveringAssessment.checkId,value.recoveredAssessment.checkId];
  if(new Set(checkIds).size!==4)throw Error('Confirmation/recovery assessments must reference distinct streak rows');
  const reads=closed(value.readsAfterRestart,['availability','status','reason','checkId'])?value.readsAfterRestart:null;
  if(!reads||reads.availability!==200||reads.status!=='available'||reads.reason!=='successful_response'||reads.checkId!==value.recoveredAssessment.checkId)throw Error('Restarted API did not serve the recovered assessment');
  if(!closed(value.restarts,['api']))throw Error('Missing confirmation/recovery restart checkpoints');
  const restart=value.restarts.api;
  if(!closed(restart,['beforeStartedAt','afterStartedAt']))throw Error('Invalid confirmation/recovery restart checkpoint');
  for(const time of Object.values(restart))if(typeof time!=='string'||!/^\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d(?:\.\d{1,9})?Z$/.test(time)||new Date(time).toISOString().slice(0,19)!==time.slice(0,19))throw Error('Invalid restart UTC time');
  if(instant(restart.afterStartedAt)<=instant(restart.beforeStartedAt))throw Error('Restart timestamp did not advance');
  if(!Array.isArray(rows)||rows.length!==5||rows.some(row=>row.monitor_id!==value.monitorId))throw Error('Durable confirmation/recovery streak must be five linked rows');
  const ordered=[...rows].sort((a,b)=>{const delta=instant(b.completed_at)-instant(a.completed_at);return delta===0n?(a.id<b.id?1:a.id>b.id?-1:0):delta>0n?1:-1;});
  const link=(assessment,row)=>{if(row.id!==assessment.checkId||instant(row.completed_at)!==instant(assessment.completedAt))throw Error('Durable confirmation/recovery assessment linkage mismatch');};
  link(value.recoveredAssessment,ordered[0]);
  link(value.recoveringAssessment,ordered[1]);
  link(value.confirmedAssessment,ordered[2]);
  link(value.transientAssessment,ordered[4]);
  for(const row of ordered.slice(0,2))if(row.result_kind!=='http_response'||row.http_status!==204)throw Error('Recovery streak must lead with two durable successes');
  for(const row of ordered.slice(2))if(row.result_kind!=='http_response'||row.http_status!==500)throw Error('Confirmation streak must rest on three durable failures');
  const monitor=monitors.filter(row=>row.id===value.monitorId);
  if(monitor.length!==1||monitor[0].target_url!=='https://example.com'||monitor[0].paused!==true)throw Error('Durable confirmation/recovery Monitor mismatch');
}
export function verifyTargetEditEvidence(capture, rows, monitors) {
  const uuid=value=>typeof value==='string'&&/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/.test(value)&&value!=='00000000-0000-0000-0000-000000000000';
  const closed=(value,keys)=>value!==null&&typeof value==='object'&&!Array.isArray(value)&&Object.keys(value).length===keys.length&&keys.every(key=>Object.hasOwn(value,key));
  const value=capture?.targetEdit;
  if(!closed(value,['monitorId','oldTarget','newTarget','claimAfterEditTarget','readsAfterRestart']) || !uuid(value.monitorId) || value.monitorId===capture.id || value.oldTarget!==capture.target)throw Error('Invalid target edit evidence');
  if(typeof value.newTarget!=='string'||value.newTarget.length===0||value.newTarget.length>88||value.newTarget===value.oldTarget)throw Error('Invalid target edit transition');
  if(value.claimAfterEditTarget!==value.newTarget)throw Error('Post-edit claim did not probe the edited target');
  const reads=closed(value.readsAfterRestart,['monitor'])?Object.values(value.readsAfterRestart):null;
  if(!reads||reads.length!==1||reads[0]!==200)throw Error('Restarted monitor read must be 200');
  if(!Array.isArray(rows)||rows.length===0||rows.some(row=>row.monitor_id!==value.monitorId))throw Error('Durable old-target evidence must remain linked to the edited Monitor');
  const monitor=monitors.filter(row=>row.id===value.monitorId);
  if(monitor.length!==1||monitor[0].target_url!==value.newTarget)throw Error('Durable edited target mismatch');
}
export function verifyDurableIncidentEvidence(capture, rows, incidents, state) {
  const uuid=value=>typeof value==='string'&&/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/.test(value)&&value!=='00000000-0000-0000-0000-000000000000';
  const closed=(value,keys)=>value!==null&&typeof value==='object'&&!Array.isArray(value)&&Object.keys(value).length===keys.length&&keys.every(key=>Object.hasOwn(value,key));
  const reason=value=>value==='unexpected_http_status'||value==='probe_failure';
  const value=capture?.durableIncident;
  if(!closed(value,['monitorId','openedIncident','resolvedIncident','incidentHistory','readsAfterRestart','availabilityAssessment','restarts']) || !uuid(value.monitorId) || value.monitorId===capture.id)throw Error('Invalid durable incident evidence');
  const opened=value.openedIncident;
  if(!closed(opened,['incidentId','openedAt','openingReason','openedByCheckId'])||!uuid(opened.incidentId)||!reason(opened.openingReason)||!uuid(opened.openedByCheckId))throw Error('Invalid opened incident evidence');
  instant(opened.openedAt);
  const resolved=value.resolvedIncident;
  if(!closed(resolved,['incidentId','openedAt','resolvedAt','openingReason','openedByCheckId','resolvedByCheckId'])||resolved.incidentId!==opened.incidentId||resolved.openedAt!==opened.openedAt||resolved.openingReason!==opened.openingReason||resolved.openedByCheckId!==opened.openedByCheckId||!uuid(resolved.resolvedByCheckId)||resolved.resolvedByCheckId===resolved.openedByCheckId)throw Error('Invalid resolved incident evidence');
  instant(resolved.resolvedAt);
  if(instant(resolved.resolvedAt)<instant(opened.openedAt))throw Error('Incident resolved before it opened');
  const history=value.incidentHistory;
  if(!closed(history,['ids','truncationSurvived'])||!Array.isArray(history.ids)||history.ids.length!==1||history.ids[0]!==opened.incidentId||history.truncationSurvived!==true)throw Error('Incident history must persist the single resolved window through truncation');
  const reads=value.readsAfterRestart;
  if(!closed(reads,['incidents','availability'])||reads.incidents!==200||reads.availability!==200)throw Error('Restarted incident and availability reads must be 200');
  const assessment=value.availabilityAssessment;
  if(!closed(assessment,['status','reason','checkId','completedAt'])||assessment.status!=='available'||assessment.reason!=='successful_response'||!uuid(assessment.checkId)||assessment.checkId!==resolved.resolvedByCheckId)throw Error('Availability must remain the separate evidence-derived assessment');
  instant(assessment.completedAt);
  if(!closed(value.restarts,['api']))throw Error('Missing durable incident restart checkpoints');
  const restart=value.restarts.api;
  if(!closed(restart,['beforeStartedAt','afterStartedAt']))throw Error('Invalid durable incident restart checkpoint');
  for(const time of Object.values(restart))if(typeof time!=='string'||!/^\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d(?:\.\d{1,9})?Z$/.test(time)||new Date(time).toISOString().slice(0,19)!==time.slice(0,19))throw Error('Invalid restart UTC time');
  if(instant(restart.afterStartedAt)<=instant(restart.beforeStartedAt))throw Error('Restart timestamp did not advance');
  if(!Array.isArray(rows))throw Error('Missing durable incident evidence rows');
  const evidence=rows.filter(row=>row.id===assessment.checkId&&row.monitor_id===value.monitorId);
  if(evidence.length!==1||evidence[0].result_kind!=='http_response'||evidence[0].http_status!==204||instant(evidence[0].completed_at)!==instant(assessment.completedAt))throw Error('Availability assessment is not linked to durable success evidence');
  const opening=rows.filter(row=>row.id===opened.openedByCheckId&&row.monitor_id===value.monitorId);
  if(opening.length!==1)throw Error('Incident opening is not linked to durable failure evidence');
  if(opened.openingReason==='unexpected_http_status'){
    const status=opening[0].http_status;
    if(opening[0].result_kind!=='http_response'||status===null||!(status<200||status>=300))throw Error('unexpected_http_status opening must rest on a non-2xx durable failure');
  }else if(!['dns_error','timeout','connect_error','tls_error','protocol_error'].includes(opening[0].result_kind))throw Error('probe_failure opening must rest on a durable probe failure');
  if(!Array.isArray(incidents)||incidents.length!==1)throw Error('Durable incident history must be exactly one resolved row');
  const incident=incidents[0];
  if(incident.id!==opened.incidentId||incident.monitor_id!==value.monitorId||incident.opening_reason!==opened.openingReason||instant(incident.opened_at)!==instant(opened.openedAt)||instant(incident.resolved_at)!==instant(resolved.resolvedAt)||incident.opened_by_check_id!==null||incident.resolved_by_check_id!==null)throw Error('Durable incident window must persist with evidence links cleared by truncation');
  if(!Array.isArray(state)||state.length!==1)throw Error('Durable incident state must be exactly one row');
  const fold=state[0];
  if(fold.monitor_id!==value.monitorId||fold.failure_streak!==0||fold.success_streak!==0||fold.open_incident_id!==null)throw Error('Resolved incident state must zero both streaks and clear the open pointer');
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
  verifyHistoryEvidence(JSON.parse(readFileSync(process.argv[2],"utf8")),JSON.parse(readFileSync(process.argv[7],"utf8")));
  verifyTargetEditEvidence(JSON.parse(readFileSync(process.argv[2],"utf8")),JSON.parse(readFileSync(process.argv[8],"utf8")),JSON.parse(readFileSync(process.argv[9],"utf8")));
  verifyConfirmationRecoveryEvidence(JSON.parse(readFileSync(process.argv[2],"utf8")),JSON.parse(readFileSync(process.argv[10],"utf8")),JSON.parse(readFileSync(process.argv[11],"utf8")));
  verifyDurableIncidentEvidence(JSON.parse(readFileSync(process.argv[2],"utf8")),JSON.parse(readFileSync(process.argv[12],"utf8")),JSON.parse(readFileSync(process.argv[13],"utf8")),JSON.parse(readFileSync(process.argv[14],"utf8")));
  console.log(
    "Independent browser snapshots, inventory IDs, scheduling, deletion, history, target-edit, confirmation/recovery and durable-incident evidence match durable rows.",
  );
}
