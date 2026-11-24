import { readFileSync, writeFileSync } from 'node:fs';
const [mode,file,...args]=process.argv.slice(2);
const capture=JSON.parse(readFileSync(file,'utf8'));
const uuid=/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;
if(!uuid.test(capture?.durableIncident?.monitorId??''))throw Error('Durable incident fixture Monitor ID must be captured first');
const id=capture.durableIncident.monitorId;
if(mode==='sql-rows'){
  console.log(`SELECT coalesce(json_agg(row_to_json(r)),'[]'::json) FROM (SELECT id,monitor_id,completed_at,result_kind,http_status FROM monitoring.check_runs WHERE monitor_id='${id}') r; -- incident-evidence-rows`);
}else if(mode==='sql-incidents'){
  console.log(`SELECT coalesce(json_agg(row_to_json(i)),'[]'::json) FROM (SELECT id,monitor_id,opened_at,resolved_at,opening_reason,opened_by_check_id,resolved_by_check_id FROM monitoring.incidents WHERE monitor_id='${id}') i; -- incident-evidence-incidents`);
}else if(mode==='sql-state'){
  console.log(`SELECT coalesce(json_agg(row_to_json(s)),'[]'::json) FROM (SELECT monitor_id,failure_streak,success_streak,open_incident_id FROM monitoring.incident_state WHERE monitor_id='${id}') s; -- incident-evidence-state`);
}else if(mode==='record-claim'){
  const claim=JSON.parse(readFileSync(args[0],'utf8'));
  if(claim.monitorId!==id||!uuid.test(claim.checkId))throw Error('Internal claim does not match the durable incident fixture');
}else if(mode==='restart'){
  const [service,phase,time]=args;if(!['api'].includes(service)||!['before','after'].includes(phase))throw Error('Invalid restart checkpoint');
  capture.durableIncident.restarts??={};capture.durableIncident.restarts[service]??={};capture.durableIncident.restarts[service][phase+'StartedAt']=time;
}else if(mode==='record-reads'){
  const reads=JSON.parse(readFileSync(args[0],'utf8'));
  if(JSON.stringify(reads)!==JSON.stringify({incidents:200,availability:200}))throw Error('Restarted incident and availability reads must be 200');
  capture.durableIncident.readsAfterRestart=reads;
}else throw Error('Unknown durable incident evidence mode');
if(!mode.startsWith('sql-'))writeFileSync(file,JSON.stringify(capture));