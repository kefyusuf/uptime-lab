import {readFileSync} from 'node:fs';
import {pathToFileURL} from 'node:url';
function instant(value){const match=/^(\d{4}-\d\d-\d\d)[T ](\d\d:\d\d:\d\d)(?:\.(\d{1,9}))?(Z|\+00(?::00)?)$/.exec(value);if(!match)throw new Error('Invalid UTC evidence time');return BigInt(Date.parse(`${match[1]}T${match[2]}Z`))*1000000n+BigInt((match[3]||'').padEnd(9,'0'));}
export function verifyEvidence(capture,rows) {
  if(!/^[0-9a-f-]{36}$/.test(capture.id)||capture.target!=='http://web/')throw new Error('Invalid browser monitor evidence');
  for(const card of ['raw','availability']) {
    const value=capture[card],row=rows.find(row=>row.id===value.checkId&&row.monitor_id===capture.id);
    if(!row||row.result_kind!=='policy_rejected'||row.http_status!==null||!Number.isInteger(row.duration_ms)||row.duration_ms<0||row.duration_ms>20000||(card==='raw'&&row.duration_ms!==value.durationMs)||instant(row.completed_at)!==instant(value.completedAt))throw new Error(`Durable ${card} evidence mismatch`);
  }
}
if(process.argv[1]&&import.meta.url===pathToFileURL(process.argv[1]).href){verifyEvidence(JSON.parse(readFileSync(process.argv[2],'utf8')),JSON.parse(readFileSync(process.argv[3],'utf8')));console.log('Both independent browser snapshots match durable terminal rows.');}
