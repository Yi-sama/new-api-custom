"""Deploy only the web-image size policy over the current Studio release."""

import hashlib
import json
import os
from pathlib import Path
import shlex
import subprocess
import sys
import time


ROOT = Path(__file__).resolve().parent.parent
SOURCES = ROOT / 'sources'
CONTAINER = 'image-studio-open-webui-1'
HELPER = '/app/backend/open_webui/open_webui_bridge.py'
ROUTER = '/app/backend/open_webui/routers/images.py'
EXPECTED_HELPER = '9a4808e337ce93c13f25dd9a7f042aaba8b17f7ec79255025187813aabd0baba'
EXPECTED_ROUTER = 'e0062a303d3fb2f949d9e53244798d7d8b7b500fb8222cc4e7ffd4b7feae0b0e'
OTHER_CONTAINERS = (
    'new-api', 'sub2api', 'chatgpt2api-trial',
    'codex-proxy-rs-codex-proxy-rs-1', 'cpr-image-test-gateway-1',
)
UP = ['up', '-d', '--no-deps', '--pull', 'never', '--no-build', 'open-webui']


def run(args):
    return subprocess.run(args, capture_output=True, text=True, check=True, timeout=90).stdout


def inspect(name):
    return json.loads(run(['docker', 'inspect', name]))[0]


def snapshots():
    return {
        name: {field: item[field] for field in ('Id', 'Image', 'RestartCount')}
        | {'started_at': item['State']['StartedAt']}
        for name in OTHER_CONTAINERS
        for item in (inspect(name),)
    }


def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def prepare():
    if os.uname().nodename != 'ser021360749320':
        raise RuntimeError('Wrong server')
    webui = inspect(CONTAINER)
    if webui['State'].get('Health', {}).get('Status') != 'healthy':
        raise RuntimeError('Studio is not healthy')
    available = next(int(line.split()[1]) for line in Path('/proc/meminfo').read_text().splitlines()
                     if line.startswith('MemAvailable:'))
    if available < 1_000_000:
        raise RuntimeError('Insufficient available memory')
    files = webui['Config']['Labels']['com.docker.compose.project.config_files'].split(',')
    compose = ['docker', 'compose', '-p', 'image-studio', '--env-file', '/opt/image-studio/.env']
    for file in files:
        compose.extend(['-f', file])
    backup = ROOT / 'before'
    actual = run(['docker', 'exec', CONTAINER, 'sha256sum', HELPER, ROUTER]).splitlines()
    if [line.split()[0] for line in actual] != [EXPECTED_HELPER, EXPECTED_ROUTER]:
        raise RuntimeError('Live source changed; inspect it before preparing')
    if backup.exists():
        previous = json.loads((ROOT / 'manifest.json').read_text())
        if previous['state'] != 'prepared' or files != list(previous['config_hashes']):
            raise RuntimeError('Existing preparation is not the same rolled-back release')
    else:
        backup.mkdir()
        run(['docker', 'cp', f'{CONTAINER}:{HELPER}', str(backup / 'open_webui_bridge.py')])
        run(['docker', 'cp', f'{CONTAINER}:{ROUTER}', str(backup / 'images.py')])
    if (digest(backup / 'open_webui_bridge.py') != EXPECTED_HELPER
            or digest(backup / 'images.py') != EXPECTED_ROUTER):
        raise RuntimeError('Live source changed; inspect it before deploying')
    router = (backup / 'images.py').read_text()
    old = "    if bridge_enabled() and image_config.IMAGE_GENERATION_ENGINE == 'openai' and form_data.size:\n"
    new = "    if bridge_enabled() and image_config.IMAGE_GENERATION_ENGINE == 'openai':\n"
    if router.count(old) != 1:
        raise RuntimeError('Expected exactly one size-validation guard')
    (SOURCES / 'images.py').write_text(router.replace(old, new), encoding='utf-8')
    for path in (SOURCES / 'open_webui_bridge.py', SOURCES / 'images.py'):
        path.chmod(0o644)
    mounts = [
        {'type': 'bind', 'source': str(SOURCES / 'open_webui_bridge.py'), 'target': HELPER, 'read_only': True},
        {'type': 'bind', 'source': str(SOURCES / 'images.py'), 'target': ROUTER, 'read_only': True},
    ]
    override = ROOT / 'backend.override.json'
    override.write_text(json.dumps({'services': {'open-webui': {'volumes': mounts}}}, indent=2) + '\n')
    candidate = compose + ['-f', str(override)]
    before = json.loads(run(compose + ['config', '--format', 'json']))
    after = json.loads(run(candidate + ['config', '--format', 'json']))
    after['services']['open-webui']['volumes'] = [
        v for v in after['services']['open-webui']['volumes'] if v['target'] not in (HELPER, ROUTER)
    ]
    if before != after:
        raise RuntimeError('Unexpected Compose changes')
    (backup / 'compose.json').write_text(json.dumps(before, indent=2) + '\n')
    manifest = {
        'state': 'prepared', 'container_id': webui['Id'], 'image': webui['Image'],
        'old_compose': compose, 'new_compose': candidate,
        'config_hashes': {file: digest(Path(file)) for file in files},
        'env_hash': digest(Path('/opt/image-studio/.env')),
        'source_hashes': {str(p): digest(p) for p in (SOURCES / 'open_webui_bridge.py', SOURCES / 'images.py')},
        'frontend': [m for m in webui['Mounts'] if m['Destination'] == '/app/build'],
        'other_containers': snapshots(),
    }
    (ROOT / 'manifest.json').write_text(json.dumps(manifest, indent=2) + '\n')
    (backup / 'rollback.sh').write_text('#!/bin/sh\nset -eu\n' + shlex.join(compose + UP) + '\n')
    (backup / 'rollback.sh').chmod(0o700)
    print('PREPARED: exact source backup; current frontend and all other configuration preserved')


