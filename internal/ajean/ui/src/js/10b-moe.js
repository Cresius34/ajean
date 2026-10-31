// Moteur MoE : installation « en un clic » du moteur spécialisé Qwen3.8 Flash Next.
// L'utilisateur choisit la version (Swift ou Classique) et la qualité (quant) ;
// tout le reste (cartes, RAM, mmap, réglages) est déduit côté serveur
// (backend_moe.go). L'installation est une tâche comme celles de llama.cpp :
// même suivi (#lc-job), même pastille, même annulation.
let moeState = null, moeFam = '', moeQuant = '', moeTarget = '', moeBackend = '';

// moeEngineLabel : « Strata 0.1.54 ». Le serveur rend déjà la chaîne toute faite
// (« Strata » + version, strataPanelResponse), mais le NOM est traduit du côté de
// l'interface : coller l'un sur l'autre donnerait « Strata Strata 0.1.54 ». On
// garde donc la partie que le nom ne couvre pas.
function moeEngineLabel(v){
  const n = t('status.engine_moe');
  const s = String(v||'').trim();
  if(!s) return n;
  if(s.toLowerCase().startsWith(n.toLowerCase())) return s;
  return n+' '+s;
}

async function loadMoe(){
  let s;
  try{ s = await jget('/api/moe'+(moeBackend?'?backend='+encodeURIComponent(moeBackend):'')); }catch(_){ return; }
  moeState = s;
  const row = document.getElementById('lc-mode-moe');
  if(!row) return;
  // Hors Linux + NVIDIA compatible, la ligne n'apparaît pas : rien à y proposer.
  // Un prérequis manquant (Python, venv) la laisse visible, avec sa raison :
  // la cacher faisait disparaître le moteur sans explication (issue #125).
  const env = s.env || {};
  row.hidden = !(env.supported || env.visible);
  // Le moteur s'appelle Strata, et sa version se lit sur la ligne (demande de
  // l'humain, 31/10 au soir). La chaîne vient du serveur — strataPanelResponse
  // rend « Strata 0.1.54 » — donc elle suit le moteur posé, sans recompilation.
  // Le nom est écrit dans sa PROPRE portée : applyI18n ne pose le texte que sur
  // la portée data-i18n, un changement de langue n'écrase pas la version.
  const ver = document.getElementById('lc-moe-ver');
  if(ver){
    const nom = t('status.engine_moe');
    const entier = moeEngineLabel(s.version);
    // La ligne porte déjà « Strata » (porté par data-i18n) : on n'y met que le
    // reste, sinon la carte afficherait « Strata Strata 0.1.54 ».
    ver.textContent = entier.startsWith(nom) ? entier.slice(nom.length) : entier;
  }
  const st = document.getElementById('lc-moe-state');
  if(st) st.textContent = !env.supported ? (env.reason || '')
    : (s.installed||[]).length ? t('moe.state_installed').replace('{n}', s.installed.length)
    : '';
}

