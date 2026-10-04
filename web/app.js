'use strict';
const $=id=>document.getElementById(id);
const managementRoot=CPAPluginSession.resourcePrefix(window)+'/v0/management';
const apiRoot=managementRoot+'/plugins/cpa-window-starter';
let managementKey='', accounts=[], saved=null, currentStatus=null, dirty=false, connected=false, connectionRevision=0;
let timezonePicker=null;
let provider='codex', lang='zh-CN', connectionMessage='connecting', catalog=new Map(), modelsRevision=0, metadataUnavailable=false;
const t=(key,values={})=>{let text=translations[key]?.[lang==='en'?1:0]||key; for(const [k,v] of Object.entries(values)) text=text.replaceAll('{'+k+'}',v);return text;};
const backend=text=>lang==='en'?(backendEnglish[text] || (String(text).startsWith('Antigravity: ')?'Antigravity: '+backend(String(text).slice(13)):text)):text;
const node=(tag,text,cls)=>{const el=document.createElement(tag);if(text!==undefined) el.textContent=text;if(cls)el.className=cls;return el;};
function feedback(text,error=false){$('feedback').textContent=text;$('feedback').classList.toggle('error',error);clearTimeout(feedback.timer);feedback.timer=setTimeout(()=>$('feedback').textContent='',5000);}
async function api(path,options={}) {
 const response=await fetch(path,{...options,cache:'no-store',credentials:'same-origin',headers:{Authorization:`Bearer ${managementKey}`,'Content-Type':'application/json'}});
 let data;try{data=await response.json();}catch{throw new Error(t('invalidResponse'));}
 if(!response.ok){if(response.status===401||response.status===403){useNativeSettings('expired');throw new Error(t('expired'));}throw new Error(backend(typeof data.error==='string'?data.error:data.message||`HTTP ${response.status}`));}
 return data;
}
function selectedIDs(){return [...$('accountList').querySelectorAll('input:checked')].map(i=>i.value);}
function activeStatus(){return provider==='codex'?currentStatus:currentStatus?.antigravity;}
function markDirty(){dirty=true;$('dirtyHint').textContent=t('dirty');updateCount();}
function updateCount(){$('selectedCount').textContent=selectedIDs().length;}
function addTime(value=''){
 const wrapper=node('div',undefined,'timeEntry'),input=node('input');input.type='time';input.step='60';input.value=value;input.required=true;input.setAttribute('aria-label',t('time'));input.addEventListener('input',markDirty);
 const remove=node('button','×','remove');remove.type='button';remove.setAttribute('aria-label',t('removeTime'));remove.addEventListener('click',()=>{wrapper.remove();markDirty();});wrapper.append(input,remove);$('timeList').append(wrapper);
}
function renderAccounts(selected=CPAWindowUI.plan(saved||{},provider).accounts||[]){
 $('accountList').replaceChildren();const items=accounts.filter(a=>a.provider===provider);
 for(const id of selected)if(!items.some(a=>a.id===id))items.push({id,label:id,missing:true});
 if(!items.length){$('accountList').append(node('p',t('noAccounts'),'empty'));return updateCount();}
 const groups=new Map();for(const a of items){const plan=a.plan||'unknown';if(!groups.has(plan))groups.set(plan,[]);groups.get(plan).push(a);}
 for(const [plan,group] of groups){
  const heading=node('div',undefined,'accountGroup');heading.append(node('strong',`${plan==='unknown'?t('unknown'):plan} (${group.length})`));
  for(const [key,checked] of [['selectGroup',true],['clearGroup',false]]){
   const button=node('button',t(key),'secondary');button.type='button';button.addEventListener('click',()=>{const ids=new Set(selectedIDs());for(const a of group){if(checked&&!a.disabled&&!a.missing)ids.add(a.id);else if(!checked)ids.delete(a.id);}renderAccounts([...ids]);markDirty();renderModels();});heading.append(button);
  }
  $('accountList').append(heading);
  for(const a of group){
   const card=node('label',undefined,'accountCard'),input=node('input');input.type='checkbox';input.value=a.id;input.checked=selected.includes(a.id);input.addEventListener('change',()=>{markDirty();renderModels();});
   const details=node('span');details.append(node('strong',a.label||a.email||a.name||a.id),node('small',a.name||a.id));
   card.append(input,details,node('span',t(a.missing?'missing':a.disabled?'disabled':a.unavailable?'unavailable':'ready'),'badge'));$('accountList').append(card);
  }
 }
 $('metadataHint').textContent=metadataUnavailable?t('metadataFailed'):'';updateCount();
}
function renderModels(preferred=$('model').value){
 const ids=selectedIDs(),models=CPAWindowUI.commonModels(ids,catalog,provider),select=$('model');select.required=ids.length>0||provider==='codex';select.replaceChildren();
 const placeholder=node('option',t('chooseModel'));placeholder.value='';select.append(placeholder);
 for(const m of models){const option=node('option',m.id);option.value=m.id;select.append(option);}
 if(preferred&&!models.some(m=>m.id===preferred)){const old=node('option',`${preferred} (${t('unverifiedModel')})`);old.value=preferred;select.append(old);}
 select.value=preferred||'';
 const loading=ids.some(id=>!catalog.has(id));const failed=ids.some(id=>catalog.has(id)&&catalog.get(id)===null);
 $('modelHint').textContent=!ids.length?'':loading?t('modelsLoading'):failed?t('modelsFailed'):!models.length?t('noCommon'):'';
}
async function loadCatalog(revision){
 const generation=++modelsRevision;catalog=new Map();renderModels();let index=0;const list=[...accounts];
 await Promise.all(Array.from({length:Math.min(4,list.length)},async()=>{
  while(index<list.length){const a=list[index++];if(revision!==connectionRevision||generation!==modelsRevision)return;
   let models=null;try{const data=await api(`${managementRoot}/auth-files/models?name=${encodeURIComponent(a.name||a.id)}`);models=Array.isArray(data.models)?data.models.filter(m=>typeof m.id==='string'):null;}catch{}
   if(revision!==connectionRevision||generation!==modelsRevision)return;
   catalog.set(a.id,models);renderModels();
  }
 }));
}
async function enrichAccounts(revision){
 metadataUnavailable=false;
 try{const data=await api(`${managementRoot}/auth-files`);if(revision!==connectionRevision)return;
  const files=Array.isArray(data.files)?data.files:[];
  accounts=accounts.map(a=>{const meta=files.find(f=>f.id===a.id||(f.auth_index&&f.auth_index===a.auth_index)||(a.name&&f.name===a.name));return {...a,plan:meta?CPAWindowUI.planOf(meta):'unknown'};});
 }catch{if(revision!==connectionRevision)return;metadataUnavailable=true;}
 if(revision!==connectionRevision)return;
 const selected=selectedIDs();renderAccounts(selected);await loadCatalog(revision);
}
function zoneHint(){$('zoneOffset').textContent=CPAWindowUI.offset($('timezone').value);}
function loadForm(config){saved=config;dirty=false;const p=CPAWindowUI.plan(config,provider);$('scheduleEnabled').checked=!!p.schedule_enabled;$('timezone').value=p.timezone||'Asia/Shanghai';$('timeList').replaceChildren();for(const at of p.times||[])addTime(at);renderAccounts(p.accounts||[]);renderModels(p.model||'');$('dirtyHint').textContent='';zoneHint();timezonePicker?.refresh();}
function formatDate(value){if(!value||value.startsWith('0001-'))return '—';const date=new Date(value);if(Number.isNaN(+date))return '—';return new Intl.DateTimeFormat(lang,{timeZone:activeStatus()?.config.timezone||'UTC',month:'2-digit',day:'2-digit',hour:'2-digit',minute:'2-digit',second:'2-digit',hourCycle:'h23'}).format(date);}
function renderStatus(status){
 currentStatus=status;const s=activeStatus();if(!s)return;
 $('nextTrigger').textContent=formatDate(s.next_trigger);$('timezoneLabel').textContent=s.config.timezone+' · '+CPAWindowUI.offset(s.config.timezone);
 $('scheduleState').textContent=t(s.last_error?'inspect':s.running?'running':!s.config.schedule_enabled?'paused':s.config.accounts.length?'waiting':'chooseAccounts');
 $('runningHint').textContent=s.last_error?backend(s.last_error):s.config.accounts.length?t('timesPerDay',{n:s.config.times.length}):t('saveAccounts');
 $('runButton').disabled=s.running||!s.config.accounts.length||!!s.last_error;
 $('history').replaceChildren();const rows=[...(s.results||[])].reverse().slice(0,40);
 if(!rows.length){const tr=node('tr'),td=node('td',t('emptyHistory'),'empty');td.colSpan=4;tr.append(td);$('history').append(tr);}
 for(const row of rows){const tr=node('tr'),at=node('td',formatDate(row.scheduled_for));at.append(node('small',t(row.mode==='scheduled'?'scheduled':'manual')));const result=node('td');result.append(node('span',t(row.status),`tag ${row.status}`),node('small',backend(row.message)));const reset=node('td',row.reset_at?formatDate(row.reset_at):t('unconfirmed'));if(row.reset_at)reset.append(node('small',t('upstreamWindow')));tr.append(node('td',row.label),at,result,reset);$('history').append(tr);}
}
async function refreshStatus(){const revision=connectionRevision;const status=await api(`${apiRoot}/status`);if(revision!==connectionRevision)return null;renderStatus(status);return status;}
async function refreshAccounts(){const revision=connectionRevision,selected=selectedIDs();const data=await api(`${apiRoot}/accounts`);if(revision!==connectionRevision)return;accounts=data.accounts||[];renderAccounts(selected);await enrichAccounts(revision);}
function useNativeSettings(message){managementKey='';connected=false;connectionRevision++;modelsRevision++;accounts=[];saved=null;currentStatus=null;catalog.clear();$('accountList').replaceChildren();$('history').replaceChildren();$('connectButton').disabled=false;$('workspace').hidden=true;$('headerActions').hidden=true;$('connectionPanel').hidden=false;connectionMessage=message;$('connectionHint').textContent=t(message);}
async function connectSavedSession(){
 const revision=++connectionRevision;$('nativeSettings').href=CPAPluginSession.nativeSettingsURL(window);$('connectButton').disabled=true;
 const demo=new URLSearchParams(location.search).has('demo');const session=demo?{managementKey:'demo-key'}:CPAPluginSession.savedConnection(window);if(demo)$('demoNotice').hidden=false;
 if(!session){useNativeSettings('noSession');return;}managementKey=session.managementKey;
 try{const status=await api(`${apiRoot}/status`);if(revision!==connectionRevision)return;const data=await api(`${apiRoot}/accounts`);if(revision!==connectionRevision)return;
  accounts=data.accounts||[];connected=true;loadForm(status.config);renderStatus(status);$('connectionPanel').hidden=true;$('workspace').hidden=false;$('headerActions').hidden=false;
  void enrichAccounts(revision);
 }catch{if(revision===connectionRevision)useNativeSettings('connectFailed');}finally{if(revision===connectionRevision)$('connectButton').disabled=false;}
}
function setTheme(){document.documentElement.dataset.theme=CPAWindowUI.theme(window);}
function setLanguage(){
 const next=CPAWindowUI.language(window);lang=next;document.documentElement.lang=lang;document.title=t('title')+' · CPA';timezonePicker?.refresh();
 for(const el of document.querySelectorAll('[data-i18n]'))el.textContent=t(el.dataset.i18n);
 $('connectionHint').textContent=t(connectionMessage);
 if(connected){const selected=selectedIDs();renderAccounts(selected);renderModels();if(currentStatus)renderStatus(currentStatus);$('dirtyHint').textContent=dirty?t('dirty'):'';}
 for(const el of $('timeList').querySelectorAll('input'))el.setAttribute('aria-label',t('time'));
 for(const el of $('timeList').querySelectorAll('button'))el.setAttribute('aria-label',t('removeTime'));
}
function switchProvider(next){if(next===provider)return;if(dirty){feedback(t('switchDirty'),true);return;}provider=next;$('codexTab').classList.toggle('secondary',provider!=='codex');$('antigravityTab').classList.toggle('secondary',provider!=='antigravity');$('quotaNotice').hidden=provider!=='antigravity';loadForm(currentStatus.config);renderStatus(currentStatus);}
$('codexTab').addEventListener('click',()=>switchProvider('codex'));$('antigravityTab').addEventListener('click',()=>switchProvider('antigravity'));
try{const media=window.matchMedia?.('(prefers-color-scheme: dark)');if(media?.addEventListener)media.addEventListener('change',setTheme);else media?.addListener?.(setTheme);}catch{}
$('connectButton').addEventListener('click',connectSavedSession);
window.addEventListener('storage',event=>{
 if(event.key===null||event.key==='cli-proxy-theme')setTheme();
 if(event.key===null||event.key==='cli-proxy-language')setLanguage();
 if(new URLSearchParams(location.search).has('demo'))return;
 if(event.key!==null&&!['isLoggedIn','cli-proxy-auth','apiBase','apiUrl','managementKey'].includes(event.key))return;
 const session=CPAPluginSession.savedConnection(window);if(!session||session.managementKey!==managementKey)useNativeSettings('changed');
});
$('settingsForm').addEventListener('submit',async event=>{
 event.preventDefault();$('saveButton').disabled=true;
 const p={schedule_enabled:$('scheduleEnabled').checked,times:[...$('timeList').querySelectorAll('input')].map(i=>i.value),timezone:$('timezone').value.trim(),model:$('model').value.trim(),accounts:selectedIDs()};
 const updated=CPAWindowUI.mergePlan(saved,provider,p),prefix=provider==='antigravity'?'antigravity_':'';
 const patch=Object.fromEntries(Object.keys(p).map(k=>[prefix+k,p[k]]));const revision=connectionRevision;
 try{const normalized=await api(`${apiRoot}/validate`,{method:'POST',body:JSON.stringify(updated)});if(revision!==connectionRevision)return;
  await api(`${apiRoot}/config`,{method:'PATCH',body:JSON.stringify(patch)});if(revision!==connectionRevision)return;
  let status;for(let i=0;i<20;i++){status=await refreshStatus();if(revision!==connectionRevision||!status)return;
   if(JSON.stringify(CPAWindowUI.plan(status.config,provider))===JSON.stringify(CPAWindowUI.plan(normalized,provider)))break;
   if(i===19)throw new Error(t('notApplied'));await new Promise(r=>setTimeout(r,150));
  }loadForm(status.config);feedback(t('saved'));
 }catch(error){feedback(error.message,true);}finally{$('saveButton').disabled=false;}
});
$('runButton').addEventListener('click',async()=>{if(dirty){feedback(t('saveFirst'),true);return;}$('runButton').disabled=true;try{await api(`${apiRoot}/run`,{method:'POST',body:JSON.stringify({provider})});feedback(t('started'));await refreshStatus();}catch(error){feedback(error.message,true);}});
$('addTime').addEventListener('click',()=>{if($('timeList').children.length>=24){feedback(t('maxTimes'),true);return;}addTime();markDirty();});
$('scheduleEnabled').addEventListener('change',markDirty);$('timezone').addEventListener('input',()=>{markDirty();zoneHint();});$('model').addEventListener('change',markDirty);
$('refreshAccounts').addEventListener('click',()=>refreshAccounts().catch(e=>feedback(e.message,true)));$('refreshStatus').addEventListener('click',()=>refreshStatus().catch(e=>feedback(e.message,true)));
timezonePicker=CPATimezonePicker.mount({input:$('timezone'),zones:CPAWindowUI.zones(),language:()=>lang});
setTheme();setLanguage();
// A same-origin CPA iframe receives host changes without waiting for polling or storage writes.
try {
 if(window.parent!==window&&window.MutationObserver){
  const observer=new window.MutationObserver(()=>{setTheme();if(CPAWindowUI.language(window)!==lang)setLanguage();});
  observer.observe(window.parent.document.documentElement,{attributes:true,attributeFilter:['data-theme','lang']});
  window.addEventListener('pagehide',()=>observer.disconnect(),{once:true});
 }
}catch{}
setInterval(()=>{setTheme();if(CPAWindowUI.language(window)!==lang)setLanguage();if(connected&&!document.hidden)refreshStatus().catch(e=>feedback(e.message,true));},5000);
void connectSavedSession();
