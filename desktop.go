package sidecar

const desktopHTML = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width,initial-scale=1">
  <title>YouTube Sidecar</title>
  <style>
    :root{color-scheme:dark;--bg:#10131a;--panel:#191e28;--panel2:#202735;--line:#30394a;--text:#f3f5f8;--muted:#a6b0c0;--accent:#ff4055;--green:#53d49a;--yellow:#f6c660}
    *{box-sizing:border-box}body{margin:0;background:radial-gradient(ellipse at 80% -10%,#392332 0,transparent 42%),var(--bg);color:var(--text);font:15px/1.5 system-ui,-apple-system,"Segoe UI",sans-serif}
    main{width:min(940px,calc(100% - 32px));margin:48px auto 72px}.brand{display:flex;align-items:center;gap:13px}.mark{display:grid;place-items:center;width:46px;height:46px;border-radius:15px;background:var(--accent);font-size:20px;font-weight:800}.eyebrow{color:#f48a99;text-transform:uppercase;letter-spacing:.14em;font-size:11px;font-weight:700}.brand h1{font-size:28px;line-height:1.15;margin:1px 0}.sub{color:var(--muted);margin:19px 0 27px;max-width:690px}.panel{background:linear-gradient(145deg,#1d2330,#171c25);border:1px solid var(--line);border-radius:17px;padding:22px;margin:14px 0;box-shadow:0 14px 45px #0002}.row{display:flex;justify-content:space-between;align-items:flex-start;gap:18px}.step{color:#fb8795;font-size:12px;font-weight:800;letter-spacing:.1em;text-transform:uppercase}.panel h2{font-size:18px;margin:3px 0 5px}.help{color:var(--muted);margin:2px 0 17px;max-width:730px}.controls{display:flex;gap:10px;align-items:center;flex-wrap:wrap}input,select{background:#111620;color:var(--text);border:1px solid #414b5e;border-radius:9px;padding:11px 12px;font:inherit;min-height:43px}input:focus,select:focus{outline:2px solid #ff405588;border-color:#ff697a}input.url{flex:1;min-width:250px}input.name{width:210px}button,.button{display:inline-flex;align-items:center;justify-content:center;gap:7px;border:0;border-radius:9px;background:var(--accent);color:#fff;padding:10px 15px;font:700 14px system-ui;cursor:pointer;text-decoration:none;min-height:42px}button.secondary,.button.secondary{background:#30394a;color:var(--text)}button.ghost{background:transparent;border:1px solid var(--line);color:var(--muted)}button:disabled{opacity:.5;cursor:wait}.status{display:inline-flex;align-items:center;gap:7px;border:1px solid var(--line);border-radius:999px;padding:6px 10px;color:var(--muted);font-size:12px;font-weight:650}.dot{width:8px;height:8px;border-radius:50%;background:#798395}.status.good{color:#b8f2d8;border-color:#35644f}.status.good .dot{background:var(--green)}.status.warn{color:#f8dfa2;border-color:#66532b}.status.warn .dot{background:var(--yellow)}.message{min-height:22px;margin-top:12px;color:var(--muted);font-size:13px}.message.error{color:#ff9ba6}.message.good{color:#82e2b8}.station{display:grid;grid-template-columns:minmax(0,1fr) auto;gap:14px;padding:15px 0;border-top:1px solid var(--line)}.station:first-child{border-top:0}.station-title{font-weight:700;overflow-wrap:anywhere}.station-id{color:var(--muted);font-size:12px;font-family:ui-monospace,monospace}.station-actions{display:flex;gap:8px;align-items:center;flex-wrap:wrap;justify-content:flex-end}.small{font-size:12px;padding:7px 10px;min-height:34px}.callout{background:#111620;border:1px solid #30394a;border-radius:10px;padding:12px 14px;color:var(--muted);font-size:13px}.api{font-family:ui-monospace,monospace;color:#ffd3d8;word-break:break-all}.footer{color:#7f8a9b;font-size:12px;margin-top:23px}.toggle{display:flex;align-items:center;gap:11px}.toggle input{accent-color:var(--accent);width:18px;height:18px;min-height:0}.label{display:block;color:var(--muted);font-size:12px;font-weight:650;margin:0 0 5px}.grid{display:grid;grid-template-columns:minmax(0,1fr) minmax(190px,260px);gap:14px;align-items:end}@media(max-width:650px){main{margin-top:28px}.panel{padding:17px}.row{display:block}.row .status{margin-top:10px}.grid{grid-template-columns:1fr}.station{grid-template-columns:1fr}.station-actions{justify-content:flex-start}input.name{width:100%}}
  </style>
</head>
<body>
<main>
  <header class="brand"><div class="mark">▶</div><div><div class="eyebrow">Desktop sidecar</div><h1>YouTube Sidecar</h1></div></header>
  <p class="sub">Play YouTube and YouTube Music links through the same station API used by SiriusXM Sidecar. Save stations you use often, or pass any YouTube link directly to the stream endpoint. Public videos do not require browser sign-in.</p>

  <section class="panel">
    <div class="row"><div><div class="step">01 · Playback</div><h2>Choose how yt-dlp reads the media</h2></div></div>
    <p class="help">Direct mode starts playback without saving the full media. Temporary download mode finishes a temporary download first, then removes it when playback ends.</p>
    <div class="controls"><select id="mode" aria-label="Playback mode"><option value="direct">Direct stream</option><option value="download">Temporary download</option></select><button class="secondary" id="saveMode">Save mode</button><span id="modeMessage" class="message" style="margin:0"></span></div>
  </section>

  <section class="panel">
    <div class="step">02 · Stations</div><h2>Add a YouTube video or music link</h2>
    <p class="help">Saved stations appear in <span class="api">GET /api/stations</span>. Public, unlisted, shared, and YouTube Music links are supported when yt-dlp can access them.</p>
    <form id="addForm" class="controls"><input class="url" id="videoURL" type="url" placeholder="https://youtu.be/... or music.youtube.com/..." required><input class="name" id="stationName" type="text" maxlength="100" placeholder="Station name (optional)"><button id="addButton">Add station</button></form>
    <div class="message" id="addMessage" role="status"></div>
    <div id="stationList"></div>
  </section>

  <section class="panel">
    <div class="step">03 · STR API</div><h2>Sidecar address</h2>
    <p class="help">Set STR's stream-source base URL to your computer's LAN address. Each saved station has an audio stream and a video stream URL.</p>
    <div class="callout">Base URL: <span id="baseURL" class="api">Loading…</span><br><br>Ad hoc URL pattern: <span id="dynamicExample" class="api">Loading…</span><br>Append <span class="api">?format=video</span> to stream video with audio. The default stream is MP3 audio for speaker compatibility.</div>
  </section>
  <p class="footer">Requires yt-dlp and FFmpeg on PATH. To change their locations, set YT_DLP and FFMPEG before launching the sidecar.</p>
</main>
<script>
const $=s=>document.querySelector(s);let state;
async function api(path,options={}){const res=await fetch('/app/api'+path,{...options,headers:{'Content-Type':'application/json',...(options.headers||{})}});const body=await res.json().catch(()=>({error:'Unexpected response'}));if(!res.ok)throw new Error(body.error||('Request failed ('+res.status+')'));return body}
function msg(el,text,type=''){el.textContent=text;el.className='message '+type}
function streamURL(s,format='audio'){return state.base_url+'/api/stations/'+encodeURIComponent(s.id)+'/stream'+(format==='video'?'?format=video':'')}
async function refresh(){state=await api('/state');$('#mode').value=state.stream_mode;$('#baseURL').textContent=state.base_url;$('#dynamicExample').textContent=state.base_url+'/api/stations/<URL-encoded YouTube URL>/stream';renderStations()}
function renderStations(){const root=$('#stationList');if(!state.stations.length){root.innerHTML='<div class="callout" style="margin-top:12px">No saved stations yet. Add a YouTube link above.</div>';return}root.innerHTML=state.stations.map(s=>'<div class="station"><div><div class="station-title">'+escapeHTML(s.name||s.title||'YouTube video')+'</div><div class="station-id">'+escapeHTML(s.id)+'</div></div><div class="station-actions"><a class="button secondary small" href="'+streamURL(s)+'" target="_blank" rel="noreferrer">Play audio</a><a class="button secondary small" href="'+streamURL(s,'video')+'" target="_blank" rel="noreferrer">Play video</a><button class="ghost small remove" data-id="'+escapeHTML(s.id)+'">Remove</button></div></div>').join('');root.querySelectorAll('.remove').forEach(b=>b.addEventListener('click',async()=>{try{await api('/stations/'+encodeURIComponent(b.dataset.id),{method:'DELETE'});await refresh()}catch(e){msg($('#addMessage'),e.message,'error')}}))}
function escapeHTML(s){return String(s).replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]))}
$('#saveMode').onclick=async()=>{try{await api('/mode',{method:'POST',body:JSON.stringify({mode:$('#mode').value})});msg($('#modeMessage'),'Saved.','good')}catch(e){msg($('#modeMessage'),e.message,'error')}};
$('#addForm').onsubmit=async e=>{e.preventDefault();const b=$('#addButton');b.disabled=true;msg($('#addMessage'),'Reading video details with yt-dlp…');try{const station=await api('/stations',{method:'POST',body:JSON.stringify({url:$('#videoURL').value,name:$('#stationName').value})});$('#videoURL').value='';$('#stationName').value='';await refresh();msg($('#addMessage'),'Added “'+station.name+'”.','good')}catch(e){msg($('#addMessage'),e.message,'error')}finally{b.disabled=false}};
refresh().catch(e=>msg($('#addMessage'),e.message,'error'));
</script>
</body>
</html>`
