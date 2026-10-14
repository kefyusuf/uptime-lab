import { readFileSync, writeFileSync } from 'node:fs';
const [mode,file,...args]=process.argv.slice(2);
const capture=JSON.parse(readFileSync(file,'utf8'));
const uuid=/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;
if(!uuid.test(capture.id))throw Error('Invalid captured Monitor ID');
const id=capture.id;
if(mode==='sql-prepare'){
  // Isolate claim opportunities without changing historical terminal evidence.
  console.log(`UPDATE monitoring.monitors SET paused=true WHERE id<>'${id}';`);
}else if(mode==='sql-snapshot'){
  console.log(`SELECT json_build_object('paused',m.paused,'checkIds',coalesce((SELECT json_agg(id ORDER BY id) FROM monitoring.check_runs WHERE monitor_id=m.id),'[]'::json),'pending',(SELECT count(*) FROM monitoring.check_runs WHERE monitor_id=m.id AND completed_at IS NULL),'due',(SELECT max(completed_at)<=now()-interval '60 seconds' FROM monitoring.check_runs WHERE monitor_id=m.id)) FROM monitoring.monitors m WHERE id='${id}';`);
}else if(mode==='sql-rows'){
  console.log(`SELECT coalesce(json_agg(row_to_json(r)),'[]'::json) FROM (SELECT id,monitor_id,completed_at,result_kind,http_status,duration_ms FROM monitoring.check_runs WHERE monitor_id='${id}') r;`);
}else if(mode==='record-snapshot'){
  const [phase,input]=args,snapshot=JSON.parse(readFileSync(input,'utf8'));
  if(snapshot.paused!==true||snapshot.pending!==0||!Array.isArray(snapshot.checkIds)||!snapshot.checkIds.length||!snapshot.checkIds.every(check=>uuid.test(check)))throw Error('Paused scheduling fixture or snapshot invalid');
  capture.scheduling[phase==='before'?'checkIdsBeforePause':'checkIdsWhilePaused']=snapshot.checkIds;
  if(phase==='during'&&JSON.stringify(capture.scheduling.checkIdsBeforePause)!==JSON.stringify(snapshot.checkIds))throw Error('New CheckRun appeared while paused');
}else if(mode==='restart'){
  const [service,phase,time]=args;if(!['api','checker'].includes(service)||!['before','after'].includes(phase))throw Error('Invalid restart checkpoint');
  capture.scheduling.restarts??={};capture.scheduling.restarts[service]??={};capture.scheduling.restarts[service][phase+'StartedAt']=time;
}else if(mode==='record-resume'){
  const snapshot=JSON.parse(readFileSync(args[0],'utf8'));
  if(snapshot.paused!==false)throw Error('Resume did not persist');
  const inserted=snapshot.checkIds.filter(check=>!capture.scheduling.checkIdsBeforePause.includes(check));
  if(inserted.length!==1||!uuid.test(inserted[0]))throw Error('Exactly one new resumed claim is required');
  capture.scheduling.checkIdAfterResume=inserted[0];
}else throw Error('Unknown scheduling evidence mode');
if(!mode.startsWith('sql-'))writeFileSync(file,JSON.stringify(capture));
