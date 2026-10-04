const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');
const source = fs.readFileSync(new URL('../web/cpa-session.js', 'file://' + __filename), 'utf8');
const context = vm.createContext({ TextEncoder, TextDecoder, Uint8Array, URL, atob });
vm.runInContext(source, context);
const session = context.CPAPluginSession;
function browser(values = {}) {
 return { location: new URL('http://127.0.0.1:18419/v0/resource/plugins/cpa-window-starter/status'), navigator: { userAgent: 'CPA test browser' }, localStorage: { getItem: name => values[name] ?? null } };
}
const fixture = "enc::v1::GE4aWREGClpDVkMRGUQ1BBEQSwAYGxEXBUhKAkJGWFxRSVVSAAgGFgQfCQwdGFxZWlBeGS41L1Q/AApWGkAeABQSCV8TABxKGRxCCxRCChVEQBYLAxIMV18dEUFZUBdIHhECEAQVNR1CQUBBQkoSFEVIRF1JHRsKJiIySRsLUU4QHw==";
function state(values) { return JSON.stringify({ state: values, version: 0 }); }
test('uses the official obfuscated saved login without another password', () => {
 assert.equal(session.savedConnection(browser({ isLoggedIn:'true', 'cli-proxy-auth':fixture })).managementKey, 'local-plugin-smoke-management');
});
test('logged-out state cannot reuse a stale saved key', () => {
 assert.equal(session.savedConnection(browser({ 'cli-proxy-auth':fixture })), null);
});
test('unsaved login routes to native settings without reading a legacy key', () => {
 assert.equal(session.savedConnection(browser({ isLoggedIn:'true', 'cli-proxy-auth':state({apiBase:'http://127.0.0.1:18419',rememberPassword:false}), managementKey:'stale-key' })), null);
});
test('never forwards credentials stored for a different origin or CPA path', () => {
 for (const base of ['https://other.example', 'http://127.0.0.1:18419/another-cpa', 'http://name:password@127.0.0.1:18419']) {
  assert.equal(session.savedConnection(browser({isLoggedIn:'true','cli-proxy-auth':state({apiBase:base,managementKey:'fixture-key',rememberPassword:true})})), null);
 }
});
test('legacy remembered login is accepted only for this CPA origin', () => {
 assert.equal(session.savedConnection(browser({ isLoggedIn:'true', apiBase:JSON.stringify('http://127.0.0.1:18419/v0/management'), managementKey:JSON.stringify('fixture-key') })).managementKey,'fixture-key');
});
test('unavailable storage and malformed codecs fall back without logging or requests', () => {
 const win=browser(); win.localStorage.getItem=()=>{throw new Error('blocked');}; assert.equal(session.savedConnection(win),null);
 assert.equal(session.savedConnection(browser({isLoggedIn:'true','cli-proxy-auth':'enc::v1::not base64'})),null);
 assert.equal(session.savedConnection(browser({isLoggedIn:'true','cli-proxy-auth':'enc::v2::unsupported'})),null);
});
test('native settings navigation preserves reverse-proxy prefix and contains no key', () => {
 const win=browser(); win.location=new URL('http://127.0.0.1:18419/cpa/v0/resource/plugins/cpa-window-starter/status');
 assert.equal(session.nativeSettingsURL(win),'http://127.0.0.1:18419/cpa/management.html#/plugins');
});
