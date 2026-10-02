import {test} from 'node:test';import assert from 'node:assert/strict';import {verifyEvidence} from './verify-web-evidence.mjs';
const id='aa29443e-c597-4e7b-9202-a4762e0e04c0',a='bb29443e-c597-4e7b-9202-a4762e0e04c0',b='cc29443e-c597-4e7b-9202-a4762e0e04c0';
const capture={id,target:'http://web/',raw:{checkId:a,completedAt:'2026-10-02T12:00:00.123456Z',durationMs:0},availability:{checkId:b,completedAt:'2026-10-02T12:00:01Z'}};
const rows=[a,b].map((id,index)=>({id,monitor_id:capture.id,result_kind:'policy_rejected',http_status:null,duration_ms:0,completed_at:index?'2026-10-02T12:00:01+00:00':'2026-10-02T12:00:00.123456+00:00'}));
test('independent captured rows remain valid after newer completion',()=>assert.doesNotThrow(()=>verifyEvidence(capture,[...rows,{...rows[1],id:'later'}])));
for(const field of ['monitor_id','result_kind','completed_at','http_status','duration_ms'])test(`rejects mismatched ${field}`,()=>{const mutated=structuredClone(rows);mutated[0][field]=field==='http_status'?200:field==='duration_ms'?null:'wrong';assert.throws(()=>verifyEvidence(capture,mutated));});
