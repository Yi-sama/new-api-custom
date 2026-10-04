"""Check native image tool binding without importing the app or calling a provider."""

from __future__ import annotations

import asyncio
import base64
import copy
import io
import inspect
import json
import logging
import mimetypes
import re
import sys
import unittest
from functools import partial, update_wrapper
from pathlib import Path
from types import SimpleNamespace
from typing import Any, get_args, get_type_hints

import aiohttp
from fastapi import Request, UploadFile
from langchain_core.utils.function_calling import convert_to_openai_function
from pydantic import Field, create_model

from test_open_webui_bridge import bridge, load_patched_functions, patcher


CANDIDATE = '--candidate' in sys.argv
if CANDIDATE:
    sys.argv.remove('--candidate')

PACKAGE = Path('/app/backend/open_webui')


class NativeImageToolTests(unittest.IsolatedAsyncioTestCase):
    async def asyncSetUp(self):
        self.sent = []
        self.enabled = True
        self.allowed = True

        async def image_generations(request, form_data, metadata, user, **kwargs):
            self.sent.append((request, user.id, form_data.model, form_data.prompt, form_data.size))
            return []

        async def get_many(*keys):
            return {'image_generation.enable': self.enabled}

        async def get_config(key):
            return {}

        async def has_permission(user_id, key, permissions):
            self.assertEqual(key, 'features.image_generation')
            return self.allowed

        builtin_path = PACKAGE / 'tools/builtin.py'
        builtin_source = builtin_path.read_text(encoding='utf-8')
        if CANDIDATE:
            builtin_source = patcher.patch_builtin_tools(builtin_source, builtin_path)
        functions = load_patched_functions(
            builtin_source,
            {'generate_image'},
            {
                '__name__': __name__,
                'Request': Request,
                'UserModel': SimpleNamespace,
                'CreateImageForm': lambda **values: SimpleNamespace(n=1, **values),
                'image_generations': image_generations,
                'is_saved_chat_id': lambda value: False,
                'bridge_enabled': lambda: True,
                'bridge_image_generation_model': bridge.bridge_image_generation_model,
                'bridge_image_generation_size': bridge.bridge_image_generation_size,
                'HTTPException': bridge.HTTPException,
                'JSONCodec': json,
                'log': logging.getLogger('native-image-test'),
            },
        )
        path = PACKAGE / 'utils/tools.py'
        source = path.read_text(encoding='utf-8')
        if CANDIDATE:
            source = patcher.patch_tools(source, path)
        names = {
            'get_builtin_tools', 'get_builtin_function_introspection',
            'get_async_tool_function_and_apply_extra_params', 'get_updated_tool_function',
            'get_builtin_tool_spec', 'build_builtin_tool_spec_json',
            'convert_function_to_pydantic_model', 'parse_description', 'parse_docstring',
            'clean_properties', 'clean_openai_tool_schema',
        }
        self.functions = load_patched_functions(source, names, {
            **functions,
            'inspect': inspect, 'get_type_hints': get_type_hints, 'get_args': get_args,
            'partial': partial, 'update_wrapper': update_wrapper,
            'copy': copy, 're': re, 'Any': Any, 'Field': Field, 'create_model': create_model,
            'convert_pydantic_model_to_openai_function_spec': convert_to_openai_function,
            'Config': SimpleNamespace(get_many=get_many, get=get_config),
            'has_permission': has_permission,
            'get_attached_knowledge': lambda model, metadata: [],
        })
        self.request = SimpleNamespace(state=SimpleNamespace())
        self.model = {'info': {'meta': {'builtinTools': {
            category: False for category in (
                'time', 'user_input', 'files', 'knowledge', 'chats', 'subagents',
                'memory', 'web_search', 'code_interpreter', 'notes', 'channels',
                'tasks', 'automations', 'calendar', 'notifications',
            )
        }}}}

    async def tools(self, features, user_id='alice'):
        return await self.functions['get_builtin_tools'](
            self.request,
            {
                '__user__': {'id': user_id, 'role': 'user'},
                '__features__': features,
                '__metadata__': {'features': features},
            },
            features,
            self.model,
        )

    async def test_native_registration_and_rebinding_preserve_request_model(self):
        alice = await self.tools({'image_generation': True, 'image_generation_model': 'chosen-a'})
        bob = await self.tools({'image_generation': True, 'image_generation_model': 'chosen-b'}, 'bob')
        for tool in (alice['generate_image'], bob['generate_image']):
            self.assertEqual(set(tool['spec']['parameters']['properties']), {'prompt', 'size', 'aspect_ratio'})
            self.assertEqual(tool['spec']['parameters']['required'], ['prompt'])
        updated = await self.functions['get_updated_tool_function'](
            alice['generate_image']['callable'],
            {'__messages__': [], '__files__': []},
        )
        results = await asyncio.gather(
            updated(prompt='test-a'),
            bob['generate_image']['callable'](prompt='test-b'),
        )
        self.assertEqual([json.loads(result).get('status') for result in results], ['success', 'success'])
        self.assertCountEqual(self.sent, [
            (self.request, 'alice', 'chosen-a', 'test-a', None),
            (self.request, 'bob', 'chosen-b', 'test-b', None),
        ])

    async def test_conversation_ratio_or_pixels_override_menu_and_omission_uses_menu(self):
        tools = await self.tools({
            'image_generation': True, 'image_generation_model': 'gpt-image-2', 'image_generation_size': '9:16',
        })
        generate = tools['generate_image']['callable']
        for args, expected in (
            ({'aspect_ratio': '4:3'}, '1152x864'),
            ({'size': '1984x1200'}, '1984x1200'),
            ({}, '864x1536'),
        ):
            result = json.loads(await generate(prompt='test', **args))
            self.assertEqual(result['status'], 'success')
            self.assertEqual(self.sent[-1][-1], expected)

    async def test_web_model_auto_menu_and_square_tool_argument_reach_generation(self):
        tools = await self.tools({
            'image_generation': True, 'image_generation_model': 'gpt-image-2.5-web',
            'image_generation_size': 'auto',
        })
        generate = tools['generate_image']['callable']
        for args, expected in (({}, '512x512'), ({'aspect_ratio': '1:1'}, '512x512'),
                               ({'size': '1024x1024'}, '1024x1024')):
            result = json.loads(await generate(prompt='test', **args))
            self.assertEqual(result['status'], 'success')
            self.assertEqual(self.sent[-1][2], 'gpt-image-2.5-web')
            self.assertEqual(self.sent[-1][-1], expected)

    async def test_unsupported_dimensions_cannot_silently_retry_different_size(self):
        tools = await self.tools({'image_generation': True, 'image_generation_model': 'gpt-image-2'})
        generate = tools['generate_image']['callable']
        with self.assertLogs('native-image-test', level='ERROR'):
            rejected = json.loads(await generate(prompt='test', size='1980x1200'))
            retry = json.loads(await generate(prompt='test', size='1984x1200'))
        self.assertIn('1980x1200', rejected['error'])
        self.assertIn('new user message', retry['error'])
        self.assertEqual(self.sent, [])
        self.request = SimpleNamespace(state=SimpleNamespace())
        confirmed = (await self.tools({'image_generation': True, 'image_generation_model': 'gpt-image-2'}))['generate_image']['callable']
        self.assertEqual(json.loads(await confirmed(prompt='test', size='1984x1200'))['status'], 'success')

    async def test_missing_selection_does_not_call_generation_or_use_default(self):
        tools = await self.tools({'image_generation': True})
        with self.assertLogs('native-image-test', level='ERROR'):
            result = json.loads(await tools['generate_image']['callable'](prompt='test'))
        self.assertIn('Select an image generation model', result['error'])
        self.assertEqual(self.sent, [])

    async def test_global_toggle_user_permission_and_chat_toggle_still_gate_tool(self):
        for enabled, allowed, selected in ((False, True, True), (True, False, True), (True, True, False)):
            with self.subTest(enabled=enabled, allowed=allowed, selected=selected):
                self.enabled, self.allowed = enabled, allowed
                tools = await self.tools({'image_generation': selected, 'image_generation_model': 'chosen'})
                self.assertNotIn('generate_image', tools)
        self.assertEqual(self.sent, [])


