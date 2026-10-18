// Memory defaults and per-preset overrides. Conversation vision is independent.
let memoryDefaults = null, memoryPaths = null;
async function loadMemoryPreferences(){
  try{
    const s=await jget('/api/embeddings'); memoryDefaults=s.defaults; memoryPaths=s.default_paths;
    const el=document.getElementById('memory-system'); if(el) el.value=s.defaults.enabled?'hybrid':'classic';
    const sub=document.getElementById('memory-system-effective');
    if(sub) sub.textContent=(s.preset_memory_system?'Preset prioritaire · ':'Actif · ')+(s.config.enabled?(s.ready&&s.chunks?'Hybride prêt':'Hybride demandé · repli Classique tant que l’index n’est pas prêt'):'Classique');
  }catch(_){}
}
async function setMemorySystem(mode){
  try{const r=await jpost('/api/embeddings/mode',{mode});if(r.error)throw Error(r.error);await loadMemoryPreferences();toast('Système mémoire enregistré.');}catch(e){toast(e.message);await loadMemoryPreferences();}
}
function memoryValue(key,fallback){return cfgReadKey('MEMORY_EMBED_'+key)||fallback;}
async function populateMemoryPreset(){
  try{
    const [s,bins,models]=await Promise.all([jget('/api/embeddings'),jget('/api/backends'),jget('/api/models')]);
    memoryDefaults=s.defaults;memoryPaths=s.default_paths;
    document.getElementById('m-memory-mode').value=cfgReadKey('MEMORY_SYSTEM');
    document.getElementById('m-memory-provider').value=memoryValue('PROVIDER',s.defaults.provider);
    document.getElementById('m-memory-bin').value=memoryValue('BIN',s.defaults.bin);
    document.getElementById('m-memory-model').value=memoryValue('MODEL',s.defaults.model);
    document.getElementById('m-memory-device').value=memoryValue('DEVICE',s.defaults.device);
    document.getElementById('m-memory-gpu').value=memoryValue('GPU_DEVICE',s.defaults.gpu_device||'');
    document.getElementById('m-memory-dimensions').value=cfgReadKey('MEMORY_EMBED_DIMENSIONS');
    document.getElementById('m-memory-threads').value=cfgReadKey('MEMORY_EMBED_THREADS');
    const active=memoryValue('MODALITIES',s.defaults.modalities).split(',');
    for(const m of ['image','audio','video']) document.getElementById('m-memory-'+m).checked=active.includes(m);
    for(const [id,items] of [['memory-backends',bins],['memory-models',models.filter(m=>/embeddinggemma/i.test(m.name))]]){
      const list=document.getElementById(id);list.replaceChildren();for(const item of items){const o=document.createElement('option');o.value=item.path;o.label=item.name;list.append(o);}
    }
    for(const p of Object.values(s.default_paths||{})){
      for(const [id,value] of [['memory-backends',p.bin],['memory-models',p.model]]){const o=document.createElement('option');o.value=value;document.getElementById(id).append(o);}
    }
    syncMemoryOptions();
  }catch(e){document.getElementById('m-memory-note').textContent='Réglages mémoire indisponibles : '+e.message;}
}
function syncMemoryOptions(){
  const provider=document.getElementById('m-memory-provider').value;
  document.getElementById('m-memory-gpu-row').style.display=document.getElementById('m-memory-device').value==='gpu'?'':'none';
  document.getElementById('m-memory-note').textContent=provider==='transformers'
    ?'Premier usage : Installer-Multimodal.cmd (runtime CPU portable + modèle complet). GPU : runtime PyTorch compatible requis. Médias locaux liés depuis les notes ; pas de médias dans le coffre chiffré.'
    :'GGUF : le moteur CPU est fourni. Pour le GPU, choisir un llama-server ROCm/Vulkan compatible. Les notes restent les sources de vérité.';
}
function changeMemoryProvider(){
  const provider=document.getElementById('m-memory-provider').value;
  cfgWriteKey('MEMORY_EMBED_PROVIDER',provider);
  if(memoryPaths && memoryPaths[provider]){
    for(const key of ['bin','model']){document.getElementById('m-memory-'+key).value=memoryPaths[provider][key];cfgWriteKey('MEMORY_EMBED_'+key.toUpperCase(),memoryPaths[provider][key]);}
  }
  if(provider==='llama_cpp'){
    for(const m of ['image','audio','video'])document.getElementById('m-memory-'+m).checked=false;
    cfgWriteKey('MEMORY_EMBED_MODALITIES','text');
  }
  if(document.getElementById('m-memory-device').value==='gpu') changeMemoryDevice();
  syncMemoryOptions();
}
function changeMemoryDevice(){
  const device=document.getElementById('m-memory-device').value;
  cfgWriteKey('MEMORY_EMBED_DEVICE',device);
  if(device==='gpu'){
    const id=document.getElementById('m-memory-provider').value==='transformers'?'cuda:0':'ROCm0';
    document.getElementById('m-memory-gpu').value=id;cfgWriteKey('MEMORY_EMBED_GPU_DEVICE',id);
    if(document.getElementById('m-memory-provider').value==='llama_cpp'&&cfgReadKey('BIN')){
      document.getElementById('m-memory-bin').value=cfgReadKey('BIN');cfgWriteKey('MEMORY_EMBED_BIN',cfgReadKey('BIN'));
    }
  }
  syncMemoryOptions();
}
function changeMemoryModalities(){
  const active=['text',...['image','audio','video'].filter(m=>document.getElementById('m-memory-'+m).checked)];
  if(active.length>1 && document.getElementById('m-memory-provider').value!=='transformers'){
    document.getElementById('m-memory-provider').value='transformers';changeMemoryProvider();
  }
  cfgWriteKey('MEMORY_EMBED_MODALITIES',active.join(','));syncMemoryOptions();
}
async function resetMemoryPreset(){
  for(const key of ['PROVIDER','BIN','MODEL','DEVICE','GPU_DEVICE','MODALITIES','DIMENSIONS','THREADS'])cfgWriteKey('MEMORY_EMBED_'+key,'');
  cfgWriteKey('MEMORY_SYSTEM','');await populateMemoryPreset();
}
