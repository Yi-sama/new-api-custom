from __future__ import annotations

import ast
import asyncio
import importlib.util
import os
import tempfile
import unittest
from pathlib import Path
from types import SimpleNamespace


HERE = Path(__file__).resolve().parent


def load_module(name: str, path: Path):
    spec = importlib.util.spec_from_file_location(name, path)
    module = importlib.util.module_from_spec(spec)
    assert spec and spec.loader
    spec.loader.exec_module(module)
    return module


def load_patched_functions(source: str, names: set[str], globals_dict: dict):
    tree = ast.parse(source)
    nodes = [
        node
        for node in tree.body
        if isinstance(node, (ast.FunctionDef, ast.AsyncFunctionDef)) and node.name in names
    ]
    assert {node.name for node in nodes} == names
    for node in nodes:
        node.decorator_list = []
    module = ast.Module(body=nodes, type_ignores=[])
    ast.fix_missing_locations(module)
    namespace = dict(globals_dict)
    exec(compile(module, '<patched-fixture>', 'exec'), namespace)
    return namespace


bridge = load_module('open_webui_bridge_test', HERE / 'open_webui_bridge.py')
patcher = load_module('patch_open_webui_test', HERE / 'patch_open_webui.py')


class Request:
    def __init__(self, cookies=None, headers=None):
        self.cookies = cookies or {}
        self.headers = headers or {}


class BridgeHelperTests(unittest.TestCase):
    def setUp(self):
        self.old_origin = os.environ.get('NEW_API_STUDIO_ORIGIN')
        os.environ['NEW_API_STUDIO_ORIGIN'] = 'http://host.docker.internal:3000'

    def tearDown(self):
        if self.old_origin is None:
            os.environ.pop('NEW_API_STUDIO_ORIGIN', None)
        else:
            os.environ['NEW_API_STUDIO_ORIGIN'] = self.old_origin

    def test_missing_bridge_cookie_fails_closed(self):
        with self.assertRaises(bridge.HTTPException) as raised:
            bridge.bridge_headers_and_cookies(Request(), SimpleNamespace(email='user@example.test'))
        self.assertEqual(raised.exception.status_code, 401)
        self.assertIn('sign in again', raised.exception.detail)

    def test_headers_are_limited_to_cookie_group_and_expected_identity(self):
        request = Request(
            cookies={'new_api_image_studio': 'session', 'token': 'native', 'other': 'blocked'},
            headers={
                'X-NewAPI-Group': '%E4%B8%AD%E6%96%87',
                'X-NewAPI-Identity': 'attacker@example.test',
                'Authorization': 'blocked',
            },
        )
        headers, cookies = bridge.bridge_headers_and_cookies(request, SimpleNamespace(email='USER@example.test'))
        self.assertEqual(
            headers,
            {
                'Content-Type': 'application/json',
                'X-NewAPI-Identity': 'user@example.test',
                'X-NewAPI-Group': '%E4%B8%AD%E6%96%87',
            },
        )
        self.assertEqual(cookies, {'new_api_image_studio': 'session'})

    def test_group_override_is_request_local(self):
        request = Request(cookies={'new_api_image_studio': 'session'}, headers={'X-NewAPI-Group': 'chat-group'})
        headers, _ = bridge.bridge_headers_and_cookies(
            request,
            SimpleNamespace(email='user@example.test'),
            group_override='image-group',
        )
        self.assertEqual(headers['X-NewAPI-Group'], 'image-group')
        self.assertEqual(request.headers['X-NewAPI-Group'], 'chat-group')

    def test_configured_image_group_and_empty_group_are_not_chat_groups(self):
        self.assertFalse(bridge.bridge_chat_group_allowed(Request(headers={}), 'image-group'))
        self.assertFalse(bridge.bridge_chat_group_allowed(Request(headers={'X-NewAPI-Group': 'image-group'}), 'image-group'))
        self.assertFalse(bridge.bridge_chat_group_allowed(Request(headers={'X-NewAPI-Group': '%69%6d%61%67%65%2d%67%72%6f%75%70'}), 'image-group'))
        self.assertTrue(bridge.bridge_chat_group_allowed(Request(headers={'X-NewAPI-Group': 'chat-group'}), 'image-group'))
        self.assertTrue(bridge.bridge_chat_group_allowed(Request(headers={}), ''))

    def test_trusted_name_percent_utf8_and_bridge_target(self):
        request = Request(headers={'X-Studio-Name': '%E5%BC%A0%E4%B8%89'})
        self.assertEqual(bridge.canonical_trusted_name(request, 'fallback@example.test'), '张三')
        self.assertEqual(bridge.bridge_base_url(), 'http://host.docker.internal:3000/api/image-studio/v1')
        self.assertTrue(bridge.is_bridge_url('http://host.docker.internal:3000/api/image-studio/v1'))
        self.assertFalse(bridge.is_bridge_url('http://upstream.example/v1'))

    def test_model_url_is_bounded_to_the_bridge_origin_and_endpoint(self):
        base = bridge.bridge_base_url()
        self.assertTrue(bridge.is_bridge_url(f'{base}/models'))
        for url in [
            'http://other.example/api/image-studio/v1/models',
            'http://host.docker.internal:3000/v1/models',
            f'{base}-other/models', f'{base}/models-other', f'{base}/../models',
        ]:
            with self.subTest(url=url):
                self.assertFalse(bridge.is_bridge_url(url))

    def test_nonbridge_helper_is_disabled_without_origin(self):
        os.environ.pop('NEW_API_STUDIO_ORIGIN', None)
        self.assertFalse(bridge.bridge_enabled())
        self.assertIsNone(bridge.bridge_base_url())

    def test_bridge_image_model_requires_a_nonempty_string(self):
        request = SimpleNamespace(state=SimpleNamespace())
        self.assertEqual(bridge.bridge_image_generation_model(request, {'image_generation_model': ' gpt-image-1 '}), 'gpt-image-1')
        for features in (None, {}, {'image_generation_model': ''}, {'image_generation_model': 1}):
            with self.subTest(features=features):
                with self.assertRaises(bridge.HTTPException) as raised:
                    bridge.bridge_image_generation_model(request, features)
                self.assertEqual(raised.exception.status_code, 400)
