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
                  name='Kicau Mania - VPS Dev - Schema 010', scope='collection',
                  description='API schema 010 plus skenario mock berurutan. Terapkan seed 010 secara eksplisit. Jangan Run All: katalog manual berisi mutasi/penghapusan nyata.')]
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
def fixture(group, number):
    return f'6d6f636b-{group:04x}-4000-8000-{number:012d}'

variables.update(engine_id=fixture(15,1),engine_token='',media_upload_id='',media_recording_id='',media_session_id='',channel_id='gantangan01')
variables.update(email='admin@mock.kicau.test', password='KicauMock2026!',
                 user_id=fixture(1,7), owner_id=fixture(1,2), new_owner_id=fixture(1,8),
                 arena_id=fixture(2,1), other_arena_id=fixture(2,2), species_id=fixture(14,1),
                 bird_id=fixture(9,1), event_id=fixture(4,1), category_id=fixture(5,1),
                 round_id=fixture(6,1), gantangan_id=fixture(3,1), criterion_id=fixture(7,1),
                 criterion_2_id=fixture(7,2), criterion_3_id=fixture(7,3),
                 staff_email='juri.1@mock.kicau.test',staff_password='KicauMock2026!',
                 staff_user_id=fixture(1,5), mock_registration_1_id='', mock_registration_2_id='')
mock_roles = [
    ('admin',1,'admin@mock.kicau.test','ADMIN'),
    ('admin_a',2,'admin.a@mock.kicau.test','ADMIN'),
    ('operator_a',3,'operator.a@mock.kicau.test','OPERATOR'),
    ('operator_b',4,'operator.b@mock.kicau.test','OPERATOR'),
    ('juri_1',5,'juri.1@mock.kicau.test','JURI'),
    ('juri_2',6,'juri.2@mock.kicau.test','JURI'),
    ('peserta_1',7,'peserta.1@mock.kicau.test','PESERTA'),
    ('peserta_2',8,'peserta.2@mock.kicau.test','PESERTA')]
for role,number,email,_ in mock_roles:
    variables.update({f'mock_{role}_email':email,f'mock_{role}_id':fixture(1,number),f'token_{role}':''})
resources += [dict(_id=ENV, _type='environment', parentId=WORKSPACE, name='Base Environment',
                   data=variables, dataPropertyOrder=None, color=None, isPrivate=False, metaSortKey=0),
              dict(_id='jar_kicau_schema010', _type='cookie_jar', parentId=WORKSPACE,
                   name='Cookie Jar', cookies=[])]
resources += [dict(_id='env_kicau_schema010_local',_type='environment',parentId=ENV,
                   name='VPS Development',data=dict(base_url='https://dev.megaallstararena.com',frontend_origin='https://dev.megaallstararena.com'),
                   dataPropertyOrder=None,color=None,isPrivate=False,metaSortKey=1)]
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
    if '/engine/media/' in path or '/media/' in path: return '15 Media desktop dan upload privat'
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
                              dict(criteria_id=ref('criterion_2_id'), score_value='8.75'),
                              dict(criteria_id=ref('criterion_3_id'), score_value='8.50')]),
        'MediaPair': dict(arena_id=ref('arena_id')),
        'MediaIngestion': dict(engine_id=ref('engine_id'),session_id=ref('media_session_id'),round_id=ref('round_id'),mappings=[dict(channel_id=ref('channel_id'),registration_id=ref('registration_id'),gantangan_id=ref('gantangan_id'))]),
        'MediaUpload': dict(container='mkv',session_id=ref('media_session_id'),channel_id=ref('channel_id'),recording_id='local-recording-id',size_bytes=1,sha256='REPLACE_WITH_FILE_SHA256_HEX',started_at='2026-11-15T00:00:00Z',ended_at='2026-11-15T00:01:00Z'),
        'MediaPart': dict(number=1,sha256='REPLACE_WITH_PART_SHA256_BASE64'),
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
               user='staff_user_id', role='staff_role', session='session_id', award='award_id',engine_id='engine_id')
    for segment, key in [('users', 'user_id'), ('bird-species', 'species_id'), ('birds', 'bird_id'),
                          ('events', 'event_id'), ('categories', 'category_id'), ('rounds', 'round_id'),
                          ('gantangan', 'gantangan_id'), ('criteria', 'criterion_id'),
                          ('registrations', 'registration_id')]:
        if f'/{segment}/{{id}}' in path: ids['id'] = key
    if '/engine/media/uploads/{id}' in path: ids['id']='media_upload_id'
    if '/media/recordings/{id}' in path: ids['id']='media_recording_id'
    url = ref('base_url') + re.sub(r'\{(\w+)\}', lambda m: ref(ids[m[1]]), path)
    headers = [dict(name='Accept', value='image/jpeg, image/png' if path.endswith('/proof') and method == 'GET' else 'application/json')]
    public = path in ['/api/v1/auth/login', '/api/v1/auth/refresh', '/api/v1/auth/logout']
    if not public: headers.append(dict(name='Authorization', value='Bearer ' + ref('engine_token' if '/engine/media/' in path else 'access_token')))
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
    if path=='/api/v1/engine/media/uploads' and method=='POST': capture='media_upload_id'
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
    if path == '/api/v1/media/engines/{engine_id}' and method=='POST':
        request['afterResponseScript'] += "if (ok) {const d=insomnia.response.json().data;if(d?.token)insomnia.environment.set('engine_token',d.token);if(d?.id)insomnia.environment.set('engine_id',d.id); }\n"
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

