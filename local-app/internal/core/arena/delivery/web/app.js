'use strict';
const $ = id => document.getElementById(id);
const api = '/api/v1/arena';
let view = null, operator = '', scope = '', sending = false, pc = null, mediaSession = '', generation = 0;
const db = new Promise((resolve, reject) => {
  const request = indexedDB.open('arena-offline-v1', 1);
  request.onupgradeneeded = () => request.result.createObjectStore('records', {keyPath:'id'});
  request.onsuccess = () => resolve(request.result);
  request.onerror = () => reject(request.error);
});
async function records(mode, action) {
  const database = await db;
  return new Promise((resolve, reject) => {
    const tx = database.transaction('records', mode); let result;
    const req = action(tx.objectStore('records'));
    if (req) req.onsuccess = () => { result = req.result; };
    tx.oncomplete = () => resolve(result);
    tx.onerror = tx.onabort = () => reject(tx.error || new Error('Penyimpanan perangkat gagal.'));
  });
}
const put = value => records('readwrite', store => store.put(value));
const get = key => records('readonly', store => store.get(key));
const remove = key => records('readwrite', store => store.delete(key));
const all = () => records('readonly', store => store.getAll());
function message(text) { $('message').textContent = text || ''; }
function node(tag, text, cls) { const n = document.createElement(tag); if (text !== undefined) n.textContent = text; if (cls) n.className = cls; return n; }
function id() { return crypto.randomUUID ? crypto.randomUUID() : Array.from(crypto.getRandomValues(new Uint32Array(4)), n => n.toString(16).padStart(8,'0')).join(''); }
function key(round, participant) { return `${scope}:${round}:${participant}`; }
async function request(path, options = {}, admin = false) {
  const headers = {'Content-Type':'application/json', ...(options.headers || {})};
  if (admin) headers.Authorization = 'Bearer ' + operator;
  let response;
  try { response = await fetch(api + path, {...options, headers, credentials:'same-origin', signal: options.signal || AbortSignal.timeout(10000)}); }
  catch (cause) { const e = new Error('Server arena belum dapat dijangkau. Draft tetap di perangkat.'); e.cause = cause; throw e; }
  if (!response.ok) {
    let body; try { body = await response.json(); } catch { body = {}; }
    const e = new Error(body.error || `HTTP ${response.status}`); e.status = response.status; throw e;
  }
  const text = await response.text(); return text ? JSON.parse(text) : null;
}
async function catalog() {
  const body = await request('/events'); const selected = $('events').value;
  $('events').replaceChildren();
  for (const event of body.data) { const option = node('option', event.name); option.value = event.id; $('events').append(option); }
  if (Array.from($('events').options).some(o => o.value === selected)) $('events').value = selected;
  $('event-switch').replaceChildren(...Array.from($('events').options).map(o=>o.cloneNode(true)));$('event-switch').value=$('events').value;
}
async function load() {
  if (operator) {
    await catalog();
    const event = $('events').value;
    view = event ? (await request('/operator/events/' + encodeURIComponent(event), {}, true)).data : null;
  } else { view = (await request('/me')).data; }
  scope = view ? `${view.id}:${view.judge_id || 'operator'}` : '';
  $('login').hidden = true; $('workspace').hidden = false; $('logout').hidden = false;
  $('operator-tools').hidden = !operator;
  $('event-name').textContent = view ? view.name : 'Belum ada paket event';
  await render(); await updateStatus();
}
async function render() {
  $('rounds').replaceChildren(); if (!view) return;
  for (const round of view.rounds) {
    const section = node('section', undefined, 'round');
    section.append(node('h2', `${round.name} — ${round.state}`));
    section.append(node('p', `${round.completed}/${round.required} nilai tersimpan di arena`, 'progress'));
    if (round.state === 'INTERRUPTED') section.append(node('p', 'Ronde terinterupsi. Tunggu operator melanjutkan; draft tetap dapat disimpan di perangkat.', 'warning'));
    if (operator) {
      const actions = node('div', undefined, 'toolbar');
      const allowed = round.state === 'PREPARED' ? ['start','cancel'] : round.state === 'ACTIVE' ? ['close','cancel'] : round.state === 'INTERRUPTED' ? ['resume','close','cancel'] : [];
      const names = {start:'Mulai', resume:'Lanjutkan', close:'Tutup resmi', cancel:'Batalkan'};
      for (const action of allowed) {
        const button = node('button', names[action]);
        button.onclick = async () => {
          const reason = action === 'cancel' ? prompt('Alasan pembatalan (wajib):') : '';
          if (action === 'cancel' && !reason) return;
          button.disabled = true;
          try {
            await request(`/operator/events/${encodeURIComponent(view.id)}/rounds/${encodeURIComponent(round.id)}/transition`, {method:'POST',body:JSON.stringify({action,reason})}, true);
            message('Status ronde tersimpan di arena.'); await load();
          } catch (e) { message(e.message); button.disabled = false; }
        };
        actions.append(button);
      }
      section.append(actions);
    }
    const cards = node('div', undefined, 'cards');
    for (const participant of [...round.participants].sort((a,b) => a.lapak - b.lapak)) {
      const card = node('article', undefined, 'card'); card.append(node('h3', `Lapak ${participant.lapak} · ${participant.name}`));
      const live = node('button', 'Lihat live');
      live.onclick = () => play(round.playback[participant.channel_id], participant.name).catch(e => { $('video-state').textContent = 'Media tidak tersedia: ' + e.message + '. Penilaian tetap dapat berjalan.'; });
      card.append(live);
      if (operator) {
        const result = round.results.find(r => r.participant_id === participant.id);
        if (result) card.append(node('p', `${round.state === 'CLOSED' ? 'Hasil' : 'Sementara'}: ${result.average.toFixed(3)} (${result.count}/${result.required})`));
      } else {
        const cell = key(round.id, participant.id), draft = await get('draft:' + cell), queued = await get('queue:' + cell);
		card.dataset.cell=cell;
        const accepted = round.scores.find(r => r.participant_id === participant.id);
        const values = draft ? draft.scores : accepted ? accepted.scores : {};
        const form = node('form'), inputs = {};
        const status = node('p', queued ? queued.blocked ? queued.error : 'Menunggu dikirim ke server arena' : draft ? 'Draft tersimpan di perangkat' : accepted ? `Tersimpan di server arena · revisi ${accepted.revision}` : 'Belum diisi', 'status');
        for (const criterion of view.criteria) {
          const label = node('label', criterion.name, 'score'), input = document.createElement('input');
          input.type = 'number'; input.min = '1'; input.max = '10'; input.step = '1'; input.required = true;
          input.value = values[criterion.id] ?? ''; input.disabled = ['CLOSED','CANCELLED'].includes(round.state);
          input.setAttribute('aria-label', criterion.name + ' ' + participant.name); inputs[criterion.id] = input;
          input.oninput = async () => {
            const scores = Object.fromEntries(Object.entries(inputs).map(([k,n]) => [k,n.value === '' ? '' : Number(n.value)]));
            try { await put({id:'draft:'+cell, scope, round:round.id, participant:participant.id, scores, editedAt:Date.now()}); status.textContent = 'Draft tersimpan di perangkat; belum diterima server arena.'; }
            catch { status.textContent = 'Gagal menyimpan draft perangkat. Jangan tutup halaman.'; }
          };
          label.append(input); form.append(label);
        }
        const submit = node('button', 'Kirim nilai ke arena'); submit.type = 'submit';
        submit.disabled = round.state !== 'ACTIVE' || !!(queued && !queued.blocked);
        form.onsubmit = async event => {
          event.preventDefault(); if (!form.reportValidity()) return;
          const existing = await get('queue:' + cell); if (existing && !existing.blocked) return;
          const scores = Object.fromEntries(Object.entries(inputs).map(([k,n]) => [k,Number(n.value)]));
          const latest=round.scores.find(r=>r.participant_id===participant.id);
          const pending = {id:'queue:'+cell, scope, round:round.id, editedAt:Date.now(), body:{event_id:view.id,judge_id:view.judge_id,operation_id:id(),participant_id:participant.id,expected_revision:latest ? latest.revision : 0,scores}};
          try {
            await put(pending); submit.disabled = true; status.textContent = 'Menunggu konfirmasi server arena…';
            await flush(); await load();
          } catch (e) { message(e.message); }
        };
        form.append(submit, status); card.append(form);
      }
      cards.append(card);
    }
    section.append(cards); $('rounds').append(section);
  }
}
async function flush() {
  if (sending || operator || !scope) return; sending = true;
  const activeScope=scope;
  try {
    for (const item of (await all()).filter(r => r.scope === activeScope && r.id.startsWith('queue:') && !r.blocked).sort((a,b) => a.editedAt-b.editedAt)) {
      if(scope!==activeScope||operator||!view)break;
      try {
        const result = await request(`/rounds/${encodeURIComponent(item.round)}/scores`, {method:'POST',body:JSON.stringify(item.body)});
        await remove(item.id);
        const draftID = item.id.replace('queue:','draft:'), draft = await get(draftID);
        if (draft && draft.editedAt <= item.editedAt) await remove(draftID);
        if(scope!==activeScope||operator||!view)break;
        const round = view.rounds.find(r => r.id === item.round);
        if (round) { round.scores = round.scores.filter(r => r.participant_id !== item.body.participant_id); round.scores.push(result.data); }
		const cell=item.id.slice('queue:'.length),card=Array.from(document.querySelectorAll('[data-cell]')).find(n=>n.dataset.cell===cell);
		if(card){card.querySelector('.status').textContent=draft&&draft.editedAt>item.editedAt?'Draft perangkat lebih baru; kirim untuk membuat revisi berikutnya.':`Tersimpan di server arena · revisi ${result.data.revision}`;card.querySelector('button[type="submit"]').disabled=round.state!=='ACTIVE';}
        message('Nilai tersimpan di server arena. Sinkronisasi cloud berjalan terpisah.');
      } catch (e) {
        if ([400,403,404,409].includes(e.status)) { item.blocked = true; item.error = e.message + ' Muat ulang dan periksa sebelum mengirim revisi baru.'; await put(item); message(item.error); }
        else { message(e.message); break; }
      }
    }
  } finally { sending = false; }
}
async function updateStatus() {
	const health=await request('/status');
	$('connection').textContent=health.media.state==='RUNNING'?'Arena siap · proses media berjalan':'Arena siap · media terganggu, penilaian tetap tersedia';
	if(!health.backup_configured||health.backup_failed) $('connection').textContent+=' · periksa backup';
  if (operator) { const result = await request('/operator/sync', {}, true); $('sync-state').textContent = `${result.pending_changes} perubahan menunggu acknowledgment cloud.`; }
  else if (scope) { const pending = (await all()).filter(r => r.scope === scope && r.id.startsWith('queue:')); $('sync-state').textContent = `${pending.length} kiriman perangkat belum terselesaikan. Nilai resmi selalu disimpan di server arena.`; }
  if(!operator&&view){const states=(await request('/me/status')).data;if(states.some(s=>view.rounds.find(r=>r.id===s.id)?.state!==s.state))await load();}
}
async function stopVideo() {
  generation++; if (pc) { pc.close(); pc = null; }
  $('video').srcObject = null; $('player').hidden = true;
  const session = mediaSession; mediaSession = '';
  if (session) { try { await fetch(session, {method:'DELETE',signal:AbortSignal.timeout(3000)}); } catch {} }
}
async function play(url, name) {
  await stopVideo(); const current = generation;
  if (!url || !window.RTCPeerConnection) throw new Error('WebRTC tidak tersedia');
  $('player').hidden = false; $('player-name').textContent = name; $('video-state').textContent = 'Menghubungkan video lokal…';
  const peer = new RTCPeerConnection({iceServers:[]}); pc = peer;
  peer.addTransceiver('video', {direction:'recvonly'}); peer.addTransceiver('audio', {direction:'recvonly'});
  const received=new MediaStream();
  peer.ontrack = event => { if (pc === peer) { if(!received.getTracks().some(t=>t.id===event.track.id))received.addTrack(event.track);$('video').srcObject = received; $('video-state').textContent = 'Live melalui jaringan lokal'; } };
  await peer.setLocalDescription(await peer.createOffer());
  if (peer.iceGatheringState !== 'complete') await new Promise(resolve => { const timeout = setTimeout(resolve, 3000); peer.onicegatheringstatechange = () => { if (peer.iceGatheringState === 'complete') { clearTimeout(timeout); resolve(); } }; });
  if (current !== generation) return;
  const answer = await fetch(url, {method:'POST',headers:{'Content-Type':'application/sdp'},body:peer.localDescription.sdp,signal:AbortSignal.timeout(10000)});
  if (!answer.ok) throw new Error('WHEP HTTP ' + answer.status);
  const location = answer.headers.get('Location'); if (location) mediaSession = new URL(location, url).href;
  if (current !== generation) { await stopVideo(); return; }
  await peer.setRemoteDescription({type:'answer',sdp:await answer.text()});
  $('player').scrollIntoView({behavior:'smooth'});
}
$('sign-in').onclick = async () => { scope='';try { await request('/login', {method:'POST',body:JSON.stringify({event_id:$('events').value,username:$('username').value,password:$('password').value})}); $('password').value=''; operator=''; await load(); } catch(e) { message(e.message); } };
$('operator-login').onclick = async () => { scope='';operator=$('operator-token').value; try { await request('/operator/sync',{},true); $('operator-token').value=''; await load(); } catch(e) { operator=''; message(e.message); } };
$('logout').onclick = async () => { scope='';view=null;operator='';try { await request('/logout',{method:'POST'}); } catch {} await stopVideo();$('workspace').hidden=true;$('login').hidden=false;$('logout').hidden=true;message('Keluar. Draft yang belum terkirim tetap tersimpan pada perangkat ini.'); };
$('refresh').onclick = () => load().catch(e=>message(e.message));
$('retry').onclick = async () => { try { await flush(); await load(); } catch(e) { message(e.message); } };
$('stop-video').onclick = stopVideo;
$('event-switch').onchange=()=>{$('events').value=$('event-switch').value;load().catch(e=>message(e.message));};
$('import').onclick = async () => { try { const file=$('bundle').files[0]; if (!file) throw new Error('Pilih paket event JSON.'); await request('/operator/events',{method:'POST',body:await file.text()},true); await load(); message('Paket event tersimpan lokal.'); } catch(e) { message(e.message); } };
$('backup').onclick = async () => { try { await request('/operator/backup',{method:'POST'},true); message('Snapshot database dibuat pada BACKUP_DIR.'); } catch(e) { message(e.message); } };
$('pull').onclick=async()=>{try{await request('/operator/packages/pull',{method:'POST',body:JSON.stringify({event_id:$('cloud-event').value})},true);await load();message('Paket cloud tersimpan di arena dan siap digunakan offline.');}catch(e){message(e.message);}};
$('history').onclick=async()=>{try{if(!view)throw new Error('Pilih event.');const data=await request('/operator/events/'+encodeURIComponent(view.id)+'/revisions',{},true);const url=URL.createObjectURL(new Blob([JSON.stringify(data,null,2)],{type:'application/json'}));const link=document.createElement('a');link.href=url;link.download='revisi-'+view.id+'.json';link.click();setTimeout(()=>URL.revokeObjectURL(url),1000);}catch(e){message(e.message);}};
window.addEventListener('online',()=>flush().catch(e=>message(e.message)));
window.addEventListener('pagehide',()=>{ if(pc)pc.close(); });
setInterval(async()=>{ try { await fetch('/healthz',{signal:AbortSignal.timeout(3000)}).then(r=>{if(!r.ok)throw new Error();}); $('connection').textContent='Server arena dapat diakses'; await flush(); if(!$('workspace').hidden)await updateStatus(); } catch { $('connection').textContent='Koneksi arena terputus · draft tetap di perangkat'; } },5000);
(async()=>{try{await db;await catalog();try{await load();}catch(e){if(e.status!==401)throw e;}}catch(e){message(e.message);}})();
if ('serviceWorker' in navigator) navigator.serviceWorker.register('/arena/sw.js', {scope:'/arena/'}).catch(()=>{});
