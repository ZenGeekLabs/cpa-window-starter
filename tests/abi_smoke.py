#!/usr/bin/env python3
"""Exercise the real dynamic library against fake CPA callbacks, without credentials."""
import argparse
import base64
import ctypes as c
import datetime as dt
import json
import pathlib
import platform
import threading
import time

class Buffer(c.Structure):
    _fields_ = [('ptr', c.c_void_p), ('len', c.c_size_t)]
HostCall = c.CFUNCTYPE(c.c_int, c.c_void_p, c.c_char_p, c.POINTER(c.c_uint8), c.c_size_t, c.POINTER(Buffer))
Free = c.CFUNCTYPE(None, c.c_void_p, c.c_size_t)
PluginCall = c.CFUNCTYPE(c.c_int, c.c_char_p, c.POINTER(c.c_uint8), c.c_size_t, c.POINTER(Buffer))
Shutdown = c.CFUNCTYPE(None)
class HostAPI(c.Structure):
    _fields_ = [('abi_version', c.c_uint32), ('ctx', c.c_void_p), ('call', HostCall), ('free_buffer', Free)]
class PluginAPI(c.Structure):
    _fields_ = [('abi_version', c.c_uint32), ('call', PluginCall), ('free_buffer', Free), ('shutdown', Shutdown)]

parser = argparse.ArgumentParser()
parser.add_argument('library')
parser.add_argument('--work-dir', required=True)
parser.add_argument('--schedule', action='store_true')
args = parser.parse_args()
work = pathlib.Path(args.work_dir).resolve()
work.mkdir(parents=True, exist_ok=True)
state = work / 'state.json'
allocations = {}
executions = []
callback_errors = []
fail_auth = None
lock = threading.Lock()

@HostCall
def host_call(ctx, method, request, length, response):
    try:
        payload = json.loads(c.string_at(request, length))
        name = method.decode()
        error_status = 0
        if name == 'host.auth.list':
            result = {'files': [{'id': 'account-a', 'provider': 'codex', 'name': 'a.json', 'label': 'Mock A'}, {'id': 'account-b', 'provider': 'codex', 'name': 'b.json', 'label': 'Mock B'}, {'id':'account-g','provider':'antigravity','name':'g.json'}, {'id': 'disabled', 'provider': 'codex', 'name': 'c.json', 'disabled': True}]}
        elif name == 'host.model.execute':
            assert payload['forced_provider'] == ('antigravity' if payload['auth_id']=='account-g' else 'codex')
            assert payload['auth_id'] in ('account-a', 'account-b', 'account-g')
            body = json.loads(base64.b64decode(payload['body']))
            assert ('reasoning_effort' not in body) if payload['auth_id']=='account-g' else body['reasoning_effort']=='low'
            assert len(body['messages']) == 1
            saved = json.loads(pathlib.Path(str(state)+'.antigravity').read_text(encoding='utf-8') if payload['auth_id']=='account-g' else state.read_text(encoding='utf-8'))
            assert any(row['auth_id'] == payload['auth_id'] and row['status'] == 'running' for row in saved['results'])
            with lock:
                executions.append({'auth_id': payload['auth_id'], 'at': time.time()})
            if fail_auth == payload['auth_id']:
                error_status = 429
            result = {'status_code': 200, 'headers': {'X-Codex-Primary-Window-Minutes': ['300'], 'X-Codex-Primary-Reset-At': [str(int(time.time()) + 18000)], 'X-Codex-Primary-Used-Percent': ['0.1']}, 'body': base64.b64encode(b'{"choices":[{"message":{"content":"OK"}}]}').decode()}
        else:
            raise AssertionError(f'unexpected callback {name}')
        raw = json.dumps({'ok': False, 'error': {'code': 'host_call_failed', 'http_status': error_status}} if error_status else {'ok': True, 'result': result}).encode()
    except Exception as error:
        callback_errors.append(str(error))
        raw = json.dumps({'ok': False, 'error': {'code': 'mock_error', 'http_status': 500}}).encode()
    memory = c.create_string_buffer(raw)
    ptr = c.addressof(memory)
    with lock:
        allocations[ptr] = memory
    response[0].ptr = ptr
    response[0].len = len(raw)
    return 0

@Free
def host_free(ptr, length):
    with lock:
        allocations.pop(ptr, None)

lib = c.CDLL(str(pathlib.Path(args.library).resolve()))
lib.cliproxy_plugin_init.argtypes = [c.POINTER(HostAPI), c.POINTER(PluginAPI)]
lib.cliproxy_plugin_init.restype = c.c_int
host = HostAPI(1, None, host_call, host_free)
plugin = PluginAPI()
assert lib.cliproxy_plugin_init(c.byref(host), c.byref(plugin)) == 0
assert plugin.abi_version == 1

def call(method, payload):
    raw = json.dumps(payload).encode()
    request = (c.c_uint8 * len(raw)).from_buffer_copy(raw)
    response = Buffer()
    code = plugin.call(method.encode(), request, len(raw), c.byref(response))
    assert code == 0, method
    result = json.loads(c.string_at(response.ptr, response.len))
    plugin.free_buffer(response.ptr, response.len)
    assert result['ok'], result
    return result['result']

def management(method, path, body=None, query=None):
    payload = {'Method': method, 'Path': path, 'Headers': {}, 'Query': query or {}}
    if body is not None:
        payload['Body'] = base64.b64encode(json.dumps(body).encode()).decode()
    response = call('management.handle', payload)
    raw = base64.b64decode(response['Body'])
    return response['StatusCode'], raw

