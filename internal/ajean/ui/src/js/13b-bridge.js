// Pont Ajean↔Ajean — prêter le cerveau de cette machine à un autre Ajean du
// même réseau local. Réservé à la version portable : /api/bridge répond 403
// ailleurs, et la carte reste cachée dans ce cas. Éteint par défaut côté Go ;
// cette carte ne va jamais allumer quelque chose d'elle-même, elle lit l'état.
//
// Ce que le pont expose : uniquement /v1/* (l'API du modèle), verrouillé par la
// clé API de la machine. Ni la mémoire, ni le chat, ni le pilotage.

let bridgeEtat = null;

function renderBridge(d){
  bridgeEtat = d;
  const card = document.getElementById('bridge-details');
  if(!card || !d || d.ok !== true) return;      // 403/404 : pas de carte ici
  card.hidden = false;

  const live = !!d.listening;
  setBadge('bridge-badge', live ? true : (d.enabled ? 'warn' : false),
           live ? 'à l’écoute' : (d.enabled ? 'à réveiller' : 'éteint'));

  const st = document.getElementById('bridge-state');
  if(st){
    st.textContent = live ? 'en ligne' : (d.enabled ? 'aucune écoute' : 'fermé');
    st.classList.toggle('on', live);
  }
  const on = document.getElementById('bridge-on');
  if(on) on.checked = !!d.enabled;
  const port = document.getElementById('bridge-port');
  if(port) port.value = d.port || '';
  const url = document.getElementById('bridge-url');
  if(url) url.value = (d.urls && d.urls.length && live) ? d.urls[0] : '';

  // Ce que le pont prêtera réellement. Les deux causes de « ouvert mais
  // injoignable » sont dites ici plutôt que de laisser l'utilisateur deviner :
  // pas de clé (le pont refuse tout le monde) et rien derrière (pas de cerveau).
  const mdl = document.getElementById('bridge-model');
  const note = document.getElementById('bridge-note');
  if(mdl && note){
    if(d.model){
      mdl.textContent = 'Modèle prêté : ' + d.model;
      note.textContent = 'Écoute sur le port ' + d.port + ' ; moteur derrière sur 127.0.0.1:' + (d.engine_serving || d.engine_port) + '.';
    } else if(live){
      mdl.textContent = 'Aucun modèle ne répond derrière ce pont';
      note.textContent = 'Le pont écoute mais le moteur est muet : activer un preset MoE (page Modèles), sinon l’autre Ajean recevra une erreur.';
    } else {
      mdl.textContent = 'Aucun modèle prêté';
      note.textContent = 'Un pont ouvert sans moteur actif n’a rien à prêter : activer un preset MoE (page Modèles).';
    }
  }
}

async function loadBridge(){
  try{
    const r = await jfetch('/api/bridge');
    if(r.status === 403 || r.status === 404) return;   // pas ce build
    const d = await r.json();
    renderBridge(d);
  }catch(e){ /* carte absente de cette version : on n'en parle pas */ }
}

async function bridgeToggle(wanted){
  const box = document.getElementById('bridge-on');
  try{
    const r = await jpost('/api/bridge', { enabled: !!wanted });
    if(!r.ok){ throw new Error(r.error || 'refusé'); }
    renderBridge(r);
    toast(wanted ? 'Pont ouvert sur le réseau local' : 'Pont coupé');
  }catch(e){
    if(box) box.checked = !wanted;
    toast('Pont : ' + e.message);
  }
}

async function bridgeSavePort(){
  const inp = document.getElementById('bridge-port');
  const p = ((inp && inp.value) || '').trim();
  try{
    const r = await jpost('/api/bridge', { port: p });
    if(!r.ok){ throw new Error(r.error || 'refusé'); }
    renderBridge(r);
    toast('Port du pont : ' + r.port);
  }catch(e){
    toast('Port du pont : ' + e.message);
    if(inp && bridgeEtat) inp.value = bridgeEtat.port || '';
  }
}