class PatchedBehaviorTests(unittest.TestCase):
    def test_profile_canonical_name_preserves_other_fields(self):
        updates = []

        class Users:
            @staticmethod
            async def update_user_by_id(user_id, data, db=None):
                updates.append((user_id, data, db))
                return SimpleNamespace(id=user_id, name=data['name'])

        class Form:
            def model_dump(self):
                return {'name': 'forged', 'bio': 'kept'}

        functions = load_patched_functions(
            patcher.patch_auths(AUTHS_SOURCE, Path('auths.py')),
            {'update_profile'},
            {
                'Users': Users,
                'WEBUI_AUTH_TRUSTED_EMAIL_HEADER': 'X-Studio-Email',
                'canonical_trusted_name': bridge.canonical_trusted_name,
                'publish_event': self._publish_event,
                'EVENTS': SimpleNamespace(USER_PROFILE_UPDATED='updated'),
                'HTTPException': bridge.HTTPException,
                'ERROR_MESSAGES': SimpleNamespace(DEFAULT=lambda: 'default', INVALID_CRED='invalid'),
            },
        )
        request = Request(headers={'X-Studio-Name': '%E5%BC%A0%E4%B8%89'})
        user = asyncio.run(functions['update_profile'](request, Form(), SimpleNamespace(id='u1', email='user@test'), 'db'))

        self.assertEqual(user.name, '张三')
        self.assertEqual(updates, [('u1', {'name': '张三', 'bio': 'kept'}, 'db')])

    def test_existing_user_name_sync_and_identity_mismatch(self):
        updates = []
        existing = SimpleNamespace(id='u1', email='user@test', name='old name')

        class Auths:
            @staticmethod
            async def authenticate_user_by_email(email, db=None):
                return existing

        class Users:
            @staticmethod
            async def update_user_by_id(user_id, data, db=None):
                updates.append((user_id, data, db))
                existing.name = data['name']
                return existing

            @staticmethod
            async def get_user_by_id(user_id):
                return existing

        signin = load_patched_functions(
            patcher.patch_auths(AUTHS_SOURCE, Path('auths.py')),
            {'signin'},
            {
                'Auths': Auths,
                'Users': Users,
                'WEBUI_AUTH_TRUSTED_EMAIL_HEADER': 'X-Studio-Email',
                'WEBUI_AUTH_TRUSTED_GROUPS_HEADER': None,
                'canonical_trusted_name': bridge.canonical_trusted_name,
            },
        )['signin']
        asyncio.run(signin(Request(headers={'X-Studio-Email': 'USER@test', 'X-Studio-Name': 'New%20Name'}), None, None, 'db'))
        self.assertEqual(updates, [('u1', {'name': 'New Name'}, 'db')])

        current_user = load_patched_functions(
            patcher.patch_auth(AUTH_SOURCE, Path('auth.py')),
            {'get_current_user'},
            {
                'Users': Users,
                'WEBUI_AUTH_TRUSTED_EMAIL_HEADER': 'X-Studio-Email',
                'WEBUI_AUTH_TRUSTED_NAME_HEADER': None,
                'canonical_trusted_name': bridge.canonical_trusted_name,
                'HTTPException': bridge.HTTPException,
                'status': bridge.status,
                'ERROR_MESSAGES': SimpleNamespace(INVALID_TOKEN='invalid', UNAUTHORIZED='unauthorized'),
            },
        )['get_current_user']
        with self.assertRaises(bridge.HTTPException) as raised:
            asyncio.run(current_user(Request(headers={'X-Studio-Email': 'other@test'}), None, None))
        self.assertEqual(raised.exception.status_code, 401)
        self.assertIn('sign in again', raised.exception.detail)

    def test_runtime_config_bridge_and_nonbridge_branches(self):
        class Config:
            @staticmethod
            async def get_many(*keys):
                return {
                    'openai.enable': False,
                    'openai.api_base_urls': ['http://native/v1'],
                    'openai.api_keys': ['native-key'],
                    'openai.api_configs': {'0': {'enable': True}},
                }

        patched = patcher.patch_openai(OPENAI_SOURCE, Path('openai.py'))
        bridge_runtime = load_patched_functions(
            patched,
            {'get_openai_runtime_config'},
            {
                'Config': Config,
                'bridge_enabled': lambda: True,
                'bridge_base_url': lambda: 'http://host.docker.internal:3000/api/image-studio/v1',
            },
        )['get_openai_runtime_config']
        self.assertEqual(
            asyncio.run(bridge_runtime()),
            (True, ['http://host.docker.internal:3000/api/image-studio/v1'], [''], {'0': {'auth_type': 'none', 'enable': True}}),
        )

        native_runtime = load_patched_functions(
            patched,
            {'get_openai_runtime_config'},
            {'Config': Config, 'bridge_enabled': lambda: False, 'bridge_base_url': bridge.bridge_base_url},
        )['get_openai_runtime_config']
        self.assertEqual(asyncio.run(native_runtime())[1:3], (['http://native/v1'], ['native-key']))

    def test_empty_authorized_models_skip_cache_and_do_not_use_base_models(self):
        class Config:
            @staticmethod
            async def get_many(*keys):
                return {'models.base_models_cache': True}

        class Redis:
            async def get(self, key):
                raise AssertionError('bridge mode must not read the base-model cache')

        functions = load_patched_functions(
            patcher.patch_models(MODELS_SOURCE, Path('models.py')),
            {'get_all_base_models', 'get_all_models'},
            {
                'Request': object,
                'UserModel': object,
                'Config': Config,
                'bridge_enabled': lambda: True,
                'fetch_openai_models': self._empty_openai_models,
                'asyncio': asyncio,
                'RedisDict': dict,
                'JSONCodec': SimpleNamespace(dumps=lambda value: value),
                'log': SimpleNamespace(warning=lambda *args: None),
            },
        )
        request = SimpleNamespace(
            app=SimpleNamespace(state=SimpleNamespace(BASE_MODELS=[{'id': 'stale'}], MODELS={'stale': {'id': 'stale'}}, redis=Redis())),
            state=SimpleNamespace(),
        )
        self.assertEqual(asyncio.run(functions['get_all_models'](request, user=SimpleNamespace())), [])
        self.assertEqual(request.state.newapi_bridge_models, {})
        self.assertEqual(request.app.state.BASE_MODELS, [{'id': 'stale'}])

    @staticmethod
    async def _publish_event(*args, **kwargs):
        return None

    @staticmethod
    async def _empty_openai_models(request, user=None):
        return []


