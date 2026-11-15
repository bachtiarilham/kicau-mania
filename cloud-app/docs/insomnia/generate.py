"""Generate the Insomnia v4 collection from the mounted-route Swagger contract."""
import copy
import hashlib
import json
import re
from pathlib import Path

ROOT = Path(__file__).resolve().parent
SPEC = json.loads((ROOT.parent / 'generated/swagger.json').read_text(encoding='utf-8'))
WORKSPACE = 'wrk_kicau_schema010'
ENV = 'env_kicau_schema010'
resources = [dict(_id=WORKSPACE, _type='workspace', parentId=None,
                  name='Kicau Mania - Schema 010', scope='collection',
                  description='API nyata schema 010. Baca README.md; tidak bergantung seed/mock lama. Jangan Run All: transisi, logout, dan penghapusan adalah tindakan nyata.')]
variables = dict(base_url='https://dev.megaallstararena.com', frontend_origin='https://dev.megaallstararena.com',
                 email='', password='', access_token='', my_user_id='',
                 device_id='insomnia-kicau-admin-device', staff_device_id='insomnia-kicau-juri-device',
                 staff_email='', staff_phone='', staff_password='', staff_user_id='', staff_role='JURI',
                 user_id='', owner_id='', new_owner_id='', arena_id='', other_arena_id='',
                 bird_id='', species_id='', event_id='', category_id='', round_id='',
                 gantangan_id='', criterion_id='', criterion_2_id='', registration_id='',
                 award_id='', session_id='', user_version=1, bird_version=1,
                 cursor='', search='', history_after_version=0,
                 event_date='2026-11-15', new_password='')
resources += [dict(_id=ENV, _type='environment', parentId=WORKSPACE, name='Base Environment',
                   data=variables, dataPropertyOrder=None, color=None, isPrivate=False, metaSortKey=0),
              dict(_id='jar_kicau_schema010', _type='cookie_jar', parentId=WORKSPACE,
                   name='Cookie Jar', cookies=[])]
groups = {}


def ref(key):
    return '{{ _.%s }}' % key


def folder(label):
    if label not in groups:
        key = 'fld_' + hashlib.sha256(label.encode()).hexdigest()[:20]
        groups[label] = key
        resources.append(dict(_id=key, _type='request_group', parentId=WORKSPACE,
                              name=label, description='Lihat README untuk prasyarat dan urutan.',
                              environment={}, metaSortKey=int(label.split()[0])))
    return groups[label]


def group(path, method):
    if method == 'DELETE': return '99 Penghapusan dan pencabutan - jalankan manual'
    if '/auth/' in path: return '01 Autentikasi dan sesi'
    if path.startswith('/api/v1/users'): return '03 Master peserta dan EO'
    if path.startswith('/api/v1/bird-species'): return '04 Spesies burung'
    if '/awards' in path: return '14 Penghargaan dan sertifikat'
    if path.startswith('/api/v1/birds'): return '05 Burung, transfer dan history'
    if '/staff' in path: return '02 Akses petugas'
    if '/scores/' in path or path.startswith('/api/v1/judge/'): return '13 Scoring juri'
    if '/registrations' in path: return '12 Registrasi, pembayaran dan diskualifikasi'
    for segment, label in [('events', '07 Event lomba'), ('categories', '08 Kategori lomba'),
                           ('gantangan', '09 Gantangan'), ('criteria', '10 Kriteria penilaian'),
                           ('rounds', '11 Ronde, roster dan rekap')]:
        if '/' + segment in path: return label
    return '06 Arena'


def example(name):
    return {
        'Login': dict(email=ref('email'), password=ref('password'), device_id=ref('device_id')),
        'Participant': dict(name='Peserta uji', phone='', email=None, is_eo=False, is_active=True),
        'Bird': dict(name='Burung uji', owner_id=ref('user_id'), bird_species_id=ref('species_id')),
        'Arena': dict(name='Arena uji', address='Alamat arena', timezone='Asia/Jakarta', is_active=True),
        'Species': dict(name='Spesies uji'),
        'Gantangan': dict(number=1, description='Gantangan uji', baris=1, kolom=1),
        'Event': dict(owner_id=ref('owner_id'), title='Lomba uji', description='Uji operasional',
                      event_date=ref('event_date'), status='SCHEDULED'),
        'Category': dict(code='UJI', name='Kategori uji'),
        'Round': dict(event_id=ref('event_id'), category_id=ref('category_id'),
                      bird_species_id=ref('species_id'), round_number=1, title='Ronde uji'),
        'Criterion': dict(criteria_name='Irama'),
        'Registration': dict(user_id=ref('user_id'), arena_id=ref('arena_id'), round_id=ref('round_id'),
                             bird_id=ref('bird_id'), gantangan_id=ref('gantangan_id'),
                             entry_fee='150000.00', payment_method='CASH'),
        'Transfer': dict(owner_id=ref('new_owner_id'), notes='Transfer kepemilikan',
                         expected_version='__NUM_bird_version__'),
        'Award': dict(registration_id=None, award_name='Penghargaan eksternal', event_name='Event eksternal',
                      organizer_name='Penyelenggara eksternal', rank=1,
                      awarded_at=ref('event_date'), certificate_url=None),
        'Scores': dict(scores=[dict(criteria_id=ref('criterion_id'), score_value='9.25'),
                              dict(criteria_id=ref('criterion_2_id'), score_value='8.75')]),
        'Password': dict(old_password=ref('password'), new_password=ref('new_password')),
        'Credentials': dict(password=ref('staff_password')),
        'Transition': dict(expected_status='PREPARED'),
        'TicketAction': dict(reason='')
    }.get(name, {})


