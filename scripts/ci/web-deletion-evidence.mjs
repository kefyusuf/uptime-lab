import { readFileSync, writeFileSync } from 'node:fs';
const [mode,file,...args]=process.argv.slice(2);
const capture=JSON.parse(readFileSync(file,'utf8'));
const uuid=/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;
if(!uuid.test(capture?.deletion?.monitorId??''))throw Error('Deletion fixture Monitor ID must be captured first');
const id=capture.deletion.monitorId;
if(mode==='sql-rows'){
  console.log(`SELECT coalesce(json_agg(row_to_json(r)),'[]'::json) FROM (SELECT id,monitor_id,completed_at,result_kind,http_status,duration_ms FROM monitoring.check_runs WHERE monitor_id='${id}') r; -- deletion-evidence-rows`);
}else if(mode==='sql-monitor'){
  console.log(`SELECT coalesce(json_agg(row_to_json(m)),'[]'::json) FROM (SELECT id,target_url,paused FROM monitoring.monitors WHERE id='${id}') m; -- deletion-evidence-monitor`);
}else if(mode==='record-claim'){
  const claim=JSON.parse(readFileSync(args[0],'utf8'));
  if(claim.monitorId!==id||!uuid.test(claim.checkId))throw Error('Internal claim does not match the deletion fixture');
  capture.deletion.checkRunsBeforeDelete=[claim.checkId];
}else if(mode==='restart'){
  const [service,phase,time]=args;if(!['api','checker'].includes(service)||!['before','after'].includes(phase))throw Error('Invalid restart checkpoint');
  capture.deletion.restarts??={};capture.deletion.restarts[service]??={};capture.deletion.restarts[service][phase+'StartedAt']=time;
}else if(mode==='record-reads'){
  const reads=JSON.parse(readFileSync(args[0],'utf8'));
  const expected={monitor:404,availability:404,latestResult:404,scheduling:404};
  if(JSON.stringify(reads)!==JSON.stringify(expected))throw Error('Deleted monitor reads are not uniformly 404');
  capture.deletion.readsAfterDelete=reads;
}else if(mode==='record-delivery'){
  const delivery=JSON.parse(readFileSync(args[0],'utf8'));
  if(delivery.attempts!==1||delivery.status!==404)throw Error('Delivery failure must be exactly one 404 not-found attempt');
  capture.deletion.deliveryFailures=delivery;
}else if(mode==='record-final'){
  const rows=JSON.parse(readFileSync(args[0],'utf8'));
  if(!Array.isArray(rows)||rows.length!==0)throw Error('Durable CheckRun rows survived deletion');
  capture.deletion.checkRunsAfterDelete=[];
}else throw Error('Unknown deletion evidence mode');
if(!mode.startsWith('sql-'))writeFileSync(file,JSON.stringify(capture));
