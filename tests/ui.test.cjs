const test=require('node:test'),assert=require('node:assert/strict');
const ui=require('../web/ui.js');
test('language follows CPA without an independent override',()=>{
 const win={localStorage:{getItem:()=>JSON.stringify({state:{language:'en'}})},navigator:{language:'zh-CN'}};
 assert.equal(ui.language(win),'en');
 win.localStorage.getItem=()=>'{broken';assert.equal(ui.language(win),'zh-CN');
});
test('plan metadata is explicit, with unknown fallback and no raw JWT parsing',()=>{
 assert.equal(ui.planOf({id_token:{plan_type:'team'}}),'Team / Business');assert.equal(ui.planOf({plan_type:'pro'}),'Pro');
 assert.equal(ui.planOf({name:'plus-user.json',id_token:'raw-token'}),'unknown');
});
test('global timezones and DST offsets',()=>{
 assert.ok(ui.zones().includes('Europe/London'));assert.ok(ui.zones().includes('Asia/Tokyo'));
 assert.equal(ui.offset('America/New_York',new Date('2030-01-01')),'UTC-5');assert.equal(ui.offset('America/New_York',new Date('2030-07-01')),'UTC-4');
});
test('model intersection requires all selected accounts and filters Antigravity to Gemini',()=>{
 const catalog=new Map([['a',[{id:'gemini-pro'},{id:'claude-test'}]],['b',[{id:'gemini-pro'}]]]);
 assert.deepEqual(ui.commonModels(['a','b'],catalog,'antigravity'),[{id:'gemini-pro'}]);
 assert.deepEqual(ui.commonModels(['a','missing'],catalog,'antigravity'),[]);
 assert.deepEqual(ui.commonModels(['a'],catalog,'antigravity'),[{id:'gemini-pro'}]);
});
test('editing Gemini preserves all GPT fields and vice versa',()=>{
 const c={model:'gpt-test',accounts:['gpt'],times:['07:00'],antigravity_model:'gemini-test',antigravity_accounts:['gemini']};
 const next=ui.mergePlan(c,'antigravity',{model:'gemini-new',accounts:['new'],times:['08:00'],timezone:'UTC',schedule_enabled:false});
 assert.equal(next.model,'gpt-test');assert.deepEqual(next.accounts,['gpt']);assert.equal(ui.plan(next,'antigravity').model,'gemini-new');assert.equal(c.antigravity_model,'gemini-test');
});

function themeWindow(mode, dark=false) {
 const win={localStorage:{getItem:()=>JSON.stringify({state:{theme:mode,resolvedTheme:'dark'},version:0})},matchMedia:()=>({matches:dark})};
 win.parent=win;return win;
}
test('theme follows official CPA modes',()=>{
 for(const [mode,expected] of [['dark','dark'],['light','light'],['white','light']])assert.equal(ui.theme(themeWindow(mode)),expected);
 assert.equal(ui.theme(themeWindow('auto',false)),'light');assert.equal(ui.theme(themeWindow('auto',true)),'dark');
});
test('theme falls back safely for malformed or blocked storage and a cross-origin parent',()=>{
 const win=themeWindow('dark',true);win.localStorage.getItem=()=>'{malformed';
 assert.equal(ui.theme(win),'dark');
 win.parent={document:{documentElement:{dataset:{theme:'white'}}}};assert.equal(ui.theme(win),'light');
 win.parent.document.documentElement.dataset.theme='dark';assert.equal(ui.theme(win),'dark');
 win.localStorage.getItem=()=>{throw new Error('blocked');};
 Object.defineProperty(win,'parent',{get(){throw new Error('cross-origin');}});
 assert.equal(ui.theme(win),'dark');win.matchMedia=()=>{throw new Error('unavailable');};assert.equal(ui.theme(win),'light');
});

test('common model choices do not repeat the same model ID',()=>{
 const catalog=new Map([['one',[{id:'gpt-6-luna',display_name:'GPT 6.0 Luna'},{id:'gpt-6-luna',display_name:'GPT 6.0 Luna'}]],['two',[{id:'gpt-6-luna'}]]]);
 assert.deepEqual(ui.commonModels(['one','two'],catalog,'codex').map(model=>model.id),['gpt-6-luna']);
});

test('live host DOM wins over stale saved preferences and synchronizes before body exists',()=>{
 const win=themeWindow('dark');win.navigator={language:'en'};
 win.document={documentElement:{dataset:{},lang:''}};
 win.parent={document:{documentElement:{dataset:{theme:'white'},lang:'zh-CN'}}};
 ui.applyAppearance(win);assert.equal(win.document.documentElement.dataset.theme,'light');assert.equal(win.document.documentElement.lang,'zh-CN');
 win.parent.document.documentElement.dataset.theme='dark';win.parent.document.documentElement.lang='en';
 ui.applyAppearance(win);assert.equal(win.document.documentElement.dataset.theme,'dark');assert.equal(win.document.documentElement.lang,'en');
});