class ImageStudioBehaviorTests(unittest.TestCase):
    def _functions(self, globals_dict, names):
        return load_patched_functions(
            patcher.patch_images(IMAGES_SOURCE, Path('images.py')),
            names,
            {
                'Request': object,
                'Depends': lambda value: value,
                'get_verified_user': object(),
                **globals_dict,
            },
        )

    def test_admin_group_config_contract_and_public_bootstrap(self):
        async def _config_values(*_):
            return {'NEW_API_IMAGE_GROUP': 'image-group'}

        async def _publish_event(*_):
            return None

        class Config:
            @staticmethod
            async def get(key):
                return 'image-group'

        functions = self._functions(
            {
                'Config': Config,
                'bridge_enabled': lambda: True,
                'bridge_base_url': lambda: 'http://bridge',
                'bridge_headers_and_cookies': bridge.bridge_headers_and_cookies,
                'HTTPException': bridge.HTTPException,
                'get_session': None,
                'log': SimpleNamespace(exception=lambda *_: None),
                'IMAGE_CONFIG_KEYS': {'NEW_API_IMAGE_GROUP': 'image_generation.newapi_group'},
                'get_config_values': lambda _: _config_values(),
                'publish_event': _publish_event,
            },
            {'config_updates', 'get_newapi_image_group', 'get_studio_config', 'update_config'},
        )
        self.assertEqual(
            functions['config_updates']({'NEW_API_IMAGE_GROUP': 'image-group', 'NEW_API_BRIDGE_ENABLED': False}, {
                'NEW_API_IMAGE_GROUP': 'image_generation.newapi_group'
            }),
            {'image_generation.newapi_group': 'image-group'},
        )
        self.assertEqual(asyncio.run(functions['get_studio_config']()), {'group': 'image-group'})
        self.assertTrue(asyncio.run(functions['update_config'](Request(), None, None))['NEW_API_BRIDGE_ENABLED'])

    def test_studio_models_are_group_scoped_and_no_group_skips_request(self):
        calls = []

        async def get_models(request, user, group):
            calls.append(group)
            return ([{'id': 'chat-model'}, {'id': 'any-model'}], None)

        class Config:
            group = ''

            @staticmethod
            async def get(key):
                return Config.group

        functions = self._functions(
            {
                'Config': Config,
                'bridge_enabled': lambda: True,
                'get_newapi_image_models': get_models,
                'bridge_image_size_options': bridge.bridge_image_size_options,
            },
            {'get_newapi_image_group', 'get_studio_models'},
        )
        self.assertEqual(asyncio.run(functions['get_studio_models'](Request(), SimpleNamespace())), {'group': '', 'models': []})
        self.assertEqual(calls, [])
        Config.group = 'image-group'
        self.assertEqual(
            asyncio.run(functions['get_studio_models'](Request(), SimpleNamespace())),
            {'group': 'image-group', 'models': [{'id': 'chat-model', 'image_size_options': []}, {'id': 'any-model', 'image_size_options': []}]},
        )
        self.assertEqual(calls, ['image-group'])

    def test_group_errors_and_selected_model_validation(self):
        class Config:
            @staticmethod
            async def get(key):
                return 'image-group'

        async def unavailable(request, user, group):
            return [], 'Unable to retrieve image models for group image-group.'

        functions = self._functions(
            {
                'Config': Config,
                'bridge_enabled': lambda: True,
                'bridge_image_generation_model': bridge.bridge_image_generation_model,
                'get_newapi_image_models': unavailable,
                'HTTPException': bridge.HTTPException,
            },
            {'get_newapi_image_group', 'get_studio_models', 'require_newapi_image_model'},
        )
        self.assertEqual(
            asyncio.run(functions['get_studio_models'](Request(), SimpleNamespace())),
            {'group': 'image-group', 'models': [], 'error': 'Unable to retrieve image models for group image-group.'},
        )
        with self.assertRaises(bridge.HTTPException) as raised:
            asyncio.run(functions['require_newapi_image_model'](Request(), SimpleNamespace(), 'model-a'))
        self.assertEqual(raised.exception.status_code, 400)

        async def authorized(request, user, group):
            return ([{'id': 'model-a'}, 'model-b'], None)

        functions['get_newapi_image_models'] = authorized
        self.assertEqual(
            asyncio.run(functions['require_newapi_image_model'](Request(), SimpleNamespace(), 'model-b')),
            ('model-b', 'image-group'),
        )

    def test_signed_group_fetch_preserves_chat_group_and_unauthorized_is_401(self):
        captured = {}

        class Response:
            status = 401

            async def __aenter__(self):
                return self

            async def __aexit__(self, *_):
                return False

        class Session:
            def get(self, **kwargs):
                captured.update(kwargs)
                return Response()

        async def get_session():
            return Session()

        functions = self._functions(
            {
                'bridge_headers_and_cookies': bridge.bridge_headers_and_cookies,
                'bridge_base_url': lambda: 'http://bridge',
                'get_session': get_session,
                'AIOHTTP_CLIENT_SESSION_SSL': False,
                'HTTPException': bridge.HTTPException,
                'log': SimpleNamespace(exception=lambda *_: None),
            },
            {'get_newapi_image_models'},
        )
        request = Request(cookies={'new_api_image_studio': 'session'}, headers={'X-NewAPI-Group': 'chat-group'})
        with self.assertRaises(bridge.HTTPException) as raised:
            asyncio.run(functions['get_newapi_image_models'](request, SimpleNamespace(email='user@test'), 'image-group'))
        self.assertEqual(raised.exception.status_code, 401)
        self.assertEqual(captured['headers']['X-NewAPI-Group'], 'image-group')
        self.assertEqual(request.headers['X-NewAPI-Group'], 'chat-group')