// presetId (optionnel) : ouverte depuis l'engrenage d'un modèle installé, la
// fenêtre se place directement sur lui.
async function openMoe(presetId){
  // état relu à chaque ouverture : juste après une installation, l'ancien
  // proposait encore « Installer »
  await loadMoe();
  if(!moeState) return;
  const targetBackend=moeState.preset_backends&&moeState.preset_backends[presetId];
  if(targetBackend&&targetBackend!==(moeState.env||{}).backend){moeBackend=targetBackend;await loadMoe();}
  if(!(moeState.env||{}).supported && !moeState.modern){ toast(moeState.env.reason || t('moe.chip')); return; }
  moeTarget = typeof presetId === 'string' ? presetId : '';
  const s = moeState, env = s.env || {};
  document.getElementById('moe-windows-options').hidden=!s.modern;
  if(s.modern){
    moeBackend=env.backend||moeBackend||'hip';
    document.getElementById('moe-backend').value=moeBackend;
    document.getElementById('moe-toolkit-row').hidden=moeBackend!=='cuda';
    const vision=document.getElementById('moe-install-vision');vision.disabled=moeBackend==='hip';if(vision.disabled)vision.checked=false;
    document.getElementById('moe-windows-note').textContent=(env.reason?env.reason+' · ':'')+(moeBackend==='hip'
      ?'HIP : pilote AMD requis, ROCm fourni dans le dossier portable. Ce paquet est texte ; la mémoire multimodale reste indépendante.'
      :'CUDA : runtime Python et bibliothèques dans le dossier portable. CUDA 12 est proposé pour les cartes non couvertes par CUDA 13.');
  }
  const cards = (env.gpus||[]).filter(g=>g.index===env.main || g.index===env.helper)
    .sort((a,b)=>(a.index===env.main?-1:1))
    .map(g=>g.name.replace(/^NVIDIA GeForce /,'')+(g.vram_gb?' '+Math.round(g.vram_gb)+' Go':' · VRAM vérifiée à la préparation'));
  document.getElementById('moe-machine').textContent =
    t('moe.machine').replace('{gpus}', cards.join(' + '))
      .replace('{ram}', Math.round(env.ram_gb)).replace('{disk}', Math.round(env.disk_free_gb));
  if(!moeFam) moeFam = (s.families[0]||{}).id || '';
  if(moeTarget){
    for(const f of s.families){
      const hit = f.fits.find(x=>x.preset===moeTarget);
      if(hit){ moeFam = f.id; moeQuant = hit.quant.id; }
    }
  }
  moeRenderFams();
  moeRenderQuants(!moeTarget);
  // Titre de la fenêtre : « Strata 0.1.54 ». Écrit après l'ouverture car la
  // fenêtre est reconstruite à chaque fois ; le data-i18n du gabarit reste le
  // repli si la version du moteur manque.
  const mt = document.getElementById('moe-modal-title');
  if(mt && s.version) mt.textContent = moeEngineLabel(s.version);
  showModal('moe-modal');
}
function closeMoe(){ hideModal('moe-modal'); }

function moeRenderFams(){
  const box = document.getElementById('moe-fams');
  box.innerHTML = moeState.families.map(f=>{
    // Une famille que ce poste ne peut pas installer en un clic reste visible,
    // avec sa raison : la cacher ferait disparaître un modèle sans explication
    // (le même choix que pour la ligne du moteur, note du 31/10/2026).
    const off = f.blocked ? ' moe-off' : '';
    // La pastille ne porte QUE le nom du modèle. Sa description — le « about »
    // du moteur, jusqu'à 300 caractères chez unsloth — vivait ici et faisait la
    // hauteur de la pastille active, donc le pilier blanc du curseur et le texte
    // coupé caractère par caractère. Elle se lit sous le groupe, voir
    // moeFamAbout(). La raison d'une famille écartée reste au survol.
    return '<label class="lc-mode'+off+'"'+(f.blocked?' title="'+escHtml(f.blocked)+'"':'')+'><input type="radio" name="moe-fam" value="'+f.id+'"'+(f.id===moeFam?' checked':'')
    +' onchange="moeFam=this.value;moeFamAbout();moeRenderQuants(true)">'
    +escHtml(f.label)+'</label>';
  }).join('');
  moeFamAbout();
  // innerHTML removes the existing slider thumb. Trigger its native checked
  // setter after a backend refresh, even if the family itself is unchanged.
  const checked=box.querySelector('input:checked');if(checked)checked.checked=true;
}

// moeFamAbout : la description du modèle choisi, sous les pastilles.
//
// Deux régimes, et c'est voulu (arbitrage du 31/10 au soir avec l'humain) :
//   - famille ÉCARTÉE : sa raison est écrite EN ENTIER. Une famille bloquée ne
//     rend aucune taille, la liste des qualités sous elle est vide — cette ligne
//     est le seul endroit où la raison s'affiche. Sans elle, cliquer NVFP4 ne
//     donne aucune explication (le 31/10, « fits » vide et blocked posé,
//     l'interface ne disait rien : c'est le défaut réparé plus haut).
//   - famille NORMALE : le « about » du moteur répète en grande partie la liste
//     des qualités (celui d'unsloth énumère ses cinq tailles et leurs seuils de
//     RAM, que chaque ligne détaille déjà). Il est donc coupé à deux lignes par
//     le CSS, et le texte complet reste lisible au survol.
function moeFamAbout(){
  const el = document.getElementById('moe-fam-about');
  if(!el) return;
  const f = ((moeState||{}).families||[]).find(x=>x.id===moeFam);
  const texte = f ? (f.blocked || f.about || '') : '';
  el.textContent = texte;
  el.classList.toggle('moe-fam-blocke', !!(f && f.blocked));
  // Le survol rend le texte entier, y compris quand il est coupé.
  if(f && f.about) el.title = f.about; else el.removeAttribute('title');
}

