import { readFileSync, writeFileSync } from 'node:fs';
const [mode,file,...args]=process.argv.slice(2);
const capture=JSON.parse(readFileSync(file,'utf8'));
const uuid=/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;
if(mode!=='record-monitor'&&!uuid.test(capture?.confirmationRecovery?.monitorId??''))throw Error('Confirmation/recovery fixture Monitor ID must be captured first');
const id=capture.confirmationRecovery?.monitorId;
if(mode==='sql-rows'){
  console.log(`SELECT coalesce(json_agg(row_to_json(r)),'[]'::json) FROM (SELECT id, monitor_id, completed_at, result_kind, http_status, duration_ms FROM monitoring.check_runs WHERE monitor_id='${id}') r; -- confirmation-recovery-evidence-rows`);
}else if(mode==='sql-monitor'){
  console.log(`SELECT coalesce(json_agg(row_to_json(m)),'[]'::json) FROM (SELECT id, target_url, paused FROM monitoring.monitors WHERE id='${id}') m; -- confirmation-recovery-evidence-monitor`);
}else if(mode==='record-monitor'){
  const monitorId=String(args[0]??'').trim();
  if(!uuid.test(monitorId))throw Error('Confirmation/recovery fixture Monitor ID is not a UUID');
  capture.confirmationRecovery={monitorId};
}else if(mode==='restart'){
  const [service,phase,time]=args;if(!['api'].includes(service)||!['before','after'].includes(phase))throw Error('Invalid restart checkpoint');
  capture.confirmationRecovery.restarts??={};capture.confirmationRecovery.restarts[service]??={};capture.confirmationRecovery.restarts[service][phase+'StartedAt']=time;
}else if(mode==='record-reads'){
  const reads=JSON.parse(readFileSync(args[0],'utf8'));
  const recoveredCheckId=capture.confirmationRecovery.recoveredAssessment?.checkId;
  if(JSON.stringify(reads)!==JSON.stringify({availability:200,status:'available',reason:'successful_response',checkId:recoveredCheckId}))throw Error('Restarted availability read must serve the recovered assessment');
  capture.confirmationRecovery.readsAfterRestart=reads;
}else throw Error('Unknown confirmation/recovery evidence mode');
if(!mode.startsWith('sql-'))writeFileSync(file,JSON.stringify(capture));