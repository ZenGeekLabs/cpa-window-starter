const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');
const path = require('node:path');
const dir = path.resolve(__dirname, '../web');
const html = fs.readFileSync(path.join(dir, 'status.html'), 'utf8');
const fixture = JSON.parse(fs.readFileSync(path.join(__dirname, 'cpa_session.test.cjs'), 'utf8').match(/const fixture = (.+);/)[1]);
class Element {
 constructor() { this.handlers=new Map(); this.children=[]; this.textContent=''; this.value=''; this.hidden=false; this.disabled=false; this.classList={add(){},remove(){},toggle(){}}; }
 append(...items) { this.children.push(...items); }
 replaceChildren(...items) { this.children=items; }
 addEventListener(name,handler) { this.handlers.set(name,handler); }
 setAttribute() {}
 querySelectorAll() { return []; }
}
function environment(options={}) {
 const nodes=new Map([...html.matchAll(/id="([^"]+)"/g)].map(m=>[m[1],new Element()]));
 nodes.get('workspace').hidden=true;
 const values={isLoggedIn:'true','cli-proxy-auth':fixture,...options.storage};
 const win={location:new URL('http://127.0.0.1:18419/v0/resource/plugins/cpa-window-starter/status'),navigator:{userAgent:'CPA test browser'},localStorage:{getItem:name=>values[name]??null}};
 const handlers=new Map(), intervals=[], calls=[], writes=[], mediaHandlers=new Map();
 const media={matches:!!options.darkSystem,addEventListener:(name,fn)=>mediaHandlers.set(name,fn)};
 win.matchMedia=()=>media;win.parent=win;
 win.localStorage.setItem=(name,value)=>{values[name]=value;writes.push([name,value]);};
 win.navigator.language='zh-CN';
 win.addEventListener=(name,handler)=>handlers.set(name,handler);
 const config={enabled:true,schedule_enabled:false,times:['07:00','13:00','19:00'],timezone:'Asia/Shanghai',model:'gpt-6-sol',accounts:[]};
 const status={config,results:[],running:false};
 const response=(code,data)=>({ok:code===200,status:code,json:async()=>data});
 let resolveRequest;
 const fetch=async(url,init)=>{
  calls.push({url,init});
  if(options.pending) return new Promise(resolve=>{resolveRequest=()=>resolve(response(200,status));});
  if(options.unauthorized) return response(401,{error:'unauthorized'});
  return response(200,url.endsWith('/accounts')?{accounts:[]}:status);
 };
 const documentElement={lang:'zh-CN',dataset:{}};
 const context=vm.createContext({CPATimezonePicker:{mount:()=>({refresh(){}})},window:win,location:win.location,document:{documentElement,querySelectorAll:()=>[],getElementById:id=>nodes.get(id),createElement:()=>new Element(),hidden:false},URL,URLSearchParams,TextEncoder,TextDecoder,Uint8Array,atob,Intl,Date,fetch,setInterval:fn=>{intervals.push(fn);return 1;},setTimeout,clearTimeout});
 vm.runInContext(fs.readFileSync(path.join(dir,'cpa-session.js'),'utf8'),context);
 for(const file of ['ui.js','i18n.js','app.js']) vm.runInContext(fs.readFileSync(path.join(dir,file),'utf8'),context);
 return {nodes,values,calls,handlers,intervals,writes,media,mediaHandlers,documentElement,resolveRequest:()=>resolveRequest()};
}
const flush=()=>new Promise(resolve=>setImmediate(resolve));
test('saved CPA login opens the real page code without a password field or model request',async()=>{
 const env=environment(); await flush();
 assert.equal(/type="password"|id="managementKey"|id="loginForm"/.test(html),false);
 assert.equal(env.nodes.get('workspace').hidden,false);
 assert.equal(env.nodes.get('connectionPanel').hidden,true);
 assert.equal(env.nodes.get('headerActions').hidden,false);
 assert.equal(env.calls.length,3);
 for(const call of env.calls){assert.equal(call.init.headers.Authorization,'Bearer local-plugin-smoke-management');assert.equal(call.init.method,undefined);assert.equal(call.url.includes('local-plugin-smoke-management'),false);}
 assert.equal(env.calls.some(c=>c.url.includes('/run')),false);
});
test('stale CPA login probes once and returns to native settings',async()=>{
 const env=environment({unauthorized:true}); await flush();
 assert.equal(env.calls.length,1);
 assert.equal(env.nodes.get('workspace').hidden,true);
 assert.equal(env.nodes.get('nativeSettings').href,'http://127.0.0.1:18419/management.html#/plugins');
});
test('logout clears the plugin session and stops status polling',async()=>{
 const env=environment(); await flush();
 env.values.isLoggedIn='false'; env.handlers.get('storage')({key:'isLoggedIn'});
 env.intervals[0](); await flush();
 assert.equal(env.nodes.get('workspace').hidden,true);
 assert.equal(env.calls.length,3);
});
test('a pending login response cannot reopen the page after CPA logout',async()=>{
 const env=environment({pending:true});
 env.values.isLoggedIn='false'; env.handlers.get('storage')({key:'isLoggedIn'});
 env.resolveRequest(); await flush();
 assert.equal(env.calls.length,1);
 assert.equal(env.nodes.get('workspace').hidden,true);
});

test('page follows CPA and system appearance live without changing CPA preferences',async()=>{
 const env=environment({storage:{'cli-proxy-theme':JSON.stringify({state:{theme:'dark',resolvedTheme:'dark'},version:0})}});await flush();
 assert.equal(env.documentElement.dataset.theme,'dark');
 env.values['cli-proxy-theme']=JSON.stringify({state:{theme:'white',resolvedTheme:'light'},version:0});
 env.handlers.get('storage')({key:'cli-proxy-theme'});assert.equal(env.documentElement.dataset.theme,'light');
 env.values['cli-proxy-theme']=JSON.stringify({state:{theme:'auto',resolvedTheme:'light'},version:0});
 env.media.matches=true;env.mediaHandlers.get('change')();assert.equal(env.documentElement.dataset.theme,'dark');
 env.values['cli-proxy-theme']=JSON.stringify({state:{theme:'light'},version:0});
 env.intervals[0]();await flush();assert.equal(env.documentElement.dataset.theme,'light');
 assert.equal(env.writes.length,0);assert.equal(env.calls.some(c=>c.init.method||c.url.includes('/run')),false);
});
test('retired plugin preferences never override CPA language or appearance',async()=>{
 const env=environment({storage:{'cli-proxy-theme':JSON.stringify({state:{theme:'white'}}),'cli-proxy-language':JSON.stringify({state:{language:'en'}}),'cpa-window-starter-theme':'dark','cpa-window-starter-language':'zh-CN'}});await flush();
 assert.equal(env.nodes.has('theme'),false);assert.equal(env.nodes.has('language'),false);
 assert.equal(env.documentElement.dataset.theme,'light');assert.equal(env.documentElement.lang,'en');
 env.values['cli-proxy-language']=JSON.stringify({state:{language:'zh-CN'}});
 env.handlers.get('storage')({key:'cli-proxy-language'});assert.equal(env.documentElement.lang,'zh-CN');
 assert.equal(env.writes.length,0);
});