// moeCatalogueLine : d'où vient la liste proposée. C'est la ligne qui rend le
// défaut visible : si le panneau est retombé sur notre liste recopiée (quatre
// tailles), l'humain doit pouvoir le lire au lieu de croire que le moteur
// n'a rien d'autre.
function moeCatalogueLine(){
  const el = document.getElementById('moe-catalogue');
  if(!el) return;
  const s = moeState || {};
  if(s.catalogue === 'moteur'){
    const n = (s.families||[]).reduce((a,f)=>a+((f.fits||[]).length), 0);
    el.textContent = 'Modèles lus dans le catalogue du moteur ('+escHtml(s.catalogue_source||'setup.py')+') : '
      +(s.families||[]).length+' familles, '+n+' tailles.';
  }else{
    el.textContent = 'Catalogue du moteur indisponible — liste interne d’Ajean ('
      +((s.families||[]).length)+' familles). '+(s.catalogue_error||'');
  }
}

function moeRenderQuants(pickReco){
  const f = moeState.families.find(x=>x.id===moeFam);
  if(!f) return;
  // à l'ouverture : le modèle actif s'il est de cette version, sinon un installé, sinon le conseillé
  if(pickReco || !f.fits.some(x=>x.quant.id===moeQuant && (x.ok || x.preset))){
    const cur = f.fits.find(x=>x.active) || f.fits.find(x=>x.preset);
    moeQuant = cur ? cur.quant.id : (f.recommend || '');
  }
  const box = document.getElementById('moe-quants');
  box.innerHTML = f.fits.map(x=>{
    const q = x.quant, sel = q.id===moeQuant;
    const tag = x.active ? t('moe.active') : x.preset ? t('moe.installed') : q.id===f.recommend ? t('moe.recommended') : '';
    const reco = tag ? ' <span class="lc-reco">'+tag+'</span>' : '';
    // installé : activable même si l'espace disque manque ; une famille que ce
    // poste ne peut pas charger n'est pas installable, mais reste affichée.
    const usable = !!x.preset || (x.ok && !x.blocked);
    const desc = x.preset ? escHtml(q.about)
      : x.blocked ? escHtml(x.blocked)
      : x.ok ? escHtml(q.about)+' · '+t('moe.download').replace('{gb}', Math.round(x.disk_gb))
      : escHtml(x.why);
    return '<div class="lc-mode'+(sel?' moe-sel':'')+(usable?'':' moe-off')+'"'
      +(usable?' onclick="moeQuant=\''+q.id+'\';moeRenderQuants(false)"':'')+'>'
      +'<div class="lc-mode-t">'+q.id+reco+'</div><div class="lc-mode-d">'+desc+'</div></div>';
  }).join('');
  const fit = f.fits.find(x=>x.quant.id===moeQuant);
  document.querySelectorAll('[data-moe-install-only]').forEach(el=>{el.hidden=!!(fit&&fit.preset)||(el.id==='moe-toolkit-row'&&moeBackend!=='cuda');});
  const go = document.getElementById('moe-go');
  go.disabled = !fit || !!fit.active || (!!moeState.modern && !(moeState.env||{}).supported && !fit.preset);
  document.getElementById('moe-memory-edit').hidden=!(fit&&fit.preset);
  document.getElementById('moe-verify').hidden=!(moeState.modern&&fit&&fit.active);
  document.getElementById('moe-verify-result').textContent='';
  document.getElementById('moe-bench').hidden = !(fit && fit.active);
  go.textContent = fit && fit.active ? t('moe.is_active') : fit && fit.preset ? t('moe.activate') : t('moe.install');
  moeRenderDisk(fit);
  moeRenderVerdict(fit);
  moeRenderSettings(fit);
  moeRenderDetails(fit);
  moeCatalogueLine();
  document.getElementById('moe-note').textContent = !fit ? t('moe.nothing_fits')
    : fit.active ? t('moe.note_active') : fit.preset ? t('moe.note_installed')
    : (fit.mmap ? t('moe.note_mmap') : t('moe.note'));
}

