// Integration test against two real binaries and two persistent databases.
const fs=require('node:fs'),path=require('node:path'),http=require('node:http'),crypto=require('node:crypto'),assert=require('node:assert/strict');
const {spawn}=require('node:child_process');
const root=path.resolve(__dirname,'../..'),dir=path.join(root,'runtime/browser-check/exchange-'+Date.now());fs.mkdirSync(dir,{recursive:true});
const operator=crypto.randomBytes(32).toString('hex'),machine=crypto.randomBytes(32).toString('hex'),admin=crypto.randomBytes(32).toString('hex');
const bundle=JSON.parse(fs.readFileSync(path.join(root,'runtime/browser-check/demo/event.json')));
const credentials=fs.readFileSync(path.join(root,'runtime/browser-check/demo/judge-credentials.csv'),'utf8').trim().split(/\r?\n/)[1].split(',');
let local,cloud,localBase,cloudBase,localEnv,cloudEnv,cookie='',log='';
const pause=ms=>new Promise(r=>setTimeout(r,ms));
async function wait(fn,label){const end=Date.now()+25000;while(Date.now()<end){try{if(await fn())return;}catch{}await pause(100);}throw new Error('Timed out: '+label);}
async function port(){const s=http.createServer();await new Promise(r=>s.listen(0,'127.0.0.1',r));const n=s.address().port;await new Promise(r=>s.close(r));return n;}
async function start(binary,env,base){const child=spawn(path.join(root,'runtime/browser-check',binary),['-env',env],{cwd:root,stdio:['ignore','pipe','pipe'],windowsHide:true});for(const stream of [child.stdout,child.stderr])stream.on('data',b=>log=(log+b).slice(-10000));await wait(async()=>{if(child.exitCode!==null)throw new Error('process exited');return(await fetch(base+'/healthz')).ok;},binary);return child;}
async function stop(child){if(!child||child.exitCode!==null)return;await new Promise(resolve=>{child.once('exit',resolve);child.kill();});}
async function request(base,route,token,body,expected=200){const res=await fetch(base+route,{method:body===undefined?'GET':'POST',headers:{'Content-Type':'application/json',...(token?{Authorization:'Bearer '+token}:{}),...(cookie&&base===localBase?{Cookie:cookie}:{})},body:body===undefined?undefined:JSON.stringify(body)});assert.equal(res.status,expected,route+' HTTP '+res.status);if(route.endsWith('/login'))cookie=res.headers.get('set-cookie').split(';')[0];const text=await res.text();return text?JSON.parse(text):null;}
(async()=>{
  cloudBase='http://127.0.0.1:'+await port();localBase='http://127.0.0.1:'+await port();
  const normalize=p=>p.replaceAll('\\','/');cloudEnv=path.join(dir,'cloud.env');localEnv=path.join(dir,'local.env');
  fs.writeFileSync(cloudEnv,[`HTTP_ADDR=${cloudBase.slice(7)}`,`CLOUD_DATABASE='${normalize(path.join(dir,'cloud.sqlite'))}'`,`CLOUD_ADMIN_TOKEN=${admin}`,`CLOUD_ENGINES_JSON='${JSON.stringify([{arena_id:'arena01',engine_id:'engine01',token:machine}])}'`].join('\n'));
  fs.writeFileSync(localEnv,[`HTTP_ADDR=${localBase.slice(7)}`,`API_TOKEN=${operator}`,`DATA_DIR='${normalize(path.join(dir,'data'))}'`,`RUNTIME_DIR='${normalize(path.join(dir,'runtime'))}'`,'FFMPEG_BIN=deliberately-missing-ffmpeg','MEDIAMTX_BIN=deliberately-missing-mediamtx',`CHANNELS_JSON='${JSON.stringify([{id:'lapak01',source:'rtsp://127.0.0.1:29999/source'}])}'`,`CLOUD_SYNC_URL=${cloudBase}/api/v1/engines/engine01/changes`,`CLOUD_COMMANDS_URL=${cloudBase}/api/v1/engines/engine01/commands`,`CLOUD_PACKAGES_URL=${cloudBase}/api/v1/engines/engine01/events`,`CLOUD_ENGINE_TOKEN=${machine}`].join('\n'));
  cloud=await start('cloud.exe',cloudEnv,cloudBase);local=await start('arena.exe',localEnv,localBase);
  await request(cloudBase,'/api/v1/operator/packages',admin,bundle,201);
  await request(localBase,'/api/v1/arena/operator/packages/pull',operator,{event_id:'demo01'},201);
  await request(localBase,'/api/v1/arena/operator/events/demo01/rounds/ronde01/transition',operator,{action:'start'},204);
  await request(localBase,'/api/v1/arena/login','',{event_id:'demo01',username:credentials[1],password:credentials[2]});
  await request(localBase,'/api/v1/arena/rounds/ronde01/scores','',{operation_id:'score-one',participant_id:'peserta-01-01',expected_revision:0,scores:{c1:8,c2:9}});
  await wait(async()=>(await request(localBase,'/api/v1/arena/operator/sync',operator)).pending_changes===0,'durable cloud acknowledgment');
  console.log('PASS prepared package download and durable changes between real local/cloud services.');
  const command={id:'stop-test',arena_id:'arena01',engine_id:'engine01',kind:'restream.stop',payload:{target:'youtube'},expires_at:new Date(Date.now()+120000).toISOString()};
  await request(cloudBase,'/api/v1/operator/commands',admin,command,202);
  await wait(async()=>(await request(cloudBase,'/api/v1/operator/commands/engine01/stop-test',admin)).state==='SUCCEEDED','media command execution result');
  await request(cloudBase,'/api/v1/operator/commands',admin,{...command,id:'forbidden',kind:'judging.close'},400);
  console.log('PASS media command execution acknowledged; remote judging mutation rejected.');
  await stop(cloud);cloud=null;
  const submittedAt=Date.now();
  await request(localBase,'/api/v1/arena/rounds/ronde01/scores','',{operation_id:'score-offline',participant_id:'peserta-01-02',expected_revision:0,scores:{c1:7,c2:10}});
  assert.ok(Date.now()-submittedAt<1500,'scoring waited on cloud');
  await request(localBase,'/api/v1/arena/operator/packages/pull',operator,{event_id:'demo01'},503);
  const scores=(await request(localBase,'/api/v1/arena/operator/events/demo01',operator)).data.rounds[0].scores;assert.equal(scores.length,2);
  assert.ok((await request(localBase,'/api/v1/arena/operator/sync',operator)).pending_changes>0);
  cloud=await start('cloud.exe',cloudEnv,cloudBase);
  await wait(async()=>(await request(localBase,'/api/v1/arena/operator/sync',operator)).pending_changes===0,'synchronization after cloud restart');
  assert.equal((await request(cloudBase,'/api/v1/operator/commands/engine01/stop-test',admin)).state,'SUCCEEDED');
  await request(cloudBase,'/api/v1/operator/packages',admin,{...bundle,version:2},201);
  await request(localBase,'/api/v1/arena/operator/packages/pull',operator,{event_id:'demo01'},409);
  console.log('PASS cloud outage does not delay scoring; reconnect syncs; active event roster stays frozen.');
})().catch(e=>{console.error(e.message);console.error(log);process.exitCode=1;}).finally(async()=>{await stop(local);await stop(cloud);});
