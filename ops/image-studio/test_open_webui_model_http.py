"""Exercise the deployed model-request functions against an isolated HTTP server."""

from __future__ import annotations

import ast
import asyncio
import importlib.util
import json
import logging
import os
import sys
import unittest
from pathlib import Path
from types import SimpleNamespace

import aiohttp
from aiohttp import web
from fastapi import HTTPException


CANDIDATE = '--candidate' in sys.argv
if CANDIDATE:
    sys.argv.remove('--candidate')
    from patch_open_webui import patch_model_requests


class ModelHTTPTests(unittest.IsolatedAsyncioTestCase):
    async def asyncSetUp(self):
        self.old_origin = os.environ.get('NEW_API_STUDIO_ORIGIN')
        self.received = []
        self.response_status = 200
        self.response_html = False
        app = web.Application()
        app.router.add_get('/api/image-studio/v1/models', self.models)
        app.router.add_get('/external/models', self.external)
        self.runner = web.AppRunner(app)
        await self.runner.setup()
        site = web.TCPSite(self.runner, '127.0.0.1', 0)
        await site.start()
        origin = f'http://127.0.0.1:{site._server.sockets[0].getsockname()[1]}'
        os.environ['NEW_API_STUDIO_ORIGIN'] = origin
        self.origin = origin
        helper_path = (
            Path(__file__).with_name('open_webui_bridge.py') if CANDIDATE
            else Path('/app/backend/open_webui/open_webui_bridge.py')
        )
        spec = importlib.util.spec_from_file_location('bridge_under_test', helper_path)
        helper = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(helper)
        path = Path('/app/backend/open_webui/routers/openai.py')
        source = path.read_text(encoding='utf-8')
        if CANDIDATE:
            source = patch_model_requests(source, path)
        names = {
            'send_get_request', 'get_models_request', 'get_headers_and_cookies',
            'get_all_models_responses', 'get_openai_runtime_config',
        }
        nodes = [node for node in ast.parse(source).body if isinstance(node, ast.AsyncFunctionDef) and node.name in names]
        self.assertEqual({node.name for node in nodes}, names)
        self.functions = {
            'aiohttp': aiohttp,
            'asyncio': asyncio,
            'HTTPException': HTTPException,
            'JSONCodec': SimpleNamespace(loads=json.loads),
            'log': logging.getLogger('model-http-test'),
            '_MODEL_LIST_TIMEOUT': aiohttp.ClientTimeout(total=5),
            'AIOHTTP_CLIENT_SESSION_SSL': False,
            'ENABLE_FORWARD_USER_INFO_HEADERS': False,
            'is_anthropic_url': lambda url: False,
            'bridge_enabled': helper.bridge_enabled,
            'bridge_base_url': helper.bridge_base_url,
            'is_bridge_url': helper.is_bridge_url,
            'bridge_headers_and_cookies': helper.bridge_headers_and_cookies,
        }
        exec(compile(ast.Module(body=nodes, type_ignores=[]), str(path), 'exec'), self.functions)

    async def asyncTearDown(self):
        await self.runner.cleanup()
        if self.old_origin is None:
            os.environ.pop('NEW_API_STUDIO_ORIGIN', None)
        else:
            os.environ['NEW_API_STUDIO_ORIGIN'] = self.old_origin

    async def models(self, request):
        self.received.append(request)
        if self.response_status != 200:
            return web.Response(status=self.response_status)
        if self.response_html:
            return web.Response(text='<!doctype html>', content_type='text/html')
        identity = request.headers.get('X-NewAPI-Identity')
        if request.cookies.get('new_api_image_studio') != f'session-{identity}':
            return web.Response(status=401)
        group = request.headers.get('X-NewAPI-Group')
        return web.json_response({'data': [{'id': f'{identity}:{group}'}]})

    async def external(self, request):
        self.received.append(request)
        return web.json_response({'data': []})

    def request(self, identity='alice@test', group='default', cookie=True):
        return SimpleNamespace(
            cookies={
                **({'new_api_image_studio': f'session-{identity}'} if cookie else {}),
                'token': 'webui-token-must-not-leak', 'other': 'unrelated-cookie',
            },
            headers={'X-NewAPI-Group': group, 'X-NewAPI-Identity': 'forged@test', 'Authorization': 'Bearer forged'},
        )

    async def test_actual_model_http_forwards_only_current_account_and_group(self):
        for identity, group in [('alice@test', 'default'), ('alice@test', '%E4%B8%AD%E6%96%87'), ('bob@test', 'vip')]:
            with self.subTest(identity=identity, group=group):
                result = await self.functions['get_all_models_responses'](
                    self.request(identity, group), SimpleNamespace(email=identity),
                )
                self.assertEqual(result[0]['data'][0]['id'], f'{identity}:{group}')
                outbound = self.received[-1]
                self.assertEqual(dict(outbound.cookies), {'new_api_image_studio': f'session-{identity}'})
                self.assertEqual(outbound.headers.get('X-NewAPI-Identity'), identity)
                self.assertEqual(outbound.headers.get('X-NewAPI-Group'), group)
                self.assertNotIn('Authorization', outbound.headers)

    async def test_missing_cookie_and_upstream_errors_are_not_empty_success(self):
        with self.assertRaises(HTTPException) as raised:
            await self.functions['get_all_models_responses'](self.request(cookie=False), SimpleNamespace(email='alice@test'))
        self.assertEqual(raised.exception.status_code, 401)
        self.assertEqual(self.received, [])
        for status in (401, 403, 503):
            with self.subTest(status=status):
                self.response_status = status
                with self.assertRaises(HTTPException) as raised:
                    await self.functions['get_all_models_responses'](self.request(), SimpleNamespace(email='alice@test'))
                self.assertEqual(raised.exception.status_code, status)
        self.response_status = 200
        self.response_html = True
        with self.assertRaises(HTTPException) as raised:
            await self.functions['get_all_models_responses'](self.request(), SimpleNamespace(email='alice@test'))
        self.assertEqual(raised.exception.status_code, 502)

    async def test_other_connection_never_receives_studio_cookie(self):
        result = await self.functions['get_models_request'](
            self.request(), f'{self.origin}/external', '', user=SimpleNamespace(email='alice@test'),
            config={'auth_type': 'none'},
        )
        self.assertEqual(result, {'data': []})
        self.assertEqual(dict(self.received[-1].cookies), {})
        self.assertNotIn('X-NewAPI-Identity', self.received[-1].headers)
        self.assertNotIn('X-NewAPI-Group', self.received[-1].headers)


if __name__ == '__main__':
    unittest.main()