def mock_request(title, path, method='GET', values=None, payload=None, actor='admin',
                 status=200, capture=None, login=None):
    """Append one ordered scenario request, without overwriting catalogue variables."""
    op = {} if path in ('/healthz','/readyz') else SPEC['paths'][path][method.lower()]
    r = add(path, method, op, title=title, payload=payload, suffix='mock-'+title)
    r['parentId'] = folder('00 Mock - jalankan berurutan setelah seed 010')
    r['url'] = ref('base_url') + re.sub(r'\{(\w+)\}', lambda m: str((values or {})[m[1]]), path)
    r['parameters'] = []
    if any(p.get('name')=='arena_id' and p['in']=='query' for p in op.get('parameters',[])):
        r['parameters'].append(dict(name='arena_id',value=fixture(2,1),disabled=False))
    r['headers'] = [dict(name='Accept',value='application/json')]
    if actor is not None:
        r['headers'].append(dict(name='Authorization',value='Bearer '+ref('token_'+actor)))
    if payload is not None:
        r['headers'].append(dict(name='Content-Type',value='application/json'))
    if login:
        r['headers'].append(dict(name='Origin',value=ref('frontend_origin')))
    # Keep request-specific overrides explicit rather than reusing generic body defaults.
    if payload is not None:
        r['body']=dict(mimeType='application/json',text=json.dumps(payload,indent=2))
    else:r['body']={}
    js = f"const expectedStatus = {status};\nconst code = insomnia.response.code;\n"
    js += "insomnia.test('Expected HTTP '+expectedStatus, () => insomnia.expect(code).to.eql(expectedStatus));\n"
    if login:
        role,_,_,business_role=login
        js += f"insomnia.environment.unset('token_{role}');\n"
    js += "if (code === expectedStatus) {\n const body = insomnia.response.json();\n"
    js += f" insomnia.test('Envelope', () => insomnia.expect(body.success).to.eql({'true' if status<400 else 'false'}));\n"
    if status<400:
        js += " if (body.success === true) {\n"
        if login:
            role,_,_,business_role=login
            js += f"  const user=body.data?.user;\n  const identityOK=user?.id === insomnia.environment.get('mock_{role}_id') && user?.email === insomnia.environment.get('mock_{role}_email');\n"
            js += f"  const roles=[...(Array.isArray(user?.roles)?user.roles:[]),...(Array.isArray(user?.arena_roles)?user.arena_roles.map(x=>x.role):[])];\n  const roleOK=roles.includes('{business_role}');\n"
            js += "  const token=body.data?.access_token;\n  insomnia.test('Correct fixture identity and role', () => insomnia.expect(identityOK && roleOK && typeof token==='string' && token.length>0).to.eql(true));\n"
            js += f"  if(identityOK && roleOK && typeof token==='string' && token.length>0) insomnia.environment.set('token_{role}',token);\n"
            if role=='admin':js += "  if(identityOK && roleOK && typeof token==='string' && token.length>0) { insomnia.environment.set('access_token',token); insomnia.environment.set('my_user_id',user.id); }\n"
        if capture:
            js += f"  if (typeof body.data?.id==='string') insomnia.environment.set('{capture}',body.data.id);\n"
        js += " }\n"
    js += '}\n'
    r['afterResponseScript']=js
    r['description']='Skenario mock schema 010. Jalankan sesuai nomor setelah seed. Token terpisah per role; cookie refresh tetap satu jar. Expected HTTP '+str(status)+'.'
    return r

mock_request('01 Health','/healthz',actor=None)
mock_request('02 Readiness','/readyz',actor=None)
for i,role in enumerate(mock_roles,3):
    key,_,_,_=role
    mock_request(f'{i:02d} Login {key}','/api/v1/auth/login','POST',
                 payload=dict(email=ref('mock_'+key+'_email'),password=ref('password'),device_id='insomnia-mock-'+key),actor=None,login=role)
