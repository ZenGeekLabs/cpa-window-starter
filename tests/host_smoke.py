#!/usr/bin/env python3
"""Verify plugin loading in an isolated official CPA with an empty auth directory.

This fixture uses loopback port 18419 and a fabricated management key.
It refuses to mutate a host that has any accounts or enabled scheduling.
"""
import argparse
import json
import pathlib
import time
import urllib.error
import urllib.request

parser = argparse.ArgumentParser()
parser.add_argument('--report', required=True)
args = parser.parse_args()
base = 'http://127.0.0.1:18419'
root = '/v0/management/plugins/cpa-window-starter'

def request(path, method='GET', body=None, authenticated=True):
    headers = {'Authorization': 'Bearer local-plugin-smoke-management'} if authenticated else {}
    if body is not None:
        headers['Content-Type'] = 'application/json'
    req = urllib.request.Request(base + path, method=method, headers=headers, data=json.dumps(body).encode() if body is not None else None)
    try:
        with urllib.request.urlopen(req, timeout=10) as response:
            data = response.read()
            return response.status, response.headers, json.loads(data) if 'json' in response.headers.get('Content-Type', '') else data
    except urllib.error.HTTPError as error:
        return error.code, error.headers, error.read()

code, headers, listing = request('/v0/management/plugins')
assert code == 200 and headers.get('X-Cpa-Support-Plugin') == '1'
plugin = next(row for row in listing['plugins'] if row['id'] == 'cpa-window-starter')
assert plugin['registered'] and plugin['effective_enabled']
assert any(menu['menu'] == '账号定时预热' for menu in plugin['menus'])
code, _, accounts = request(root + '/accounts')
assert code == 200 and accounts['accounts'] == [], 'Test host must have no accounts'
code, _, before = request(root + '/status')
assert code == 200 and not before['config']['schedule_enabled'] and not before['config']['accounts'], 'Test host must not be scheduled'
assert request(root + '/accounts', authenticated=False)[0] == 401
for path, mime in [('status?run=1', 'text/html'), ('app.js', 'javascript'), ('style.css', 'text/css'), ('cpa-session.js', 'javascript')]:
    code, resource_headers, body = request('/v0/resource/plugins/cpa-window-starter/' + path, authenticated=False)
    assert code == 200 and mime in resource_headers['Content-Type'] and body
    assert 'Content-Security-Policy' in resource_headers
assert request('/v0/resource/plugins/cpa-window-starter/run', authenticated=False)[0] == 404
config = {'schedule_enabled': False, 'times': ['08:30', '14:45', '21:15'], 'timezone': 'Asia/Shanghai', 'model': 'gpt-6-sol', 'accounts': []}
assert request(root + '/validate', 'POST', config)[0] == 200
assert request(root + '/config', 'PATCH', config)[0] == 200
for _ in range(30):
    code, _, current = request(root + '/status')
    if current['config']['times'] == config['times']:
        break
    time.sleep(0.1)
assert current['config']['times'] == config['times'], 'Saved config did not reach native plugin'
code, _, stored = request(root + '/config')
assert code == 200 and stored['times'] == config['times']
assert not current['running'] and not current['results'] and not current.get('next_trigger')
report = {'host': 'CLIProxyAPI ' + headers.get('X-Cpa-Version', 'unknown'), 'plugin_version': plugin['metadata']['version'], 'platform': 'darwin/arm64', 'registered': True, 'effective_enabled': True, 'management_auth': 'passed', 'static_resources': 'passed', 'custom_times_saved_and_applied': config['times'], 'real_accounts_used': False, 'real_model_requests': 0}
path = pathlib.Path(args.report)
path.parent.mkdir(parents=True, exist_ok=True)
path.write_text(json.dumps(report, ensure_ascii=False, indent=2))
print('Official CPA: native registration, menu, management authentication, static assets and hot-reloaded custom times passed.')