class ImageSizePayloadTests(unittest.IsolatedAsyncioTestCase):
    async def asyncSetUp(self):
        self.outbound = []
        self.downloads = []
        self.uploads = []
        self.validated = []
        self.events = []
        self.download_error = None
        self.save_error = None
        self.content_type = 'image/png'
        self.image_bytes = base64.b64decode('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+j4WQAAAAASUVORK5CYII=')
        self.response_data = {'data': [{'b64_json': base64.b64encode(self.image_bytes).decode()}]}
        self.request = SimpleNamespace(
            state=SimpleNamespace(),
            app=SimpleNamespace(url_path_for=lambda name, id: f'/api/v1/files/{id}/content'),
        )
        self.config = SimpleNamespace(
            IMAGE_GENERATION_ENGINE='openai', IMAGE_SIZE='512x512',
            IMAGES_OPENAI_API_BASE_URL='http://unused.invalid/v1',
            IMAGES_OPENAI_API_VERSION='', IMAGES_OPENAI_API_PARAMS={'size': '1024x1024'},
        )

        test = self

        class Response:
            def __init__(self, download=False):
                self.download = download
                self.headers = {'content-type': test.content_type}

            async def __aenter__(self):
                if self.download and test.download_error:
                    raise test.download_error
                return self

            async def __aexit__(self, *args):
                pass

            def raise_for_status(self):
                pass

            async def json(self, **kwargs):
                return test.response_data

            async def read(self):
                return test.image_bytes

        def post(**kwargs):
            self.outbound.append(kwargs)
            return Response()

        def get(url, **kwargs):
            self.downloads.append((url, kwargs))
            return Response(download=True)

        async def get_session():
            return SimpleNamespace(post=post, get=get)

        async def upload_file_handler(request, file, metadata, process, user):
            if self.save_error:
                raise self.save_error
            self.uploads.append((await file.read(), file.content_type, metadata))
            return SimpleNamespace(id='saved', meta={'content_type': file.content_type}, filename='image.png')

        async def get_config():
            return self.config

        async def get_image_model(request):
            return 'admin-model'

        async def authorized(request, user, model):
            return model, 'images-group'

        def headers(request, user, group_override):
            self.assertEqual(group_override, 'images-group')
            return {'X-NewAPI-Group': group_override}, {'test-session': 'not-a-real-session'}

        path = PACKAGE / 'routers/images.py'
        source = path.read_text(encoding='utf-8')
        if CANDIDATE:
            source = patcher.patch_images(source, path)
        self.functions = load_patched_functions(source, {'image_generations', 'get_image_data', 'upload_image', '_is_same_origin'}, {
            'get_image_config': get_config, 'get_image_model': get_image_model,
            'require_newapi_image_model': authorized, 'bridge_enabled': lambda: True,
            'bridge_image_generation_size': bridge.bridge_image_generation_size,
            'bridge_base_url': lambda: 'http://fixture.invalid/api/image-studio/v1',
            'bridge_headers_and_cookies': headers, 'get_session': get_session,
            'AIOHTTP_CLIENT_SESSION_SSL': False, 're': re,
            'IMAGE_URL_RESPONSE_MODELS_REGEX_PATTERN': r'^dall-e',
            'asyncio': asyncio, 'base64': base64, 'aiohttp': aiohttp,
            'HTTPException': bridge.HTTPException, 'io': io, 'mimetypes': mimetypes,
            'UploadFile': UploadFile, 'upload_file_handler': upload_file_handler,
            'validate_url': self.validated.append,
            'urlparse': __import__('urllib.parse', fromlist=['urlparse']).urlparse,
            'log': logging.getLogger('image-delivery-test'),
            'ERROR_MESSAGES': SimpleNamespace(DEFAULT=lambda error: 'Something went wrong :/'),
        })
        self.generate = self.functions['image_generations']

    async def emit(self, event):
        self.events.append(copy.deepcopy(event))

    async def generate_image(self, size='1024x1024', execution=None, model='custom'):
        return await self.generate(
            self.request, SimpleNamespace(model=model, prompt='test', size=size, n=1),
            execution=execution, event_emitter=self.emit,
        )

    async def test_effective_size_reaches_relay_and_cannot_be_overridden_by_admin_api_params(self):
        for model, size in (('gpt-image-2', '1152x864'), ('custom', '1980x1200')):
            execution = {}
            await self.generate_image(model=model, size=size, execution=execution)
            self.assertEqual(self.outbound[-1]['json']['size'], size)
            self.assertEqual(self.outbound[-1]['json']['model'], model)
            self.assertEqual(self.outbound[-1]['headers']['X-NewAPI-Group'], 'images-group')
            self.assertEqual(execution, {'model': model, 'size': size, 'n': 1, 'stage': 'complete'})
            self.assertIn(size, self.events[-1]['data']['description'])
        self.config.IMAGES_OPENAI_API_PARAMS = {}
        execution = {}
        await self.generate_image(size=None, execution=execution)
        self.assertEqual(self.outbound[-1]['json']['size'], '512x512')
        self.assertEqual(execution['size'], '512x512')
        self.assertEqual(self.uploads[-1][0], self.image_bytes)

    async def test_invalid_direct_size_rejected_before_relay(self):
        for size in ('1980x1200', 'garbage x', '8192x8192'):
            with self.subTest(size=size), self.assertRaises(bridge.HTTPException):
                await self.generate_image(model='gpt-image-2', size=size)
        self.assertEqual(self.outbound, [])

    async def test_web_default_reaches_relay_even_when_admin_defaults_differ(self):
        self.config.IMAGE_SIZE = '1536x1024'
        for requested, expected in ((None, '512x512'), ('auto', '512x512'),
                                    ('512x512', '512x512'), ('1024x1024', '1024x1024')):
            with self.subTest(requested=requested):
                execution = {}
                await self.generate_image(model='gpt-image-2.5-web', size=requested, execution=execution)
                self.assertEqual(self.outbound[-1]['json']['model'], 'gpt-image-2.5-web')
                self.assertEqual(self.outbound[-1]['json']['size'], expected)
                self.assertEqual(execution['size'], expected)

    async def test_inline_image_wins_over_url_and_reaches_file_upload(self):
        self.response_data['data'][0]['url'] = 'https://image.invalid/expired'
        result = await self.generate_image()
        self.assertEqual(self.downloads, [])
        self.assertEqual(self.uploads[0][:2], (self.image_bytes, 'image/png'))
        self.assertEqual(result[0]['url'], '/api/v1/files/saved/content')
        self.assertEqual(len(self.outbound), 1)

    async def test_url_only_is_validated_downloaded_and_uploaded_without_identity_leak(self):
        url = 'https://image.invalid/signed-result'
        self.response_data = {'data': [{'url': url}]}
        result = await self.generate_image()
        self.assertEqual(self.validated, [url])
        self.assertIsNone(self.downloads[0][1]['headers'])
        timeout = self.downloads[0][1]['timeout']
        self.assertEqual((timeout.total, timeout.connect, timeout.sock_read), (120, 15, 30))
        self.assertEqual(self.uploads[0][0], self.image_bytes)
        self.assertEqual(result[0]['id'], 'saved')

    async def test_disconnect_and_timeout_keep_delivery_error_and_do_not_regenerate(self):
        self.response_data = {'data': [{'url': 'https://image.invalid/result'}]}
        for error in (aiohttp.ServerDisconnectedError(), asyncio.TimeoutError()):
            self.download_error = error
            execution = {}
            with self.subTest(error=type(error).__name__), self.assertLogs('image-delivery-test'), self.assertRaises(bridge.HTTPException) as caught:
                await self.generate_image(execution=execution)
            self.assertEqual(caught.exception.status_code, 502)
            self.assertIn(type(error).__name__, caught.exception.detail)
            self.assertIn('charged', caught.exception.detail)
            self.assertEqual(execution['stage'], 'downloading')
            self.assertTrue(self.request.state.newapi_image_result_failed)
        self.assertEqual(len(self.outbound), 2)
        self.assertEqual(self.uploads, [])

    async def test_empty_response_and_nonimage_body_are_not_success(self):
        for response, content_type in (({'data': []}, 'image/png'), ({'data': [{'url': 'https://image.invalid/result'}]}, 'text/html')):
            self.response_data, self.content_type = response, content_type
            with self.subTest(content_type=content_type), self.assertLogs('image-delivery-test'), self.assertRaises(bridge.HTTPException):
                await self.generate_image()
        self.assertEqual(self.uploads, [])

    async def test_file_save_error_is_distinguished_from_download(self):
        self.save_error = OSError('fixture disk error')
        execution = {}
        with self.assertLogs('image-delivery-test'), self.assertRaises(bridge.HTTPException) as caught:
            await self.generate_image(execution=execution)
        self.assertIn('saving', caught.exception.detail)
        self.assertEqual(execution['stage'], 'saving')
        self.assertEqual(self.downloads, [])

    async def test_native_tool_records_actual_request_on_success_and_failure_and_blocks_retry(self):
        native = NativeImageToolTests()
        await native.asyncSetUp()
        native.request = self.request
        native.functions['generate_image'].__globals__['image_generations'] = self.generate
        tools = await native.tools({
            'image_generation': True, 'image_generation_model': 'gpt-image-2', 'image_generation_size': '9:16',
        })
        generate = tools['generate_image']['callable']
        generate = await native.functions['get_updated_tool_function'](generate, {'__event_emitter__': self.emit})
        result = json.loads(await generate(prompt='fixture'))
        self.assertEqual(result['image_request'], {'model': 'gpt-image-2', 'size': '864x1536', 'n': 1, 'stage': 'complete'})
        self.assertTrue(any(event['type'] == 'status' and event['data']['done'] for event in self.events))
        self.assertTrue(any(event['type'] == 'chat:message:files' for event in self.events))
        self.response_data = {'data': [{'url': 'https://image.invalid/result'}]}
        self.download_error = aiohttp.ServerDisconnectedError()
        with self.assertLogs(level='ERROR'):
            failed = json.loads(await generate(prompt='fixture', size='1024x1024'))
            retry = json.loads(await generate(prompt='fixture'))
        self.assertEqual(failed['image_request']['size'], '1024x1024')
        self.assertEqual(failed['image_request']['stage'], 'downloading')
        self.assertIn('502', failed['error'])
        self.assertIn('new user message', retry['error'])
        self.assertEqual(len(self.outbound), 2)


if __name__ == '__main__':
    unittest.main()