def capture_for(path, method):
    if method == 'POST':
        return {'/api/v1/users': 'user_id', '/api/v1/birds': 'bird_id',
                '/api/v1/bird-species': 'species_id', '/api/v1/arenas': 'arena_id',
                '/api/v1/arenas/{arena}/events': 'event_id',
                '/api/v1/arenas/{arena}/categories': 'category_id',
                '/api/v1/arenas/{arena}/gantangan': 'gantangan_id',
                '/api/v1/arenas/{arena}/criteria': 'criterion_id',
                '/api/v1/arenas/{arena}/rounds': 'round_id',
                '/api/v1/registrations': 'registration_id',
                '/api/v1/birds/{id}/awards': 'award_id'}.get(path)


def script(capture=None, version=None, token=False, binary=False):
    out = "const ok = insomnia.response.code === 200;\ninsomnia.test('HTTP 200', () => insomnia.expect(insomnia.response.code).to.eql(200));\n"
    if binary: return out
    out += "if (ok) {\n const body = insomnia.response.json();\n insomnia.test('Envelope success', () => insomnia.expect(body.success).to.eql(true));\n if (body.success === true) {\n"
    if token:
        out += "  if (typeof body.data?.access_token === 'string' && body.data.access_token) insomnia.environment.set('access_token', body.data.access_token);\n  if (body.data?.user?.id) insomnia.environment.set('my_user_id', body.data.user.id);\n"
    if capture:
        out += f"  if (body.data?.id) insomnia.environment.set('{capture}', body.data.id);\n"
    if version:
        out += f"  if (Number.isInteger(body.data?.version)) insomnia.environment.set('{version}', body.data.version);\n"
    return out + ' }\n}\n'