// Place disque : libre, ce que ce choix occupe, ce qui restera.
function moeRenderDisk(fit){
  const el = document.getElementById('moe-disk');
  const free = Math.round((moeState.env||{}).disk_free_gb||0);
  if(!fit){ el.textContent = ''; return; }
  if(fit.preset){ el.textContent = t('moe.disk_installed').replace('{free}', free); return; }
  const need = Math.round(fit.disk_gb), left = free - need;
  el.textContent = (left >= 5 ? t('moe.disk_ok') : t('moe.disk_short'))
    .replace('{free}', free).replace('{need}', need).replace('{left}', Math.max(left, 0));
}

// moeRenderVerdict : l'avis du moteur sur cette machine, a cote du notre. On
// l'affiche sans lui laisser la decision : le moteur ne connait ni notre carte
// d'aide ni notre page file, et il rend les trois NVFP4 « fits » sur une carte
// AMD alors que le moteur HIP n'a pas de noyaux NVFP4 (docs/QUANTS.md, mesure
// du 31/10/2026). C'est un appui, pas un verdict.
function moeRenderVerdict(fit){
  const el = document.getElementById('moe-engine-verdict');
  if(!el) return;
  if(!fit || (!fit.engine_verdict && !fit.engine_min)){ el.textContent=''; return; }
  const mots = {fits:'tient sur ce poste', tight:'tient, mais juste', 'does not fit':'ne tient pas'};
  const v = fit.engine_verdict || '';
  const lib = mots[v.toLowerCase()] || v;
  const parts = [];
  if(lib) parts.push('Le moteur, sur ce poste : ' + lib + (v && lib!==v ? ' ('+v+')' : ''));
  if(fit.engine_min) parts.push('moteur Strata minimum ' + fit.engine_min);
  el.textContent = parts.join(' · ');
}

// Réglages modifiables d'un modèle installé.
function moeRenderSettings(fit){
  const fld = document.getElementById('moe-settings-fld'), save = document.getElementById('moe-save');
  const d = fit && fit.details;
  fld.hidden = save.hidden = !d;
  if(!d) return;
  const sel = (id, opts, cur) => '<select class="ctl" id="'+id+'" style="width:auto">'
    + opts.map(([v,l])=>'<option value="'+v+'"'+(String(v)===String(cur)?' selected':'')+'>'+escHtml(l)+'</option>').join('')+'</select>';
  const sw = (id, on) => '<label class="switch"><input type="checkbox" id="'+id+'"'+(on?' checked':'')+'><span class="slider"></span></label>';
  moeSetD = d;
  const rows = [
    [t('moe.d_ctx'), '<span class="moe-ctx"><input type="range" id="moe-s-ctx" min="32768" max="'+(d.ctx_max||262144)+'" step="4096" value="'+(+d.ctx||131072)+'" oninput="moeCtxLbl()"><span id="moe-s-ctx-v"></span></span>'],
    [t('moe.d_kv'), sel('moe-s-kv', d.modern?[['int8','int8'],['q4_0','q4_0'],['k8v4','k8v4']]:[['fp16', t('moe.kv_fp16')],['int8', t('moe.kv_int8')]], d.kv)],
    [t('moe.d_mtp'), sel('moe-s-spec', [[2,'2'],[3,'3'],[4,'4']], d.spec)],
    [t('moe.s_vision'), sw('moe-s-vision', d.vision)],
  ];
  if(d.modern)rows.push(['Réserve VRAM (Mio)','<input type="number" id="moe-s-reserve" min="0" max="65536" step="256" style="width:80px" value="'+d.vram_reserve_mib+'">']);
  // Seconde carte : aide aux experts (défaut mesuré) ou partage des couches,
  // souvent plus rapide avec deux cartes identiques (#121).
  if(d.has_helper) rows.push([t('moe.s_helper'), sel('moe-s-helper',
    [['off', t('moe.second_off')], ['helper', t('moe.second_helper')], ['layers', t('moe.second_layers')]],
    !d.helper_on ? 'off' : d.split_layers ? 'layers' : 'helper')]);
  // VRAM laissée à l'écran quand la carte principale gère aussi l'affichage (#147)
  const res = [0, 512, 1024, 1536, 2048, 3072, 4096];
  if(d.reserve_mib && !res.includes(d.reserve_mib)) res.push(d.reserve_mib);
  rows.push([t('moe.s_reserve'), sel('moe-s-reserve', res.sort((a,b)=>a-b).map(n=>[n, n ? (n/1024).toLocaleString('fr-FR')+' Go' : t('moe.reserve_none')]), d.reserve_mib||0)]);
  document.getElementById('moe-settings').innerHTML = rows.map(([a,b])=>
    '<div class="moe-kv"><span>'+escHtml(a)+'</span><span>'+b+'</span></div>').join('')
    + '<div class="moe-kv" id="moe-s-ctx-h" hidden><span></span><span class="lc-mode-d">'+escHtml(t('moe.ctx_drop_cap'))+'</span></div>';
  const hs = document.getElementById('moe-s-helper');
  if(hs) hs.addEventListener('change', moeCtxLbl);
  moeCtxLbl();
  if(d.modern&&!d.vision_available){const vision=document.getElementById('moe-s-vision');vision.disabled=true;vision.title='Encodeur absent : la vision se choisit pendant la préparation initiale du modèle (CUDA seulement).';}
}
// Le contexte maximal suit la config : en mode hors RAM (carte d'aide sans
// partage des couches) le lancement le ramène à drop_cap, le curseur s'y arrête.
let moeSetD = null;
function moeCtxCap(){
  const d = moeSetD || {}, hs = document.getElementById('moe-s-helper');
  const helper = hs ? hs.value === 'helper' : !!d.helper_on && !d.split_layers;
  return d.drop_cap && helper ? d.drop_cap : (d.ctx_max || 262144);
}
// valeur exacte en jetons (131 072 et non « 128K », source de confusion)
function moeCtxLbl(){
  const r = document.getElementById('moe-s-ctx'), o = document.getElementById('moe-s-ctx-v');
  if(!r || !o) return;
  const cap = moeCtxCap();
  r.max = cap;
  if(+r.value > cap) r.value = cap;
  const h = document.getElementById('moe-s-ctx-h');
  if(h) h.hidden = !(moeSetD && moeSetD.drop_cap && cap === moeSetD.drop_cap);
  o.textContent = (+r.value).toLocaleString('fr-FR');
  // partie remplie à gauche de la pastille (la piste est dessinée en CSS)
  r.style.setProperty('--p', ((r.value - r.min) / (r.max - r.min) * 100) + '%');
}

