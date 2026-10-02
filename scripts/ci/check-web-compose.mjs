import {readFileSync} from 'node:fs';
import {pathToFileURL} from 'node:url';
export function checkWebCompose(canonical,local,port) {
  const errors=[];
  if(!Number.isInteger(port)||port<1024||port>65535)errors.push('Invalid browser port');
  for(const [name,config] of [['canonical',canonical],['local',local]]) {
    const services=config?.services;
    if(!services||Object.keys(services).sort().join(',')!=='api,checker,db,web'){errors.push(`${name}: exact service set required`);continue;}
    for(const [service,value] of Object.entries(services)) {
      if(value.network_mode==='host'||value.container_name)errors.push(`${name}: shared host resources forbidden`);
      if((name==='canonical'||service!=='web')&&value.ports?.length)errors.push(`${name}: unexpected published port`);
    }
    const web=services.web;
    if(web.build?.dockerfile!=='apps/web/Dockerfile'||web.read_only!==true||web.init!==true||web.depends_on)errors.push(`${name}: invalid Web ownership/runtime`);
    if(String(web.environment?.UPTIME_LAB_WEB_PORT)!==String(port))errors.push(`${name}: origin port diverges`);
    if(name==='local') {
      const ports=web.ports;
      if(ports?.length!==1||ports[0].host_ip!=='127.0.0.1'||String(ports[0].published)!==String(port)||ports[0].target!==8080||ports[0].protocol!=='tcp')errors.push('local: exactly one loopback Web mapping required');
    }
  }
  return errors;
}
if(process.argv[1]&&import.meta.url===pathToFileURL(process.argv[1]).href) {
  try {const errors=checkWebCompose(JSON.parse(readFileSync(process.argv[2],'utf8')),JSON.parse(readFileSync(process.argv[3],'utf8')),Number(process.argv[4]));if(errors.length)throw new Error(errors.join('\n'));console.log('Resolved Web Compose invariants passed.');}
  catch(error){console.error(error.message);process.exitCode=1;}
}
