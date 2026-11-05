// Run after building runtime/browser-check/arena.exe and generating its demo package.
// Install Playwright under runtime/browser-check; no dependency is added to the app.
const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const crypto = require('node:crypto');
const {spawn} = require('node:child_process');
const assert = require('node:assert/strict');
const root = path.resolve(__dirname, '../..');
const {chromium} = require(path.join(root, 'runtime/browser-check/node_modules/playwright'));
const scratch = path.join(root, 'runtime/browser-check/run-' + Date.now());
fs.mkdirSync(scratch, {recursive:true});
const token = crypto.randomBytes(32).toString('hex');
const cloudToken = crypto.randomBytes(32).toString('hex');
const bundle = JSON.parse(fs.readFileSync(path.join(root,'runtime/browser-check/demo/event.json')));
const credentials = fs.readFileSync(path.join(root,'runtime/browser-check/demo/judge-credentials.csv'),'utf8').trim().split(/\r?\n/).slice(1).map(line=>line.split(','));
let child, browser, cloudOnline = false, log = '';
const delivered = new Map();
const cloud = http.createServer((req,res)=>{
  let body='';req.on('data',b=>body+=b);req.on('end',()=>{
    if(!cloudOnline){res.writeHead(503);return res.end();}
    assert.equal(req.headers.authorization,'Bearer '+cloudToken);
    const changes=JSON.parse(body).changes;
    for(const change of changes){assert.ok(!JSON.stringify(change).includes('password_hash'));delivered.set(change.id,change);}
    res.setHeader('Content-Type','application/json');res.end(JSON.stringify({acknowledged:changes.map(c=>c.id)}));
  });
});
const pause = ms => new Promise(r=>setTimeout(r,ms));
async function waitFor(fn,label,timeout=20000){const end=Date.now()+timeout;while(Date.now()<end){try{if(await fn())return;}catch{}await pause(100);}throw new Error('Timed out: '+label);}
async function freePort(){const server=http.createServer();await new Promise(r=>server.listen(0,'127.0.0.1',r));const port=server.address().port;await new Promise(r=>server.close(r));return port;}
let base, envPath;
async function start(){
  child=spawn(path.join(root,'runtime/browser-check/arena.exe'),['-env',envPath],{cwd:root,stdio:['ignore','pipe','pipe'],windowsHide:true});
  for(const stream of [child.stdout,child.stderr])stream.on('data',b=>log=(log+b.toString()).slice(-12000));
  await waitFor(async()=>{if(child.exitCode!==null)throw new Error('server exited');return (await fetch(base+'/healthz')).ok;},'arena startup');
}
async function stop(){if(!child)return;const owned=child;child=null;if(owned.exitCode!==null)return;await new Promise(resolve=>{owned.once('exit',resolve);owned.kill();});}
async function admin(route,method='GET',body){const res=await fetch(base+'/api/v1/arena/operator'+route,{method,headers:{Authorization:'Bearer '+token,'Content-Type':'application/json'},body:body===undefined?undefined:JSON.stringify(body)});if(!res.ok)throw new Error('Operator HTTP '+res.status+' '+await res.text());const text=await res.text();return text?JSON.parse(text):null;}
async function login(page,index){await page.goto(base+'/arena/');await page.waitForSelector('option[value="demo01"]',{state:'attached'});await page.fill('#username',credentials[index][1]);await page.fill('#password',credentials[index][2]);await page.click('#sign-in');await page.waitForSelector('.card');assert.equal(await page.locator('.card').count(),24);}
(async()=>{
  await new Promise(r=>cloud.listen(0,'127.0.0.1',r));
  base='http://127.0.0.1:'+await freePort();
  const normalize=p=>p.replaceAll('\\','/');
  envPath=path.join(scratch,'test.env');
  fs.writeFileSync(envPath,[`HTTP_ADDR=${base.replace('http://','')}`,`API_TOKEN=${token}`,`DATA_DIR='${normalize(path.join(scratch,'data'))}'`,`RUNTIME_DIR='${normalize(path.join(scratch,'runtime'))}'`,`BACKUP_DIR='${normalize(path.join(scratch,'backup'))}'`,"FFMPEG_BIN=deliberately-missing-ffmpeg","MEDIAMTX_BIN=deliberately-missing-mediamtx",`CHANNELS_JSON='${JSON.stringify([{id:'lapak01',source:'rtsp://127.0.0.1:29999/source'}])}'`,`CLOUD_SYNC_URL=http://127.0.0.1:${cloud.address().port}/changes`,`CLOUD_ENGINE_TOKEN=${cloudToken}`,`WEBRTC_PUBLIC_URL=http://127.0.0.1:29998`].join('\n'));
  await start();await admin('/events','POST',bundle);await admin('/events/demo01/rounds/ronde01/transition','POST',{action:'start'});
  const edge=process.env.BROWSER_BIN||'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe';
  browser=await chromium.launch({executablePath:edge,headless:true,args:['--disable-gpu']});
  const context=await browser.newContext();
  // No public website, CDN, cloud login, or previous browser cache is available.
  await context.route('**/*',route=>{const host=new URL(route.request().url()).hostname;return ['127.0.0.1','localhost'].includes(host)?route.continue():route.abort();});
  const page=await context.newPage();const errors=[];page.on('pageerror',e=>errors.push(e.message));
  await login(page,0);
  let card=page.locator('.card').nth(0);
  await card.locator('input').nth(0).fill('8');await card.locator('input').nth(1).fill('6');await card.locator('button[type="submit"]').click();
  await waitFor(async()=>((await admin('/events/demo01')).data.rounds[0].scores.length===1),'first committed score');
  await page.click('#refresh');await waitFor(()=>page.locator('.card').nth(0).locator('.status').innerText().then(t=>t.includes('revisi 1')),'stored state');
  card=page.locator('.card').nth(0);await card.locator('button').first().click();
  await waitFor(()=>page.locator('#video-state').innerText().then(t=>t.includes('Penilaian tetap')),'media failure warning');
  await page.click('#stop-video');
  card=page.locator('.card').nth(0);await card.locator('input').nth(0).fill('9');await card.locator('input').nth(1).fill('9');await card.locator('button[type="submit"]').click();
  await waitFor(async()=>((await admin('/events/demo01')).data.rounds[0].scores[0].revision===2),'revision while media is unavailable');
  console.log('PASS fresh offline judge login; 24 cards; scoring and revision with missing media/cloud.');
  await page.click('#refresh');await waitFor(()=>page.locator('.card').nth(0).locator('.status').innerText().then(t=>t.includes('revisi 2')),'revision displayed');
  await context.route('**/api/v1/arena/**',route=>route.abort());
  card=page.locator('.card').nth(1);await card.locator('input').nth(0).fill('7');await card.locator('input').nth(1).fill('7');await card.locator('button[type="submit"]').click();
  await waitFor(()=>page.evaluate(()=>new Promise(resolve=>{const req=indexedDB.open('arena-offline-v1');req.onsuccess=()=>{const tx=req.result.transaction('records');const all=tx.objectStore('records').getAll();all.onsuccess=()=>resolve(all.result.some(r=>r.id.startsWith('queue:')));};})),'durable device queue');
  assert.equal((await admin('/events/demo01')).data.rounds[0].scores.length,1);
  await context.unroute('**/api/v1/arena/**');await page.click('#retry');
  await waitFor(async()=>((await admin('/events/demo01')).data.rounds[0].scores.length===2),'device queue delivered after reconnection');
  console.log('PASS transient LAN request failure preserves queued submission and retries.');
  await stop();await start();await page.click('#refresh');
  await waitFor(()=>page.locator('.round h2').innerText().then(t=>t.includes('INTERRUPTED')),'interrupted round after restart');
  assert.equal((await admin('/events/demo01')).data.rounds[0].scores.length,2);
  assert.equal(await page.locator('.card button[type="submit"]').first().isDisabled(),true);
  await admin('/events/demo01/rounds/ronde01/transition','POST',{action:'resume'});
  await waitFor(()=>page.locator('.round h2').innerText().then(t=>t.includes('ACTIVE')),'judge sees operator resume without refreshing');
  assert.equal(await page.locator('.card button[type="submit"]').first().isDisabled(),false);
  const other=await browser.newContext();const otherPage=await other.newPage();await login(otherPage,1);
  assert.equal(await otherPage.locator('.status').filter({hasText:'Belum diisi'}).count(),24);
  await other.close();
  console.log('PASS restart preserves scores and sessions; fresh second device logs in while cloud is down.');
  await admin('/backup','POST');assert.ok(fs.readdirSync(path.join(scratch,'backup')).some(n=>n.endsWith('.sqlite')));
  cloudOnline=true;await waitFor(async()=>((await admin('/sync')).pending_changes===0),'background cloud acknowledgment',25000);
  assert.ok(delivered.size>=6);assert.equal(errors.length,0,errors.join('\n'));
  console.log('PASS consistent backup and durable cloud synchronization after reconnect.');
})().catch(e=>{console.error(e.message);console.error(log);process.exitCode=1;}).finally(async()=>{if(browser)await browser.close();await stop();await new Promise(r=>cloud.close(r));});