async function moeSaveSettings(){
  const f = moeState && moeState.families.find(x=>x.id===moeFam);
  const fit = f && f.fits.find(x=>x.quant.id===moeQuant);
  if(!fit || !fit.preset) return;
  const v = id => document.getElementById(id);
  const body = {preset: fit.preset, ctx: +v('moe-s-ctx').value, kv: v('moe-s-kv').value, spec: +v('moe-s-spec').value,
    vision: v('moe-s-vision').checked};
  if(v('moe-s-reserve'))body.reserve_mib=+v('moe-s-reserve').value;
  const second = v('moe-s-helper') ? v('moe-s-helper').value : 'off';
  body.helper = second !== 'off';
  if(body.helper) body.split = second;
  if(fit.active && !await askConfirm(t('moe.save_confirm'), {title:t('moe.modal_title'), okText:t('moe.save')})) return;
  const r = await jpost('/api/moe/settings', body);
  if(!r.ok){ toast(t('llamacpp.error_prefix')+(r.error||'')); return; }
  toast(r.restarted ? t('moe.saved_restart') : t('moe.saved'));
  moeTarget = fit.preset;
  await openMoe(fit.preset);
  if(typeof loadPresets === 'function') loadPresets();
}

// La config qui tourne (ou tournera) pour un modèle installé.
function moeRenderDetails(fit){
  const fld = document.getElementById('moe-details-fld');
  const d = fit && fit.details;
  fld.hidden = !d;
  if(!d) return;
  if(d.modern){
    const rows=[['Moteur',d.version],['Backend',d.backend.toUpperCase()],['Contexte',d.ctx],['Cache KV',d.kv],['MTP',d.spec],['Réserve VRAM',d.vram_reserve_mib+' Mio'],['Vision',d.vision?'active':'désactivée'],['Réglages de calcul','recommandations du moteur, puis choix du preset']];
    document.getElementById('moe-details').innerHTML=rows.map(([a,b])=>'<div class="moe-kv"><span>'+escHtml(a)+'</span><span>'+escHtml(String(b))+'</span></div>').join('');return;
  }
  const k = n => (+n >= 1024 ? Math.round(+n/1024)+'K' : n);
  const rows = [
    [t('moe.d_main'), d.main_gpu],
    [t('moe.d_helper'), d.helper_gpu ? d.helper_gpu + (d.split_layers ? ' · '+t('moe.second_layers') : '') : t('moe.d_none')],
    [t('moe.d_vision'), d.vision_gpu || t('moe.d_none')],
    [t('moe.s_reserve'), d.reserve_mib ? (d.reserve_mib/1024).toLocaleString('fr-FR')+' Go' : t('moe.reserve_none')],
    [t('moe.d_ctx'), (+d.ctx).toLocaleString('fr-FR')+' '+t('moe.d_tokens')],
    [t('moe.d_kv'), d.kv + (d.kv_resident ? ' · '+t('moe.d_kv_resident').replace('{n}', k(d.kv_resident)) : '')],
    [t('moe.d_mtp'), t('moe.d_mtp_val').replace('{n}', d.spec)],
    [t('moe.d_ram'), d.drop ? t('moe.d_drop') : d.mmap ? t('moe.d_mmap') : t('moe.d_inram')],
    [t('moe.d_read'), t('moe.d_read_val').replace('{chunk}', d.prefill ? k(d.prefill) : 'auto').replace('{short}', d.short_read)],
    [t('moe.d_root'), t('moe.d_root_val').replace('{n}', d.cache_root)],
    [t('moe.d_version'), d.version],
  ];
  document.getElementById('moe-details').innerHTML = rows.map(([a,b])=>
    '<div class="moe-kv"><span>'+escHtml(a)+'</span><span>'+escHtml(b)+'</span></div>').join('');
}