class PatchAstTests(unittest.TestCase):
    def test_configured_image_group_is_excluded_from_chat_model_listing(self):
        class Config:
            @staticmethod
            async def get(key):
                return 'image-group'

        functions = load_patched_functions(
            patcher.patch_openai(OPENAI_SOURCE, Path('openai.py')),
            {'get_all_models'},
            {
                'Config': Config,
                'Request': object,
                'UserModel': object,
                'bridge_enabled': lambda: True,
                'bridge_chat_group_allowed': bridge.bridge_chat_group_allowed,
            },
        )
        self.assertEqual(asyncio.run(functions['get_all_models'](Request(headers={}), SimpleNamespace())), {'data': []})
        self.assertEqual(
            asyncio.run(functions['get_all_models'](Request(headers={'X-NewAPI-Group': 'image-group'}), SimpleNamespace())),
            {'data': []},
        )
    def test_patched_functions_parse_and_include_bridge_guards(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            package = root / 'open_webui'
            (package / 'routers').mkdir(parents=True)
            (package / 'utils').mkdir()
            (package / 'tools').mkdir()
            (package / 'config.py').write_text(CONFIG_SOURCE, encoding='utf-8')
            (package / 'routers' / 'auths.py').write_text(AUTHS_SOURCE, encoding='utf-8')
            (package / 'utils' / 'auth.py').write_text(AUTH_SOURCE, encoding='utf-8')
            (package / 'routers' / 'openai.py').write_text(OPENAI_SOURCE, encoding='utf-8')
            (package / 'routers' / 'images.py').write_text(IMAGES_SOURCE, encoding='utf-8')
            (package / 'tools' / 'builtin.py').write_text(BUILTIN_SOURCE, encoding='utf-8')
            (package / 'utils' / 'tools.py').write_text(TOOLS_SOURCE, encoding='utf-8')
            (package / 'utils' / 'middleware.py').write_text(MIDDLEWARE_SOURCE, encoding='utf-8')
            (package / 'utils' / 'models.py').write_text(MODELS_SOURCE, encoding='utf-8')
            (package / 'main.py').write_text(MAIN_SOURCE, encoding='utf-8')

            patcher.apply_patch(root, HERE / 'open_webui_bridge.py')

            for relative, functions in {
                'routers/auths.py': {'signin', 'update_profile'},
                'utils/auth.py': {'get_current_user'},
                'routers/openai.py': {'get_headers_and_cookies', 'get_openai_runtime_config', 'get_all_models'},
                'routers/images.py': {'image_generations', 'image_edits', 'get_studio_config', 'get_studio_models'},
                'tools/builtin.py': {'generate_image'},
                'utils/tools.py': {'get_builtin_tools'},
                'utils/middleware.py': {'chat_image_generation_handler'},
                'utils/models.py': {'get_all_base_models', 'get_all_models'},
                'main.py': {'chat_completion'},
            }.items():
                tree = ast.parse((package / relative).read_text(encoding='utf-8'))
                found = {node.name: node for node in tree.body if isinstance(node, (ast.FunctionDef, ast.AsyncFunctionDef))}
                self.assertTrue(functions <= found.keys(), relative)

            openai_tree = ast.parse((package / 'routers' / 'openai.py').read_text(encoding='utf-8'))
            runtime = next(node for node in openai_tree.body if isinstance(node, ast.AsyncFunctionDef) and node.name == 'get_openai_runtime_config')
            self.assertIsInstance(runtime.body[0], ast.If)
            self.assertEqual(ast.unparse(runtime.body[0].test), 'bridge_enabled()')

            models = next(node for node in openai_tree.body if isinstance(node, ast.AsyncFunctionDef) and node.name == 'get_all_models')
            self.assertIn('known_models', ast.unparse(models))

            model_utils_tree = ast.parse((package / 'utils' / 'models.py').read_text(encoding='utf-8'))
            model_utils = next(node for node in model_utils_tree.body if isinstance(node, ast.AsyncFunctionDef) and node.name == 'get_all_models')
            self.assertIn('newapi_bridge_models', ast.unparse(model_utils))

            main_tree = ast.parse((package / 'main.py').read_text(encoding='utf-8'))
            chat = next(node for node in main_tree.body if isinstance(node, ast.AsyncFunctionDef) and node.name == 'chat_completion')
            self.assertIn('model_registry', ast.unparse(chat))

            builtin_tree = ast.parse((package / 'tools' / 'builtin.py').read_text(encoding='utf-8'))
            generate_image = next(node for node in builtin_tree.body if isinstance(node, ast.AsyncFunctionDef) and node.name == 'generate_image')
            self.assertIn('__features__', [argument.arg for argument in generate_image.args.args])
            self.assertNotIn('model', [argument.arg for argument in generate_image.args.args])
            self.assertIn('bridge_image_generation_model(__request__, __features__)', ast.unparse(generate_image))

            middleware_tree = ast.parse((package / 'utils' / 'middleware.py').read_text(encoding='utf-8'))
            legacy = next(node for node in middleware_tree.body if isinstance(node, ast.AsyncFunctionDef) and node.name == 'chat_image_generation_handler')
            self.assertIn("bridge_image_generation_model(request, extra_params.get('__features__'))", ast.unparse(legacy))
            self.assertNotIn('bridge_image_generation_model(request, features)', (package / 'utils' / 'middleware.py').read_text(encoding='utf-8'))
            image_source = (package / 'routers' / 'images.py').read_text(encoding='utf-8')
            self.assertIn("@router.get('/studio/config')", image_source)
            self.assertIn("@router.get('/studio')", image_source)
            self.assertIn("'image_generation.newapi_group'", image_source)
            self.assertIn("'image_generation.newapi_group': NEW_API_IMAGE_GROUP", (package / 'config.py').read_text(encoding='utf-8'))

    def test_duplicate_anchor_is_rejected(self):
        with self.assertRaises(patcher.PatchError):
            patcher.replace_once(Path('fixture.py'), 'needle needle', 'needle', 'replacement', 'duplicate')

    def test_identity_name_nonbridge_and_empty_list_ast_contracts(self):
        auth_tree = ast.parse(patcher.patch_auth(AUTH_SOURCE, Path('auth.py')))
        current_user = next(node for node in auth_tree.body if isinstance(node, ast.AsyncFunctionDef) and node.name == 'get_current_user')
        current_source = ast.unparse(current_user)
        self.assertIn('user.email != trusted_email', current_source)
        self.assertIn('Users.update_user_by_id', current_source)

        auths_tree = ast.parse(patcher.patch_auths(AUTHS_SOURCE, Path('auths.py')))
        signin = next(node for node in auths_tree.body if isinstance(node, ast.AsyncFunctionDef) and node.name == 'signin')
        self.assertIn('user.name != canonical_name', ast.unparse(signin))

        openai_tree = ast.parse(patcher.patch_openai(OPENAI_SOURCE, Path('openai.py')))
        runtime = next(node for node in openai_tree.body if isinstance(node, ast.AsyncFunctionDef) and node.name == 'get_openai_runtime_config')
        self.assertIn("values.get('openai.enable')", ast.unparse(runtime))

        models_tree = ast.parse(patcher.patch_models(MODELS_SOURCE, Path('models.py')))
        models = next(node for node in models_tree.body if isinstance(node, ast.AsyncFunctionDef) and node.name == 'get_all_models')
        self.assertIn('request.state.newapi_bridge_models = {}', ast.unparse(models))
        arena_guard = next(
            node for node in ast.walk(models)
            if isinstance(node, ast.If) and 'evaluation.arena.enable' in ast.unparse(node.test)
        )
        self.assertIn('not bridge_enabled()', ast.unparse(arena_guard.test))


CONFIG_SOURCE = '''IMAGE_GENERATION_MODEL = os.getenv('IMAGE_GENERATION_MODEL', '')

DEFAULT_CONFIG = {
    'image_generation.model': IMAGE_GENERATION_MODEL,
}
'''


AUTHS_SOURCE = '''from open_webui.internal.db import get_async_session

async def update_profile(request, form_data, session_user, db):
    if session_user:
        user = await Users.update_user_by_id(
            session_user.id,
            form_data.model_dump(),
            db=db,
        )
        if user:
            await publish_event(
                request,
                EVENTS.USER_PROFILE_UPDATED,
                actor=session_user,
                subject_id=session_user.id,
                data={'updated_fields': list(form_data.model_dump().keys())},
            )
            return user

async def signin(request, response, form_data, db):
    if WEBUI_AUTH_TRUSTED_EMAIL_HEADER:
        email = request.headers[WEBUI_AUTH_TRUSTED_EMAIL_HEADER].lower()
        user = await Auths.authenticate_user_by_email(email, db=db)
        if user:
            if WEBUI_AUTH_TRUSTED_GROUPS_HEADER:
                pass
'''

AUTH_SOURCE = '''from open_webui.env import (
    WEBUI_AUTH_TRUSTED_EMAIL_HEADER,
)
from open_webui.models.users import Users

async def get_current_user(request, response, background_tasks, auth_token=None):
    user = await Users.get_user_by_id('id')
    if user is not None:
        if True:
            if True:
                if WEBUI_AUTH_TRUSTED_EMAIL_HEADER:
                    trusted_email = request.headers.get(WEBUI_AUTH_TRUSTED_EMAIL_HEADER, '').lower()
                    if trusted_email and user.email != trusted_email:
                        raise HTTPException(
                            status_code=status.HTTP_401_UNAUTHORIZED,
                            detail='User mismatch. Please sign in again.',
                        )

                # Add user info to current span
                return user
'''

OPENAI_SOURCE = '''import logging
from aiocache import cached
from open_webui.internal.db import get_async_session

async def send_get_request(request=None, url=None, key=None, user=None, config=None):
    try:
        async with aiohttp.ClientSession(timeout=_MODEL_LIST_TIMEOUT, trust_env=True) as session:
            if request and config:
                headers, cookies = await get_headers_and_cookies(request, url, key, config, user=user)
            else:
                headers = {'Authorization': f'Bearer {key}'} if key else {}
                cookies = None
            async with session.get(
                url, headers=headers, cookies=cookies, ssl=AIOHTTP_CLIENT_SESSION_SSL,
            ) as response:
                return await response.json(loads=JSONCodec.loads)
    except Exception as e:
        # Handle connection error here
        log.error(f'Connection error: {e}')
        return None

async def get_headers_and_cookies(request, url, key=None, config=None, metadata=None, user=None):
    cookies = getattr(request, 'cookies', {}) if config.get('forward_cookies', False) else {}
    headers = {
        'Content-Type': 'application/json',
    }
    return headers, cookies

async def get_openai_runtime_config() -> tuple[bool, list[str], list[str], dict]:
    values = await Config.get_many('openai.enable', 'openai.api_base_urls', 'openai.api_keys', 'openai.api_configs')
    return (
        values.get('openai.enable'),
        values.get('openai.api_base_urls') or [],
        values.get('openai.api_keys') or [],
        values.get('openai.api_configs') or {},
    )

@cached(
    ttl=MODELS_CACHE_TTL,
    # key_builder (not key) is the per-call hook in aiocache 0.12; `key=` is a
    # static key, so a `key=lambda` collapsed every caller to one shared entry.
    key_builder=lambda _func, request, user=None: f'openai_all_models_{user.id}' if user else 'openai_all_models',
)
async def get_all_models(request: Request, user: UserModel) -> dict[str, list]:
    log.info('get_all_models()')
    models = {}
    request.app.state.OPENAI_MODELS = models
    return {'data': list(models.values())}

async def get_models(request: Request, url_idx: int | None = None, user=Depends(get_verified_user)):
    if url_idx is not None and user.role != 'admin':
        raise HTTPException(status_code=401, detail=ERROR_MESSAGES.ACCESS_PROHIBITED)

    if not await Config.get('openai.enable'):
        raise HTTPException(status_code=503, detail='OpenAI API is disabled')

async def generate_chat_completion(
    request: Request,
    form_data: dict,
    user=Depends(get_verified_user),
):
    if not await Config.get('openai.enable'):
        raise HTTPException(status_code=503, detail='OpenAI API is disabled')
    bypass_filter = getattr(request.state, 'bypass_filter', False)
    if BYPASS_MODEL_ACCESS_CONTROL:
        bypass_filter = True
    model_id = form_data.get('model')
    # Check if model is already in app state cache to avoid expensive get_all_models() call
    models = request.app.state.OPENAI_MODELS
    if not models or model_id not in models:
        await get_all_models(request, user=user)
        models = request.app.state.OPENAI_MODELS
    model = models.get(model_id)

async def responses(request, form_data, user):
    model_id = form_data.model
    # Enforce per-model access control
    await check_model_access(user, await Models.get_model_by_id(model_id), BYPASS_MODEL_ACCESS_CONTROL)
    if model_id:
        models = request.app.state.OPENAI_MODELS
        if not models or model_id not in models:
            await get_all_models(request, user=user)
            models = request.app.state.OPENAI_MODELS
        if model_id in models:
            idx = models[model_id]['urlIdx']

    url, key, api_config = await get_openai_connection(idx)

    payload['model'] = strip_provider_model_prefix(payload['model'], api_config.get('prefix_id'))

async def proxy(path, request, user):
    model_id = None
    if model_id:
        models = request.app.state.OPENAI_MODELS
        if not models or model_id not in models:
            await get_all_models(request, user=user)
            models = request.app.state.OPENAI_MODELS
        if model_id in models:
            idx = models[model_id]['urlIdx']

    url, key, api_config = await get_openai_connection(idx)
    base_url = url
'''

IMAGES_SOURCE = '''from open_webui.internal.db import get_async_session

IMAGE_CONFIG_KEYS = {
    'IMAGE_GENERATION_MODEL': 'image_generation.model',
}

def config_updates(data, key_map):
    return {key_map[field]: value for field, value in data.items() if field in key_map}

class ImagesConfig:
    IMAGE_GENERATION_MODEL: str

@router.get('/config', response_model=ImagesConfig)
async def get_config(request: Request, user=Depends(get_admin_user)):
    return await get_config_values(IMAGE_CONFIG_KEYS)

async def update_config(request, form_data, user):
    values = await get_config_values(IMAGE_CONFIG_KEYS)
    await publish_event(
        request,
    )
    return values

async def get_image_model(request):
    return 'image-model'

def get_automatic1111_api_auth(image_config):
    return ''

async def get_image_data(data: str, headers=None, trusted_base_url: str | None = None):
    try:
        async with session.get(
                data,
                headers=headers,
                ssl=AIOHTTP_CLIENT_SESSION_SSL,
        ) as r:
            return r
    except Exception as e:
        log.exception(f'Error loading image data: {e}')
        return None, None

async def image_generations(
    request: Request,
    form_data: CreateImageForm,
    metadata: dict | None = None,
    user=None,
):
    size = '512x512'
    model = await get_image_model(request)

    try:
        if image_config.IMAGE_GENERATION_ENGINE == 'openai':
            headers = {
                'Authorization': f'Bearer {image_config.IMAGES_OPENAI_API_KEY}',
                'Content-Type': 'application/json',
            }

            if ENABLE_FORWARD_USER_INFO_HEADERS:
                headers = include_user_info_headers(headers, user)

            url = f'{image_config.IMAGES_OPENAI_API_BASE_URL}/images/generations'
            data = {
                **({} if not image_config.IMAGES_OPENAI_API_PARAMS else image_config.IMAGES_OPENAI_API_PARAMS),
            }
            session = await get_session()
            async with session.post(
                url=url,
                json=data,
                headers=headers,
                ssl=AIOHTTP_CLIENT_SESSION_SSL,
            ) as r:
                pass
            if re.match(
                        IMAGE_URL_RESPONSE_MODELS_REGEX_PATTERN,
                        image_config.IMAGE_GENERATION_MODEL,
            ):
                pass
            for image in res['data']:
                if image_url := image.get('url', None):
                    image_data, content_type = await get_image_data(
                        image_url,
                        {k: v for k, v in headers.items() if k != 'Content-Type'}
                        if _is_same_origin(image_url, image_config.IMAGES_OPENAI_API_BASE_URL)
                        else None,
                    )
                else:
                    image_data, content_type = await get_image_data(image['b64_json'])

                _, image_file = await upload_image(request, image_data, content_type, {**data, **metadata}, user)
                images.append(image_file)
            return images
    except Exception as e:
        error = e
        if isinstance(e, aiohttp.ClientResponseError):
            error = e.message
        raise HTTPException(status_code=400, detail=ERROR_MESSAGES.DEFAULT(error))


class EditImageForm:
    pass

async def image_edits(request, form_data, metadata=None, user=None):
    try:
        if image_config.IMAGE_EDIT_ENGINE == 'openai':
            headers = {
                'Authorization': f'Bearer {image_config.IMAGES_EDIT_OPENAI_API_KEY}',
            }

            if ENABLE_FORWARD_USER_INFO_HEADERS:
                headers = include_user_info_headers(headers, user)

            data = {
            }
            async with session.post(
                url=f'{image_config.IMAGES_EDIT_OPENAI_API_BASE_URL}/images/edits{url_search_params}',
                headers=headers,
                data=form,
                ssl=AIOHTTP_CLIENT_SESSION_SSL,
            ) as r:
                pass
            if True:
                if True:
                    image_data = await get_image_data(
                        image_url,
                        {k: v for k, v in headers.items() if k != 'Content-Type'}
                        if _is_same_origin(image_url, image_config.IMAGES_EDIT_OPENAI_API_BASE_URL)
                        else None,
                    )
    except Exception:
        pass
'''

MODELS_SOURCE = '''from open_webui.models.users import UserModel

async def get_all_base_models(request: Request, user: UserModel = None):
    config = await Config.get_many('openai.enable', 'ollama.enable')
    openai_models, ollama_models, function_models = await asyncio.gather(openai_task, ollama_task, function_task)

    return function_models + openai_models + ollama_models

async def get_all_models(request, refresh=False, user=None):
    config = await Config.get_many(
        'models.base_models_cache',
        'evaluation.arena.enable',
        'evaluation.arena.models',
        'models.default_metadata',
    )
    use_cache = config.get('models.base_models_cache') and not refresh
    base_models = None
    if base_models is None:
        base_models = await get_all_base_models(request, user=user)
        if base_models:
            request.app.state.BASE_MODELS = base_models
            if config.get('models.base_models_cache') and redis is not None:
                await redis.set(BASE_MODELS_CACHE_KEY, JSONCodec.dumps(base_models))
        else:
            base_models = request.app.state.BASE_MODELS
    models = [model.copy() for model in base_models]
    if len(models) == 0:
        return []
    # Add arena models
    if config.get('evaluation.arena.enable'):
        models = models + [{'id': 'arena-model', 'name': 'Arena Model', 'arena': True}]
    for custom_model in custom_models:
        if True:
            base_model = base_model_lookup.get(custom_model.base_model_id)
            if base_model is None:
                base_model = base_model_lookup.get(custom_model.base_model_id.split(':')[0])
            if base_model:
                owned_by = base_model.get('owned_by', 'unknown')
                if 'pipe' in base_model:
                    pipe = base_model['pipe']
                connection_type = base_model.get('connection_type', None)

            model = {
                'id': custom_model.id,
            }
    models_dict = {}
    if isinstance(request.app.state.MODELS, RedisDict):
        try:
            request.app.state.MODELS.set(models_dict)
        except Exception as e:
            log.warning(f'Failed to update Redis model cache, using in-process cache: {e}')
            request.app.state.MODELS = models_dict
    else:
        request.app.state.MODELS = models_dict

    return models
'''

MAIN_SOURCE = '''from open_webui.constants import ERROR_MESSAGES, TASKS

async def get_models(request, refresh=False, user=None):
    all_models = await get_all_models(request, refresh=refresh, user=user)
    models = await get_filtered_models(models, user)
    return {'data': models}

async def chat_completion(request, form_data, user):
    if not request.app.state.MODELS:
        await get_all_models(request, user=user)

    model_id = form_data.get('model', None)
    if True:
        if not model_item.get('direct', False):
            if model_id not in request.app.state.MODELS:
                raise Exception('Model not found')

            model = request.app.state.MODELS[model_id]
            model_info = await Models.get_model_by_id(model_id)
            missing_base_model = bool(
                model_info and model_info.base_model_id and model_info.base_model_id not in request.app.state.MODELS
            )
            if missing_base_model and ENABLE_CUSTOM_MODEL_FALLBACK:
                fallback_model = request.app.state.MODELS.get(fallback_model_id)

            if not BYPASS_MODEL_ACCESS_CONTROL and (user.role != 'admin' or not BYPASS_ADMIN_ACCESS_CONTROL):
                try:
                    pass
                except Exception:
                    raise

    resolved_model = request.app.state.MODELS.get(target_model_id, model)

async def generate_messages(request, form_data, user):
    passthrough_params = []
    models = request.app.state.OPENAI_MODELS
    if not models or model_id not in models:
        await openai.get_all_models(request, user=user)
        models = request.app.state.OPENAI_MODELS
    model = models.get(model_id)
'''

BUILTIN_SOURCE = '''from open_webui.routers.images import (
    CreateImageForm,
)

async def generate_image(
    prompt: str,
    __request__: Request = None,
    __user__: dict = None,
    __event_emitter__: callable = None,
    __chat_id__: str = None,
    __message_id__: str = None,
) -> str:
    """
    Generate an image based on a text prompt.

    :param prompt: A detailed description of the image to generate
    """
    try:
        user = UserModel(**__user__) if __user__ else None

        images = await image_generations(
            request=__request__,
            form_data=CreateImageForm(prompt=prompt),
            metadata=None,
            user=None,
        )
        # Prepare file entries for the images
        if __event_emitter__:
            return JSONCodec.dumps({
                    'images': images,
            })
        return JSONCodec.dumps({'status': 'success', 'images': images}, ensure_ascii=False)
    except Exception as e:
        log.exception(f'generate_image error: {e}')
        return JSONCodec.dumps({'error': str(e)})
'''

TOOLS_SOURCE = '''async def get_builtin_tools(request, extra_params, features=None):
    for func in builtin_functions:
        callable = await get_async_tool_function_and_apply_extra_params(
            func,
            {
                '__files__': chat_files,
            },
            get_builtin_function_introspection(func),
        )
'''

MIDDLEWARE_SOURCE = '''from open_webui.routers.images import (
    CreateImageForm,
)

async def process(request, form_data, extra_params):
    features = form_data.pop('features', None) or {}
    extra_params['__features__'] = features
    if features:
        pass

async def chat_image_generation_handler(request, form_data, extra_params, user):
    prompt = 'prompt'
    if True:
        if True:
            images = await image_generations(
                request=request,
                form_data=CreateImageForm(**{'prompt': prompt}),
                metadata=None,
                user=user,
            )
    return images
'''


if __name__ == '__main__':
    unittest.main()