def add(path, method, op, title=None, payload=None, capture=None, suffix=''):
    method = method.upper()
    ids = dict(arena='arena_id', judge='staff_user_id', ticket='registration_id',
               user='staff_user_id', role='staff_role', session='session_id', award='award_id')
    for segment, key in [('users', 'user_id'), ('bird-species', 'species_id'), ('birds', 'bird_id'),
                          ('events', 'event_id'), ('categories', 'category_id'), ('rounds', 'round_id'),
                          ('gantangan', 'gantangan_id'), ('criteria', 'criterion_id'),
                          ('registrations', 'registration_id')]:
        if f'/{segment}/{{id}}' in path: ids['id'] = key
    url = ref('base_url') + re.sub(r'\{(\w+)\}', lambda m: ref(ids[m[1]]), path)
    headers = [dict(name='Accept', value='image/jpeg, image/png' if path.endswith('/proof') and method == 'GET' else 'application/json')]
    public = path in ['/api/v1/auth/login', '/api/v1/auth/refresh', '/api/v1/auth/logout']
    if not public: headers.append(dict(name='Authorization', value='Bearer ' + ref('access_token')))
    if public: headers.append(dict(name='Origin', value=ref('frontend_origin')))
    body = {}
    parameters = []
    body_param = next((p for p in op.get('parameters', []) if p['in'] == 'body'), None)
    if body_param:
        data = example(body_param['schema'].get('$ref', '').split('/')[-1]) if payload is None else payload
        if '/users/{id}' in path or '/birds/{id}' in path and method in ['PATCH', 'PUT']:
            data['expected_version'] = '__NUM_' + ('user_version' if '/users/' in path else 'bird_version') + '__'
        if '/birds/{id}' == path.removeprefix('/api/v1') and method in ['PATCH', 'PUT']: data.pop('owner_id', None)
        if path.endswith('/begin-judging'): data = dict(expected_status='RUNNING')
        if path.endswith('/close'): data = dict(expected_status='JUDGING')
        if path.endswith('/disqualify'): data = dict(reason='Melanggar aturan ronde')
        text = json.dumps(data, ensure_ascii=True, indent=2)
        text = re.sub(r'"__NUM_(\w+)__"', lambda m: ref(m[1]), text)
        body = dict(mimeType='application/json', text=text)
        headers.append(dict(name='Content-Type', value='application/json'))
    if method == 'POST' and path.endswith('/proof'):
        body = dict(mimeType='multipart/form-data', params=[dict(id='file_proof', name='file', type='file', fileName='', disabled=False)])
    for p in op.get('parameters', []):
        if p['in'] != 'query': continue
        key = p['name']; value = ref(key) if key in variables else ''
        if key == 'limit': value = '25'
        if key == 'after_version': value = ref('history_after_version')
        if key == 'q': value = ref('search')
        if key == 'expected_version': value = ref('user_version' if '/users/' in path else 'bird_version')
        parameters.append(dict(id='param_' + key, name=key, value=value,
                               disabled=not (p.get('required') or key in ['limit', 'after_version'] or key == 'arena_id')))
    capture = capture or capture_for(path, method)
    version = ('user_version' if path in ['/api/v1/users', '/api/v1/users/{id}'] else
               'bird_version' if path in ['/api/v1/birds', '/api/v1/birds/{id}', '/api/v1/birds/{id}/transfer'] else None)
    key = 'req_' + hashlib.sha256((method + path + suffix).encode()).hexdigest()[:24]
    request = dict(_id=key, _type='request', parentId=folder(group(path, method)),
                   name=title or method + ' ' + path.removeprefix('/api/v1'),
                   description='Tindakan nyata. Expected HTTP 200; kegagalan tidak memperbarui ID/versi/token environment. '
                               'Sesuaikan kolom kosong, ID dan state sesuai README.',
                   url=url, method=method, headers=headers, body=body, parameters=parameters,
                   authentication={}, metaSortKey=len(resources), isPrivate=False,
                   settingStoreCookies=True, settingSendCookies=True, settingEncodeUrl=True,
                   settingRebuildPath=True, settingFollowRedirects='off', preRequestScript='',
                   afterResponseScript=script(capture, version, path in ['/api/v1/auth/login', '/api/v1/auth/refresh'],
                                              path.endswith('/proof') and method == 'GET'))
    if path == '/api/v1/auth/logout':
        request['afterResponseScript'] += "if (ok) insomnia.environment.unset('access_token');\n"
    resources.append(request)
    return request


for path, operations in SPEC['paths'].items():
    for method in ['post', 'get', 'patch', 'put', 'delete']:
        if method in operations: add(path, method, operations[method])

user_op = SPEC['paths']['/api/v1/users']['post']
for title, payload, capture, suffix in [
    ('Buat akun EO (simpan owner_id)', dict(name='EO uji', phone='', email=None, is_eo=True, is_active=True), 'owner_id', 'eo'),
    ('Buat akun petugas (sebelum role/password)', dict(name='Juri uji', phone=ref('staff_phone'), email=ref('staff_email'), is_eo=False, is_active=True), 'staff_user_id', 'staff')
]:
    r = add('/api/v1/users', 'POST', user_op, title, payload, capture, suffix)
    r['afterResponseScript'] = script(capture)
add('/api/v1/auth/login', 'POST', SPEC['paths']['/api/v1/auth/login']['post'],
    'Login JURI (ganti token dan cookie aktif)',
    dict(email=ref('staff_email'), password=ref('staff_password'), device_id=ref('staff_device_id')), suffix='judge')
r = add('/api/v1/arenas/{arena}/criteria', 'POST', SPEC['paths']['/api/v1/arenas/{arena}/criteria']['post'],
        'Buat kriteria kedua (simpan criterion_2_id)', dict(criteria_name='Volume'), 'criterion_2_id', 'second')
add('/api/v1/birds/{id}/awards', 'POST', SPEC['paths']['/api/v1/birds/{id}/awards']['post'],
    'Tambah penghargaan internal dari tiket',
    dict(registration_id=ref('registration_id'), award_name='Juara lomba', rank=1,
         awarded_at=ref('event_date'), certificate_url=None), suffix='internal')
for path in ['/healthz', '/readyz']:
    r = add(path, 'GET', {}, title='GET ' + path)
    r['parentId'] = folder('00 Health dan readiness')
    r['headers'] = [h for h in r['headers'] if h['name'] != 'Authorization']

export = dict(_type='export', __export_format=4, __export_date='2026-11-15T00:00:00.000Z',
              __export_source='kicau-mania.schema010', resources=resources)
(ROOT / 'cloud-app-mock.json').write_text(json.dumps(export, ensure_ascii=True, indent=2) + '\n', encoding='utf-8')
print('Generated', sum(r['_type'] == 'request' for r in resources), 'requests.')