mock_request('11 Profil ADMIN','/api/v1/auth/me')
mock_request('12 Daftar arena','/api/v1/arenas')
mock_request('13 Tanpa token ditolak','/api/v1/auth/me',actor=None,status=401)
mock_request('14 Operator A tidak boleh arena B','/api/v1/arenas/{arena}',values=dict(arena=fixture(2,2)),actor='operator_a',status=403)
mock_request('15 Detail burung peserta 1','/api/v1/birds/{id}',values=dict(id=fixture(9,1)))
mock_request('16 History master burung','/api/v1/birds/{id}/history',values=dict(id=fixture(9,1)))
for num,participant,bird,gantangan in [(17,7,1,1),(18,8,3,2)]:
    mock_request(f'{num:02d} Daftarkan peserta {num-16}','/api/v1/registrations','POST',
                 payload=dict(arena_id=fixture(2,1),user_id=fixture(1,participant),round_id=fixture(6,1),
                              bird_id=fixture(9,bird),gantangan_id=fixture(3,gantangan),entry_fee='50000.00',payment_method='CASH'),
                 actor='operator_a',capture=f'mock_registration_{num-16}_id')
for num,ticket in [(19,1),(20,2)]:
    mock_request(f'{num:02d} Verifikasi pembayaran peserta {ticket}','/api/v1/registrations/{id}/verify','POST',
                 values=dict(id=ref(f'mock_registration_{ticket}_id')),payload={},actor='operator_a')
base='/api/v1/arenas/{arena}/rounds/{id}'
rv=dict(arena=fixture(2,1),id=fixture(6,1))
for num,judge in [(21,5),(22,6)]:
    mock_request(f'{num:02d} Roster juri {judge-4}',base+'/judges/{judge}','POST',values={**rv,'judge':fixture(1,judge)},actor='operator_a')
mock_request('23 Mulai ronde',base+'/start','POST',values=rv,payload=dict(expected_status='PREPARED'),actor='operator_a')
mock_request('24 Mulai penjurian',base+'/begin-judging','POST',values=rv,payload=dict(expected_status='RUNNING'),actor='operator_a')
mock_request('25 Tutup sebelum lengkap ditolak',base+'/close','POST',values=rv,payload=dict(expected_status='JUDGING'),actor='operator_a',status=409)
for num,ticket,judge,score in [(26,1,'juri_1','8.50'),(27,1,'juri_2','8.00'),(28,2,'juri_1','7.00'),(29,2,'juri_2','7.00')]:
    mock_request(f'{num:02d} Nilai peserta {ticket} oleh {judge}',base+'/scores/{ticket}','PUT',
                 values={**rv,'ticket':ref(f'mock_registration_{ticket}_id')},actor=judge,
                 payload=dict(scores=[dict(criteria_id=fixture(7,k),score_value=score) for k in [1,2,3]]))
mock_request('30 Revisi nilai peserta 1',base+'/scores/{ticket}','PUT',values={**rv,'ticket':ref('mock_registration_1_id')},actor='juri_1',
             payload=dict(scores=[dict(criteria_id=fixture(7,k),score_value=v) for k,v in [(1,'9.25'),(2,'8.50'),(3,'8.50')]]))
mock_request('31 Rekap langsung',base+'/recap',values=rv,actor='operator_a')
mock_request('32 Diskualifikasi peserta 2','/api/v1/registrations/{id}/disqualify','POST',
             values=dict(id=ref('mock_registration_2_id')),payload=dict(reason='Mock pelanggaran ronde'),actor='operator_a')
mock_request('33 Skor peserta diskualifikasi ditolak',base+'/scores/{ticket}','PUT',values={**rv,'ticket':ref('mock_registration_2_id')},actor='juri_1',status=409,
             payload=dict(scores=[dict(criteria_id=fixture(7,k),score_value='8.00') for k in [1,2,3]]))
mock_request('34 Aktifkan kembali peserta 2','/api/v1/registrations/{id}/requalify','POST',values=dict(id=ref('mock_registration_2_id')),payload={},actor='operator_a')
mock_request('35 Peserta membaca tiket sendiri','/api/v1/registrations/{id}',values=dict(id=ref('mock_registration_1_id')),actor='peserta_1')
mock_request('36 Peserta tidak boleh tiket orang lain','/api/v1/registrations/{id}',values=dict(id=ref('mock_registration_2_id')),actor='peserta_1',status=403)
mock_request('37 Tutup ronde lengkap',base+'/close','POST',values=rv,payload=dict(expected_status='JUDGING'),actor='operator_a')
mock_request('38 Revisi setelah CLOSED ditolak',base+'/scores/{ticket}','PUT',values={**rv,'ticket':ref('mock_registration_1_id')},actor='juri_1',status=409,
             payload=dict(scores=[dict(criteria_id=fixture(7,k),score_value='9.00') for k in [1,2,3]]))
mock_request('39 Rekap akhir',base+'/recap',values=rv,actor='operator_a')

export = dict(_type='export', __export_format=4, __export_date='2026-11-15T00:00:00.000Z',
              __export_source='kicau-mania.schema010', resources=resources)
(ROOT / 'cloud-app-mock.json').write_text(json.dumps(export, ensure_ascii=True, indent=2) + '\n', encoding='utf-8')
print('Generated', sum(r['_type'] == 'request' for r in resources), 'requests.')