config = {'enabled': True, 'schedule_enabled': False, 'times': ['07:00', '13:00', '19:00'], 'timezone': 'Asia/Shanghai', 'accounts': ['account-a', 'account-b', 'disabled'], 'model': 'gpt-6-sol', 'state_file': str(state)}
def configure():
    return call('plugin.reconfigure', {'config_yaml': base64.b64encode(json.dumps(config).encode()).decode(), 'schema_version': 6})

def status():
    return json.loads(management('GET', '/v0/management/plugins/cpa-window-starter/status')[1])

def wait_idle(timeout=10):
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        current = status()
        if not current['running']:
            return current
        time.sleep(0.03)
    raise AssertionError('batch did not finish')

try:
    registration = call('plugin.register', {'config_yaml': base64.b64encode(json.dumps(config).encode()).decode(), 'schema_version': 6})
    assert registration['schema_version'] == 6
    assert all(registration['metadata'][key] for key in ('Name', 'Version', 'Author', 'GitHubRepository'))
    assert registration['capabilities'] == {'management_api': True}
    routes = call('management.register', {})
    assert len(routes['routes']) == 4
    code, page = management('GET', '/v0/resource/plugins/cpa-window-starter/status', query={'run': ['1']})
    assert code == 200 and b'data-i18n="daily"' in page
    assert not executions
    code, _ = management('GET', '/v0/resource/plugins/cpa-window-starter/run')
    assert code == 404 and not executions
    code, accounts_raw = management('GET', '/v0/management/plugins/cpa-window-starter/accounts')
    assert code == 200 and len(json.loads(accounts_raw)['accounts']) == 4
    code, _ = management('POST', '/v0/management/plugins/cpa-window-starter/run', {})
    assert code == 202
    current = wait_idle()
    assert [row['auth_id'] for row in executions] == ['account-a', 'account-b'], json.dumps({'executions': executions, 'callback_errors': callback_errors, 'status': current}, ensure_ascii=True)
    assert [row['status'] for row in current['results']] == ['success', 'success', 'skipped']
    assert all(row.get('reset_at') for row in current['results'][:2])
    print('Native ABI: registration, static resource, exact account pinning, manual request, reset headers and disabled-account skip passed.', flush=True)
    if args.schedule:
        now = time.time()
        boundary = (int(now) // 60 + 1) * 60
        if boundary - now < 2:
            boundary += 60
        local = dt.datetime.fromtimestamp(boundary, dt.timezone(dt.timedelta(hours=8)))
        config['schedule_enabled'] = True
        config['times'] = [local.strftime('%H:%M')]
        configure()
        assert status()['config']['times'] == config['times']
        print(f'Native timer: waiting for editable schedule {config["times"][0]} Asia/Shanghai (within {boundary-now:.0f}s).', flush=True)
        deadline = time.monotonic() + (boundary-now) + 12
        while time.monotonic() < deadline:
            if len(executions) == 4:
                break
            time.sleep(0.05)
        current = wait_idle()
        assert len(executions) == 4, executions
        assert [row['auth_id'] for row in executions[-2:]] == ['account-a', 'account-b']
        assert all(row['at'] >= boundary for row in executions[-2:])
        assert len([row for row in current['results'] if row['mode'] == 'scheduled']) == 3
        configure()
        time.sleep(0.15)
        assert len(executions) == 4, 'reconfigure duplicated the schedule'
        print('Native timer: two accounts triggered once at the edited time; reconfigure did not duplicate.', flush=True)
    previous_count = len(executions)
    fail_auth = 'account-a'
    assert management('POST', '/v0/management/plugins/cpa-window-starter/run', {})[0] == 202
    current = wait_idle()
    assert len(executions) == previous_count + 2
    assert current['results'][-3]['status'] == 'failed' and current['results'][-3]['http_status'] == 429
    assert current['results'][-2]['status'] == 'success'
    print('Native ABI: standard host http_status 429 preserved; next account still completed.', flush=True)
    before_gemini = len(executions)
    config.update(antigravity_schedule_enabled=False, antigravity_accounts=['account-g'], antigravity_model='gemini-test', antigravity_timezone='Europe/London', antigravity_times=['08:30'])
    configure()
    assert management('POST', '/v0/management/plugins/cpa-window-starter/run', {'provider':'antigravity'})[0] == 202
    deadline = time.monotonic()+10
    while status()['antigravity']['running'] and time.monotonic()<deadline:
        time.sleep(.03)
    ag=status()['antigravity']
    assert len(executions)==before_gemini+1 and executions[-1]['auth_id']=='account-g'
    assert ag['results'][-1]['status']=='success' and not ag['results'][-1].get('reset_at')
    print('Native ABI: Antigravity request pinned independently; Codex quota headers not reused.', flush=True)
    assert not callback_errors, callback_errors
    report = {'platform': platform.system().lower()+'/'+platform.machine(), 'antigravity_pinning':'passed', 'native_abi': 'passed', 'host_error_http_status': 'passed', 'scheduled_timer': 'passed' if args.schedule else 'not_run', 'mock_model_calls': len(executions), 'real_accounts_used': False}
    (work / 'report.json').write_text(json.dumps(report, ensure_ascii=False, indent=2), encoding='utf-8')
finally:
    call('plugin.quiesce', {})
    plugin.shutdown()
assert not allocations, 'host response buffers leaked'