def activate():
    manifest = json.loads((ROOT / 'manifest.json').read_text())
    if manifest['state'] != 'prepared' or inspect(CONTAINER)['Id'] != manifest['container_id']:
        raise RuntimeError('Studio changed after preparation')
    for file, sha in (manifest['config_hashes'] | manifest['source_hashes']).items():
        if digest(Path(file)) != sha:
            raise RuntimeError('Prepared file changed: ' + file)
    if digest(Path('/opt/image-studio/.env')) != manifest['env_hash']:
        raise RuntimeError('Studio environment changed')
    try:
        run(manifest['new_compose'] + UP)
        deadline = time.monotonic() + 360
        while time.monotonic() < deadline:
            webui = inspect(CONTAINER)
            if webui['State'].get('Health', {}).get('Status') == 'healthy':
                break
            if not webui['State']['Running']:
                raise RuntimeError('Studio stopped during startup')
            time.sleep(3)
        else:
            raise RuntimeError('Studio health deadline exceeded')
        if (webui['Image'] != manifest['image']
                or [m for m in webui['Mounts'] if m['Destination'] == '/app/build'] != manifest['frontend']
                or snapshots() != manifest['other_containers']):
            raise RuntimeError('Image, frontend, or another service changed')
        code = (
            'from open_webui.open_webui_bridge import bridge_image_generation_size as r, bridge_image_size_options as o; '
            "assert r('gpt-image-2.5-web', {'image_generation_size':'auto'}) == '512x512'; "
            "assert r('gpt-image-2.5-web', aspect_ratio='1:1') == '512x512'; "
            "assert o('gpt-image-2.5-web') == [{'value':'1:1','size':'512x512'}]; "
            "print('LIVE_POLICY_OK')"
        )
        print(run(['docker', 'exec', CONTAINER, 'python3', '-B', '-c', code]).strip())
        manifest['state'] = 'active'
        (ROOT / 'manifest.json').write_text(json.dumps(manifest, indent=2) + '\n')
        print('ACTIVE: Studio healthy; same image and frontend; other services unchanged')
    except Exception:
        run(manifest['old_compose'] + UP)
        print('ROLLED_BACK: restored the exact previous Compose chain')
        raise


if __name__ == '__main__':
    os.umask(0o077)
    if sys.argv[1:] == ['prepare']:
        prepare()
    elif sys.argv[1:] == ['activate']:
        activate()
    else:
        raise SystemExit('Usage: deploy_web_image_size.py prepare|activate')
