import { readFileSync, writeFileSync } from 'node:fs';
const [mode,file,...args]=process.argv.slice(2);
const capture=JSON.parse(readFileSync(file,'utf8'));
const uuid=/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;
if(mode!=='sql-rows'&&mode!=='sql-monitor'&&!uuid.test(capture?.targetEdit?.monitorId??''))throw Error('Target edit fixture Monitor ID must be captured first');
if(mode==='sql-rows'){
  console.log(`SELECT coalesce(json_agg(row_to_json(r)),'[]'::json) FROM (SELECT id, monitor_id, completed_at FROM monitoring.check_runs WHERE monitor_id='${capture.targetEdit.monitorId}') r; -- target-edit-evidence-rows`);
}else if(mode==='sql-monitor'){
  console.log(`SELECT coalesce(json_agg(row_to_json(m)),'[]'::json) FROM (SELECT id, target_url FROM monitoring.monitors WHERE id='${capture.targetEdit.monitorId}') m; -- target-edit-evidence-monitor`);
}else if(mode==='record-claim'){
  const value=capture.targetEdit;
  if(typeof value.newTarget!=='string')throw Error('Edited target must be captured before the post-edit claim');
  const claim=JSON.parse(readFileSync(args[0],'utf8'));
  if(claim.monitorId!==value.monitorId||claim.targetUrl!==value.newTarget)throw Error('Post-edit claim must probe the edited target');
  value.claimAfterEditTarget=claim.targetUrl;
}else if(mode==='record-reads'){
  const value=capture.targetEdit;
  const reads=JSON.parse(readFileSync(args[0],'utf8'));
  if(reads.monitor!==200||reads.targetUrl!==value.newTarget)throw Error('Restarted monitor read must serve the edited target');
  value.readsAfterRestart={monitor:200};
}else throw Error('Unknown target edit evidence mode');
if(!mode.startsWith('sql-'))writeFileSync(file,JSON.stringify(capture));