async function moeInstall(){
  const f = moeState && moeState.families.find(x=>x.id===moeFam);
  const fit = f && f.fits.find(x=>x.quant.id===moeQuant);
  if(!fit || fit.active) return;
  // déjà installé : simple bascule, la même que dans la liste des modèles
  if(fit.preset){
    closeMoe();
    await switchTo(0, f.label+' '+moeQuant, fit.preset);
    loadMoe();
    return;
  }
  const msg = t('moe.confirm').replace('{name}', f.label+' '+moeQuant).replace('{gb}', Math.round(fit.disk_gb));
  if(!await askConfirm(msg, {title:t('moe.modal_title'), okText:t('moe.install')})) return;
  const body={family:moeFam, quant:moeQuant};
  if(moeState.modern)Object.assign(body,{backend:moeBackend,toolkit:document.getElementById('moe-toolkit').value,context:+document.getElementById('moe-install-context').value,vram_reserve_mib:+document.getElementById('moe-install-reserve').value,vision:document.getElementById('moe-install-vision').checked});
  // Poids déjà descendus à la main : transmis à setup.py comme --gguf-dir. Vide =
  // téléchargement par le moteur. Un dossier inexistant est refusé côté serveur
  // avant de lancer la tâche, pas après 40 minutes de préparation.
  const gd=document.getElementById('moe-install-ggufdir');
  if(gd&&gd.value.trim())body.gguf_dir=gd.value.trim();
  const r = await jpost('/api/moe/install', body);
  if(!r.ok){ toast(t('llamacpp.error_prefix')+(r.error||'')); return; }
  closeMoe();
  document.getElementById('lc-details').open = true;
  lcStartPolling();
}

async function moeChangeBackend(){
  moeBackend=document.getElementById('moe-backend').value;moeQuant='';moeTarget='';await openMoe();
}
async function moeEditMemory(){
  const f=moeState&&moeState.families.find(x=>x.id===moeFam),fit=f&&f.fits.find(x=>x.quant.id===moeQuant);
  if(!fit||!fit.preset)return;closeMoe();await openItem('preset',fit.preset);
}
async function moeVerify(){
  const button=document.getElementById('moe-verify'),out=document.getElementById('moe-verify-result');
  button.disabled=true;out.textContent='Test en cours : réponse, puis appel d’outil sans exécution…';
  try{const r=await jpost('/api/moe/verify',{},240000);out.textContent=(r.ok?'Réponse et appel d’outil : réussis.':'Échec : '+(r.error||'moteur indisponible'))+(r.report?' Rapport : data/'+r.report:'');}
  catch(e){out.textContent='Échec : '+e.message;}finally{button.disabled=false;}
}

