import { readFileSync, writeFileSync } from 'node:fs';
const [mode,file,...args]=process.argv.slice(2);
const capture=JSON.parse(readFileSync(file,'utf8'));
const uuid=/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;
if(mode!=='record-monitor'&&!uuid.test(capture?.history?.monitorId??''))throw Error('History fixture Monitor ID must be captured first');
const id=capture.history?.monitorId;
if(mode==='sql-rows'){
  console.log(`SELECT coalesce(json_agg(row_to_json(r)),'[]'::json) FROM (SELECT id, completed_at FROM monitoring.check_runs WHERE monitor_id='${id}') r; -- history-evidence-rows`);
}else if(mode==='record-monitor'){
  const monitorId=String(args[0]??'').trim();
  if(!uuid.test(monitorId))throw Error('History fixture Monitor ID is not a UUID');
  capture.history={monitorId};
}else if(mode==='restart'){
  const [service,phase,time]=args;if(!['api'].includes(service)||!['before','after'].includes(phase))throw Error('Invalid restart checkpoint');
  capture.history.restarts??={};capture.history.restarts[service]??={};capture.history.restarts[service][phase+'StartedAt']=time;
}else if(mode==='record-reads'){
  const reads=JSON.parse(readFileSync(args[0],'utf8'));
  if(JSON.stringify(reads)!==JSON.stringify({history:200}))throw Error('Restarted history read must be 200');
  capture.history.readsAfterRestart=reads;
}else throw Error('Unknown history evidence mode');
if(!mode.startsWith('sql-'))writeFileSync(file,JSON.stringify(capture));
