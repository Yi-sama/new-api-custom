"""Apply the bounded NewAPI bridge patch to Open WebUI v0.11.4 sources."""

from __future__ import annotations

import argparse
import ast
import shutil
import sys
from pathlib import Path


class PatchError(RuntimeError):
    pass


def replace_once(path: Path, source: str, old: str, new: str, label: str) -> str:
    matches = source.count(old)
    if matches != 1:
        raise PatchError(f'{path}: {label} expected 1 match, found {matches}')
    return source.replace(old, new, 1)


def patch_auths(source: str, path: Path) -> str:
    source = replace_once(
        path,
        source,
        "from open_webui.internal.db import get_async_session\n",
        "from open_webui.internal.db import get_async_session\nfrom open_webui.open_webui_bridge import canonical_trusted_name\n",
        'auths bridge import',
    )
    source = replace_once(
        path,
        source,
        """        user = await Auths.authenticate_user_by_email(email, db=db)\n        if user:\n            if WEBUI_AUTH_TRUSTED_GROUPS_HEADER:\n""",
        """        user = await Auths.authenticate_user_by_email(email, db=db)\n        if user:\n            canonical_name = canonical_trusted_name(request, email)\n            if user.name != canonical_name:\n                user = await Users.update_user_by_id(user.id, {'name': canonical_name}, db=db) or user\n\n            if WEBUI_AUTH_TRUSTED_GROUPS_HEADER:\n""",
        'trusted signin name synchronization',
    )
    source = replace_once(
        path,
        source,
        """    if session_user:\n        user = await Users.update_user_by_id(\n            session_user.id,\n            form_data.model_dump(),\n            db=db,\n        )\n""",
        """    if session_user:\n        profile_data = form_data.model_dump()\n        if WEBUI_AUTH_TRUSTED_EMAIL_HEADER:\n            profile_data['name'] = canonical_trusted_name(request, session_user.email)\n        user = await Users.update_user_by_id(\n            session_user.id,\n            profile_data,\n            db=db,\n        )\n""",
        'trusted profile canonical name',
    )
    return replace_once(
        path,
        source,
        "data={'updated_fields': list(form_data.model_dump().keys())},",
        "data={'updated_fields': list(profile_data.keys())},",
        'trusted profile event fields',
    )


def patch_auth(source: str, path: Path) -> str:
    source = replace_once(
        path,
        source,
        "    WEBUI_AUTH_TRUSTED_EMAIL_HEADER,\n",
        "    WEBUI_AUTH_TRUSTED_EMAIL_HEADER,\n    WEBUI_AUTH_TRUSTED_NAME_HEADER,\n",
        'trusted name env import',
    )
    source = replace_once(
        path,
        source,
        "from open_webui.models.users import Users\n",
        "from open_webui.models.users import Users\nfrom open_webui.open_webui_bridge import canonical_trusted_name\n",
        'auth bridge import',
    )
    return replace_once(
        path,
        source,
        """                if WEBUI_AUTH_TRUSTED_EMAIL_HEADER:\n                    trusted_email = request.headers.get(WEBUI_AUTH_TRUSTED_EMAIL_HEADER, '').lower()\n                    if trusted_email and user.email != trusted_email:\n                        raise HTTPException(\n                            status_code=status.HTTP_401_UNAUTHORIZED,\n                            detail='User mismatch. Please sign in again.',\n                        )\n\n                # Add user info to current span\n""",
        """                if WEBUI_AUTH_TRUSTED_EMAIL_HEADER:\n                    trusted_email = request.headers.get(WEBUI_AUTH_TRUSTED_EMAIL_HEADER, '').lower()\n                    if trusted_email and user.email != trusted_email:\n                        raise HTTPException(\n                            status_code=status.HTTP_401_UNAUTHORIZED,\n                            detail='User mismatch. Please sign in again.',\n                        )\n                    if trusted_email and WEBUI_AUTH_TRUSTED_NAME_HEADER:\n                        canonical_name = canonical_trusted_name(request, trusted_email)\n                        if user.name != canonical_name:\n                            user = await Users.update_user_by_id(user.id, {'name': canonical_name}) or user\n\n                # Add user info to current span\n""",
        'trusted current-user name synchronization',
    )


def patch_model_requests(source: str, path: Path) -> str:
    source = replace_once(
        path,
        source,
        """            if request and config:\n                headers, cookies = await get_headers_and_cookies(request, url, key, config, user=user)\n""",
        """            if is_bridge_url(url):\n                headers, cookies = bridge_headers_and_cookies(request, user)\n            elif request and config:\n                headers, cookies = await get_headers_and_cookies(request, url, key, config, user=user)\n""",
        'bridge model request credentials',
    )
    return replace_once(
        path,
        source,
        """            ) as response:\n                return await response.json(loads=JSONCodec.loads)\n    except Exception as e:\n        # Handle connection error here\n        log.error(f'Connection error: {e}')\n        return None\n""",
        """            ) as response:\n                if is_bridge_url(url) and response.status >= 400:\n                    raise HTTPException(status_code=response.status, detail='NewAPI model list unavailable')\n                return await response.json(loads=JSONCodec.loads)\n    except Exception as e:\n        log.error(f'Connection error: {e}')\n        if is_bridge_url(url):\n            if isinstance(e, HTTPException):\n                raise\n            raise HTTPException(status_code=502, detail='NewAPI model list unavailable') from e\n        return None\n""",
        'bridge model errors remain errors',
    )


def patch_openai(source: str, path: Path) -> str:
    source = patch_model_requests(source, path)
    source = replace_once(
        path,
        source,
        "import logging\n",
        "import logging\nfrom functools import wraps\n",
        'openai wraps import',
    )
    source = replace_once(
        path,
        source,
        "from open_webui.internal.db import get_async_session\n",
        """from open_webui.internal.db import get_async_session\nfrom open_webui.open_webui_bridge import (\n    bridge_base_url,\n    bridge_enabled,\n    bridge_headers_and_cookies,\n    is_bridge_url,\n)\n""",
        'openai bridge imports',
    )
    source = replace_once(
        path,
        source,
        "    is_bridge_url,\n)\n",
        "    is_bridge_url,\n    bridge_chat_group_allowed,\n)\n",
        'openai chat group guard import',
    )
    source = replace_once(
        path,
        source,
        """    cookies = getattr(request, 'cookies', {}) if config.get('forward_cookies', False) else {}\n    headers = {\n""",
        """    if is_bridge_url(url):\n        return bridge_headers_and_cookies(request, user)\n\n    cookies = getattr(request, 'cookies', {}) if config.get('forward_cookies', False) else {}\n    headers = {\n""",
        'bounded bridge headers',
    )
    source = replace_once(
        path,
        source,
        """async def get_openai_runtime_config() -> tuple[bool, list[str], list[str], dict]:\n    values = await Config.get_many('openai.enable', 'openai.api_base_urls', 'openai.api_keys', 'openai.api_configs')\n    return (\n        values.get('openai.enable'),\n        values.get('openai.api_base_urls') or [],\n        values.get('openai.api_keys') or [],\n        values.get('openai.api_configs') or {},\n    )\n""",
        """async def get_openai_runtime_config() -> tuple[bool, list[str], list[str], dict]:\n    if bridge_enabled():\n        base_url = bridge_base_url()\n        return True, [base_url], [''], {'0': {'auth_type': 'none', 'enable': True}}\n\n    values = await Config.get_many('openai.enable', 'openai.api_base_urls', 'openai.api_keys', 'openai.api_configs')\n    return (\n        values.get('openai.enable'),\n        values.get('openai.api_base_urls') or [],\n        values.get('openai.api_keys') or [],\n        values.get('openai.api_configs') or {},\n    )\n""",
        'bridge runtime config',
    )
    source = replace_once(
        path,
        source,
        """async def get_models(request: Request, url_idx: int | None = None, user=Depends(get_verified_user)):\n    if url_idx is not None and user.role != 'admin':\n        raise HTTPException(status_code=401, detail=ERROR_MESSAGES.ACCESS_PROHIBITED)\n\n    if not await Config.get('openai.enable'):\n""",
        """async def get_models(request: Request, url_idx: int | None = None, user=Depends(get_verified_user)):\n    if url_idx is not None and user.role != 'admin':\n        raise HTTPException(status_code=401, detail=ERROR_MESSAGES.ACCESS_PROHIBITED)\n\n    if not bridge_enabled() and not await Config.get('openai.enable'):\n""",
        'bridge models route enablement',
    )
    source = replace_once(
        path,
        source,
        """async def generate_chat_completion(\n    request: Request,\n    form_data: dict,\n    user=Depends(get_verified_user),\n):\n    if not await Config.get('openai.enable'):\n""",
        """async def generate_chat_completion(\n    request: Request,\n    form_data: dict,\n    user=Depends(get_verified_user),\n):\n    if not bridge_enabled() and not await Config.get('openai.enable'):\n""",
        'bridge chat enablement',
    )
    source = replace_once(
        path,
        source,
        """    bypass_filter = getattr(request.state, 'bypass_filter', False)\n    if BYPASS_MODEL_ACCESS_CONTROL:\n        bypass_filter = True\n""",
        """    bypass_filter = getattr(request.state, 'bypass_filter', False)\n    if bridge_enabled() or BYPASS_MODEL_ACCESS_CONTROL:\n        bypass_filter = True\n""",
        'bridge chat local access bypass',
    )
    source = replace_once(
        path,
        source,
        """    # Enforce per-model access control\n    await check_model_access(user, await Models.get_model_by_id(model_id), BYPASS_MODEL_ACCESS_CONTROL)\n""",
        """    # Enforce per-model access control\n    await check_model_access(\n        user,\n        await Models.get_model_by_id(model_id),\n        bridge_enabled() or BYPASS_MODEL_ACCESS_CONTROL,\n    )\n""",
        'bridge responses local access bypass',
    )
    source = replace_once(
        path,
        source,
        """@cached(\n    ttl=MODELS_CACHE_TTL,\n    # key_builder (not key) is the per-call hook in aiocache 0.12; `key=` is a\n    # static key, so a `key=lambda` collapsed every caller to one shared entry.\n    key_builder=lambda _func, request, user=None: f'openai_all_models_{user.id}' if user else 'openai_all_models',\n)\nasync def get_all_models(request: Request, user: UserModel) -> dict[str, list]:\n""",
        """def bridge_aware_models_cached(func):\n    cached_func = cached(\n        ttl=MODELS_CACHE_TTL,\n        key_builder=lambda _func, request, user=None: f'openai_all_models_{user.id}' if user else 'openai_all_models',\n    )(func)\n\n    @wraps(func)\n    async def wrapped(request: Request, user: UserModel):\n        if bridge_enabled():\n            return await func(request, user)\n        return await cached_func(request, user)\n\n    wrapped.cache = cached_func.cache\n    return wrapped\n\n\n@bridge_aware_models_cached\nasync def get_all_models(request: Request, user: UserModel) -> dict[str, list]:\n""",
        'bridge-aware model cache',
    )
    source = replace_once(
        path,
        source,
        "async def get_all_models(request: Request, user: UserModel) -> dict[str, list]:\n    log.info('get_all_models()')\n",
        "async def get_all_models(request: Request, user: UserModel) -> dict[str, list]:\n    if bridge_enabled() and not bridge_chat_group_allowed(\n        request, str(await Config.get('image_generation.newapi_group') or '').strip()\n    ):\n        return {'data': []}\n\n    log.info('get_all_models()')\n",
        'bridge image group excluded from chat models',
    )
    source = replace_once(
        path,
        source,
        """    request.app.state.OPENAI_MODELS = models\n    return {'data': list(models.values())}\n""",
        """    if bridge_enabled():\n        known_models = getattr(request.app.state, 'OPENAI_MODELS', {}) or {}\n        request.app.state.OPENAI_MODELS = {**known_models, **models}\n    else:\n        request.app.state.OPENAI_MODELS = models\n    return {'data': list(models.values())}\n""",
        'bridge private model result',
    )
    source = replace_once(
        path,
        source,
        """    # Check if model is already in app state cache to avoid expensive get_all_models() call\n    models = request.app.state.OPENAI_MODELS\n    if not models or model_id not in models:\n        await get_all_models(request, user=user)\n        models = request.app.state.OPENAI_MODELS\n    model = models.get(model_id)\n""",
        """    # Bridge model lists are private to the NewAPI session and group.\n    if bridge_enabled():\n        authorized_models = await get_all_models(request, user=user)\n        if model_id not in {model['id'] for model in authorized_models['data']}:\n            raise HTTPException(status_code=404, detail=ERROR_MESSAGES.MODEL_NOT_FOUND())\n\n    # Check if model is already in app state cache to avoid expensive get_all_models() call\n    models = request.app.state.OPENAI_MODELS\n    if not models or model_id not in models:\n        await get_all_models(request, user=user)\n        models = request.app.state.OPENAI_MODELS\n    model = models.get(model_id)\n""",
        'bridge chat model authorization',
    )
    source = replace_once(
        path,
        source,
        """    if model_id:\n        models = request.app.state.OPENAI_MODELS\n        if not models or model_id not in models:\n            await get_all_models(request, user=user)\n            models = request.app.state.OPENAI_MODELS\n        if model_id in models:\n            idx = models[model_id]['urlIdx']\n\n    url, key, api_config = await get_openai_connection(idx)\n\n    payload['model'] = strip_provider_model_prefix(payload['model'], api_config.get('prefix_id'))\n""",
        """    if model_id:\n        if bridge_enabled():\n            authorized_models = await get_all_models(request, user=user)\n            if model_id not in {model['id'] for model in authorized_models['data']}:\n                raise HTTPException(status_code=404, detail=ERROR_MESSAGES.MODEL_NOT_FOUND())\n\n        models = request.app.state.OPENAI_MODELS\n        if not models or model_id not in models:\n            await get_all_models(request, user=user)\n            models = request.app.state.OPENAI_MODELS\n        if model_id in models:\n            idx = models[model_id]['urlIdx']\n\n    url, key, api_config = await get_openai_connection(idx)\n\n    payload['model'] = strip_provider_model_prefix(payload['model'], api_config.get('prefix_id'))\n""",
        'bridge responses model authorization',
    )
    return replace_once(
        path,
        source,
        """    if model_id:\n        models = request.app.state.OPENAI_MODELS\n        if not models or model_id not in models:\n            await get_all_models(request, user=user)\n            models = request.app.state.OPENAI_MODELS\n        if model_id in models:\n            idx = models[model_id]['urlIdx']\n\n    url, key, api_config = await get_openai_connection(idx)\n    base_url = url\n""",
        """    if model_id:\n        if bridge_enabled():\n            authorized_models = await get_all_models(request, user=user)\n            if model_id not in {model['id'] for model in authorized_models['data']}:\n                raise HTTPException(status_code=404, detail=ERROR_MESSAGES.MODEL_NOT_FOUND())\n\n        models = request.app.state.OPENAI_MODELS\n        if not models or model_id not in models:\n            await get_all_models(request, user=user)\n            models = request.app.state.OPENAI_MODELS\n        if model_id in models:\n            idx = models[model_id]['urlIdx']\n\n    url, key, api_config = await get_openai_connection(idx)\n    base_url = url\n""",
        'bridge passthrough model authorization',
    )


def patch_images(source: str, path: Path) -> str:
    source = replace_once(
        path,
        source,
        "from open_webui.internal.db import get_async_session\n",
        """from open_webui.internal.db import get_async_session\nfrom open_webui.open_webui_bridge import bridge_base_url, bridge_enabled, bridge_headers_and_cookies\n""",
        'images bridge import',
    )
    source = replace_once(
        path,
        source,
        "from open_webui.open_webui_bridge import bridge_base_url, bridge_enabled, bridge_headers_and_cookies\n",
        "from open_webui.open_webui_bridge import bridge_base_url, bridge_enabled, bridge_headers_and_cookies, bridge_image_generation_model, bridge_image_generation_size, bridge_image_size_options\n",
        'images bridge model import',
    )
    source = replace_once(
        path,
        source,
        "    'IMAGE_GENERATION_MODEL': 'image_generation.model',\n",
        "    'IMAGE_GENERATION_MODEL': 'image_generation.model',\n    'NEW_API_IMAGE_GROUP': 'image_generation.newapi_group',\n",
        'newapi image group config key',
    )
    source = replace_once(
        path,
        source,
        "    IMAGE_GENERATION_MODEL: str\n",
        "    IMAGE_GENERATION_MODEL: str\n    NEW_API_IMAGE_GROUP: str = ''\n    NEW_API_BRIDGE_ENABLED: bool = False\n",
        'newapi image group config fields',
    )
    source = replace_once(
        path,
        source,
        "@router.get('/config', response_model=ImagesConfig)\nasync def get_config(request: Request, user=Depends(get_admin_user)):\n    return await get_config_values(IMAGE_CONFIG_KEYS)\n",
        "@router.get('/config', response_model=ImagesConfig)\nasync def get_config(request: Request, user=Depends(get_admin_user)):\n    values = await get_config_values(IMAGE_CONFIG_KEYS)\n    values['NEW_API_BRIDGE_ENABLED'] = bridge_enabled()\n    return values\n",
        'newapi image bridge config response',
    )
    source = replace_once(
        path,
        source,
        "    values = await get_config_values(IMAGE_CONFIG_KEYS)\n    await publish_event(\n",
        "    values = await get_config_values(IMAGE_CONFIG_KEYS)\n    values['NEW_API_BRIDGE_ENABLED'] = bridge_enabled()\n    await publish_event(\n",
        'newapi image bridge update response',
    )
    source = replace_once(
        path,
        source,
        "def get_automatic1111_api_auth(image_config):\n",
        "async def get_newapi_image_group() -> str:\n    return str(await Config.get('image_generation.newapi_group') or '').strip()\n\n\nasync def get_newapi_image_models(request: Request, user, group: str) -> tuple[list, str | None]:\n    headers, cookies = bridge_headers_and_cookies(request, user, group_override=group)\n    session = await get_session()\n    try:\n        async with session.get(url=f'{bridge_base_url()}/models', headers=headers, cookies=cookies, ssl=AIOHTTP_CLIENT_SESSION_SSL) as response:\n            if response.status == 401:\n                raise HTTPException(status_code=401, detail='NewAPI image studio session expired. Please sign in again.')\n            if response.status >= 400:\n                return [], f'Unable to retrieve image models for group {group}.'\n            payload = await response.json(content_type=None)\n    except HTTPException:\n        raise\n    except Exception:\n        log.exception('Failed to retrieve NewAPI image models')\n        return [], f'Unable to retrieve image models for group {group}.'\n    models = payload.get('data') if isinstance(payload, dict) else payload\n    return (models, None) if isinstance(models, list) else ([], f'Unable to retrieve image models for group {group}.')\n\n\n@router.get('/studio/config')\nasync def get_studio_config():\n    return {'group': await get_newapi_image_group()}\n\n\n@router.get('/studio')\nasync def get_studio_models(request: Request, user=Depends(get_verified_user)):\n    group = await get_newapi_image_group()\n    response = {'group': group, 'models': []}\n    if not group or not bridge_enabled():\n        return response\n    models, error = await get_newapi_image_models(request, user, group)\n    if error:\n        response['error'] = error\n    else:\n        response['models'] = models\n    return response\n\n\nasync def require_newapi_image_model(request: Request, user, model: str | None) -> tuple[str, str]:\n    selected_model = bridge_image_generation_model(request, {'image_generation_model': model})\n    group = await get_newapi_image_group()\n    if not group:\n        raise HTTPException(status_code=400, detail='An image generation group has not been configured.')\n    models, error = await get_newapi_image_models(request, user, group)\n    if error:\n        raise HTTPException(status_code=400, detail=error)\n    model_ids = {item if isinstance(item, str) else item.get('id') for item in models if isinstance(item, (str, dict))}\n    if selected_model not in model_ids:\n        raise HTTPException(status_code=400, detail='Selected image generation model is unavailable for the configured image group.')\n    return selected_model, group\n\n\ndef get_automatic1111_api_auth(image_config):\n",
        'newapi image studio routes',
    )
    source = replace_once(
        path, source,
        "        response['models'] = models\n",
        """        response['models'] = [
            {**(item if isinstance(item, dict) else {'id': item}),
             'image_size_options': bridge_image_size_options(item.get('id', '') if isinstance(item, dict) else item)}
            for item in models if isinstance(item, (dict, str))
        ]
""",
        'image model size capabilities',
    )
    source = replace_once(
        path,
        source,
        """        if image_config.IMAGE_GENERATION_ENGINE == 'openai':\n            headers = {\n                'Authorization': f'Bearer {image_config.IMAGES_OPENAI_API_KEY}',\n                'Content-Type': 'application/json',\n            }\n\n            if ENABLE_FORWARD_USER_INFO_HEADERS:\n                headers = include_user_info_headers(headers, user)\n\n            url = f'{image_config.IMAGES_OPENAI_API_BASE_URL}/images/generations'\n""",
        """        if image_config.IMAGE_GENERATION_ENGINE == 'openai':\n            image_base_url = image_config.IMAGES_OPENAI_API_BASE_URL\n            cookies = None\n            if bridge_enabled():\n                image_base_url = bridge_base_url()\n                model = form_data.model or model\n                headers, cookies = bridge_headers_and_cookies(request, user)\n            else:\n                headers = {\n                    'Authorization': f'Bearer {image_config.IMAGES_OPENAI_API_KEY}',\n                    'Content-Type': 'application/json',\n                }\n                if ENABLE_FORWARD_USER_INFO_HEADERS:\n                    headers = include_user_info_headers(headers, user)\n\n            url = f'{image_base_url}/images/generations'\n""",
        'bridge image generation headers',
    )
    source = replace_once(
        path,
        source,
        "                headers, cookies = bridge_headers_and_cookies(request, user)\n",
        "                headers, cookies = bridge_headers_and_cookies(request, user, group_override=bridge_group)\n",
        'bridge image request-local group',
    )
    source = replace_once(
        path,
        source,
        "    model = await get_image_model(request)\n\n    try:\n",
        "    model = await get_image_model(request)\n    bridge_group = None\n    if bridge_enabled() and image_config.IMAGE_GENERATION_ENGINE == 'openai':\n        model, bridge_group = await require_newapi_image_model(request, user, form_data.model)\n\n    try:\n",
        'newapi image model authorization',
    )
    source = replace_once(
        path, source,
        "    size = '512x512'\n",
        "    if bridge_enabled() and image_config.IMAGE_GENERATION_ENGINE == 'openai':\n"
        "        form_data.size = bridge_image_generation_size(form_data.model, size=form_data.size)\n\n"
        "    size = '512x512'\n",
        'image size validation before width and height parsing',
    )
    source = replace_once(
        path,
        source,
        """                url=url,\n                json=data,\n                headers=headers,\n                ssl=AIOHTTP_CLIENT_SESSION_SSL,\n            ) as r:\n""",
        """                url=url,\n                json=data,\n                headers=headers,\n                cookies=cookies,\n                ssl=AIOHTTP_CLIENT_SESSION_SSL,\n            ) as r:\n""",
        'bridge image generation cookie',
    )
    source = replace_once(
        path, source,
        "                **({} if not image_config.IMAGES_OPENAI_API_PARAMS else image_config.IMAGES_OPENAI_API_PARAMS),\n",
        "                **({} if not image_config.IMAGES_OPENAI_API_PARAMS else image_config.IMAGES_OPENAI_API_PARAMS),\n"
        "                **({'size': form_data.size} if bridge_enabled() and form_data.size else {}),\n",
        'explicit image size overrides provider defaults',
    )
    source = replace_once(
        path,
        source,
        """                        IMAGE_URL_RESPONSE_MODELS_REGEX_PATTERN,\n                        image_config.IMAGE_GENERATION_MODEL,\n""",
        """                        IMAGE_URL_RESPONSE_MODELS_REGEX_PATTERN,\n                        model,\n""",
        'bridge effective image response format model',
    )
    source = replace_once(
        path,
        source,
        """                        if _is_same_origin(image_url, image_config.IMAGES_OPENAI_API_BASE_URL)\n""",
        """                        if _is_same_origin(image_url, image_base_url)\n""",
        'bridge image generation origin',
    )
    source = replace_once(
        path, source,
        "async def get_image_data(data: str, headers=None, trusted_base_url: str | None = None):",
        "async def get_image_data(data: str, headers=None, trusted_base_url: str | None = None, *, strict=False):",
        'image result retrieval mode',
    )
    source = replace_once(
        path, source,
        """                data,
                headers=headers,
                ssl=AIOHTTP_CLIENT_SESSION_SSL,
""",
        """                data,
                headers=headers,
                ssl=AIOHTTP_CLIENT_SESSION_SSL,
                **({'timeout': aiohttp.ClientTimeout(total=120, connect=15, sock_read=30)} if strict else {}),
""",
        'bounded image result download',
    )
    source = replace_once(
        path, source,
        """    except Exception as e:
        log.exception(f'Error loading image data: {e}')
        return None, None
""",
        """    except Exception as e:
        if strict:
            log.error('Image result retrieval failed (%s)', type(e).__name__)
            raise
        log.exception(f'Error loading image data: {e}')
        return None, None
""",
        'preserve image download errors',
    )
    source = replace_once(
        path, source,
        """async def image_generations(
    request: Request,
    form_data: CreateImageForm,
    metadata: dict | None = None,
    user=None,
):
""",
        """async def image_generations(
    request: Request,
    form_data: CreateImageForm,
    metadata: dict | None = None,
    user=None,
    *,
    execution: dict | None = None,
    event_emitter=None,
):
    execution = execution if execution is not None else {}
""",
        'image execution display context',
    )
    source = replace_once(
        path, source,
        """            session = await get_session()
            async with session.post(
                url=url,
""",
        """            if bridge_enabled():
                execution.update(model=data['model'], size=data.get('size') or 'auto', n=data['n'], stage='generating')
                if event_emitter:
                    await event_emitter({'type': 'status', 'data': {
                        'description': f"Generating image: {execution['model']} | {execution['size']}",
                        'done': False,
                    }})
            session = await get_session()
            async with session.post(
                url=url,
""",
        'display final image request',
    )
    source = replace_once(
        path, source,
        """            for image in res['data']:
                if image_url := image.get('url', None):
                    image_data, content_type = await get_image_data(
                        image_url,
                        {k: v for k, v in headers.items() if k != 'Content-Type'}
                        if _is_same_origin(image_url, image_base_url)
                        else None,
                    )
                else:
                    image_data, content_type = await get_image_data(image['b64_json'])

                _, image_file = await upload_image(request, image_data, content_type, {**data, **metadata}, user)
                images.append(image_file)
            return images
""",
        """            if bridge_enabled():
                execution['stage'] = 'downloading'
                if not res.get('data'):
                    raise ValueError('No image data returned')
                if event_emitter:
                    await event_emitter({'type': 'status', 'data': {
                        'description': f"Retrieving image: {execution['model']} | {execution['size']}",
                        'done': False,
                    }})
            for image in res['data']:
                if bridge_enabled():
                    execution['stage'] = 'downloading'
                if bridge_enabled() and image.get('b64_json'):
                    image_data, content_type = await get_image_data(image['b64_json'], strict=True)
                elif image_url := image.get('url', None):
                    image_data, content_type = await get_image_data(
                        image_url,
                        {k: v for k, v in headers.items() if k != 'Content-Type'}
                        if _is_same_origin(image_url, image_base_url)
                        else None,
                        strict=bridge_enabled(),
                    )
                else:
                    image_data, content_type = await get_image_data(image['b64_json'], strict=bridge_enabled())

                if bridge_enabled():
                    if not image_data or not content_type:
                        raise ValueError('No usable image data returned')
                    execution['stage'] = 'saving'
                _, image_file = await upload_image(request, image_data, content_type, {**data, **metadata}, user)
                images.append(image_file)
            if bridge_enabled():
                execution['stage'] = 'complete'
            return images
""",
        'prefer inline image data and track delivery',
    )
    source = replace_once(
        path, source,
        """            return images
    except Exception as e:
        error = e
        if isinstance(e, aiohttp.ClientResponseError):
            error = e.message
        raise HTTPException(status_code=400, detail=ERROR_MESSAGES.DEFAULT(error))


class EditImageForm""",
        """            return images
    except Exception as e:
        if bridge_enabled() and execution.get('stage') in ('downloading', 'saving'):
            request.state.newapi_image_result_failed = True
            reason = type(e).__name__
            log.error('Generated image delivery failed at %s (%s)', execution['stage'], reason)
            raise HTTPException(
                status_code=502,
                detail=f"Image generation returned a result, but {execution['stage']} the image failed ({reason}). "
                "The generation may already have been charged. Do not generate again without a new user confirmation.",
            ) from e
        if bridge_enabled() and isinstance(e, HTTPException):
            raise
        error = e
        if isinstance(e, aiohttp.ClientResponseError):
            error = e.message
        raise HTTPException(status_code=400, detail=ERROR_MESSAGES.DEFAULT(error))


class EditImageForm""",
        'specific image delivery error',
    )
    source = replace_once(
        path,
        source,
        """        if image_config.IMAGE_EDIT_ENGINE == 'openai':\n            headers = {\n                'Authorization': f'Bearer {image_config.IMAGES_EDIT_OPENAI_API_KEY}',\n            }\n\n            if ENABLE_FORWARD_USER_INFO_HEADERS:\n                headers = include_user_info_headers(headers, user)\n\n            data = {\n""",
        """        if image_config.IMAGE_EDIT_ENGINE == 'openai':\n            edit_base_url = image_config.IMAGES_EDIT_OPENAI_API_BASE_URL\n            cookies = None\n            if bridge_enabled():\n                edit_base_url = bridge_base_url()\n                headers, cookies = bridge_headers_and_cookies(request, user, content_type=None)\n            else:\n                headers = {'Authorization': f'Bearer {image_config.IMAGES_EDIT_OPENAI_API_KEY}'}\n                if ENABLE_FORWARD_USER_INFO_HEADERS:\n                    headers = include_user_info_headers(headers, user)\n\n            data = {\n""",
        'bridge image edit headers',
    )
    source = replace_once(
        path,
        source,
        """                url=f'{image_config.IMAGES_EDIT_OPENAI_API_BASE_URL}/images/edits{url_search_params}',\n                headers=headers,\n                data=form,\n""",
        """                url=f'{edit_base_url}/images/edits{url_search_params}',\n                headers=headers,\n                cookies=cookies,\n                data=form,\n""",
        'bridge image edit cookie',
    )
    return replace_once(
        path,
        source,
        """                        if _is_same_origin(image_url, image_config.IMAGES_EDIT_OPENAI_API_BASE_URL)\n""",
        """                        if _is_same_origin(image_url, edit_base_url)\n""",
        'bridge image edit origin',
    )


def patch_config(source: str, path: Path) -> str:
    source = replace_once(
        path,
        source,
        "IMAGE_GENERATION_MODEL = os.getenv('IMAGE_GENERATION_MODEL', '')\n",
        "IMAGE_GENERATION_MODEL = os.getenv('IMAGE_GENERATION_MODEL', '')\n\nNEW_API_IMAGE_GROUP = os.getenv('NEW_API_IMAGE_GROUP', '')\n",
        'newapi image group environment default',
    )
    return replace_once(
        path,
        source,
        "    'image_generation.model': IMAGE_GENERATION_MODEL,\n",
        "    'image_generation.model': IMAGE_GENERATION_MODEL,\n    'image_generation.newapi_group': NEW_API_IMAGE_GROUP,\n",
        'newapi image group persistent default',
    )


def patch_builtin_tools(source: str, path: Path) -> str:
    source = replace_once(
        path,
        source,
        """from open_webui.routers.images import (\n    CreateImageForm,\n""",
        """from open_webui.open_webui_bridge import bridge_enabled, bridge_image_generation_model, bridge_image_generation_size\nfrom open_webui.routers.images import (\n    CreateImageForm,\n""",
        'builtin image bridge imports',
    )
    node = next(node for node in ast.parse(source).body if isinstance(node, ast.AsyncFunctionDef) and node.name == 'generate_image')
    lines = source.splitlines(keepends=True)
    prefix, suffix = ''.join(lines[:node.lineno - 1]), ''.join(lines[node.end_lineno:])
    source = ''.join(lines[node.lineno - 1:node.end_lineno])
    source = replace_once(
        path,
        source,
        """async def generate_image(
    prompt: str,
    __request__: Request = None,
    __user__: dict = None,
""",
        """async def generate_image(
    prompt: str,
    size: str | None = None,
    aspect_ratio: str | None = None,
    __request__: Request = None,
    __user__: dict = None,
    __features__: dict | None = None,
""",
        'builtin image feature context',
    )
    source = replace_once(
        path, source,
        "    Generate an image based on a text prompt.\n\n    :param prompt: A detailed description of the image to generate\n",
        """    Generate an image based on a text prompt.
    Only set size or aspect_ratio when the user's current image request explicitly
    specifies dimensions (including an explicit reference to earlier dimensions).
    Otherwise omit both; the user's size menu and then the administrator default apply.
    Never invent dimensions from the subject, old unrelated messages, or a tool error.
    If dimensions are unsupported, explain the error and wait for user confirmation.
    Do not silently substitute another size, aspect ratio, model, crop, or retry.

    :param prompt: A detailed description of the image to generate
    :param size: Exact requested width x height in pixels, e.g. 1980x1200. Omit unless explicitly requested.
    :param aspect_ratio: Requested width:height ratio, e.g. 4:3 or 9:16. Omit unless explicitly requested.
""",
        'image size tool contract',
    )
    source = replace_once(
        path,
        source,
        """        images = await image_generations(\n            request=__request__,\n            form_data=CreateImageForm(prompt=prompt),\n""",
        """        image_model = bridge_image_generation_model(__request__, __features__) if bridge_enabled() else None
        image_size = size
        if bridge_enabled():
            if getattr(__request__.state, 'newapi_image_result_failed', False):
                raise HTTPException(status_code=409, detail='The previous image result could not be delivered and may already have been charged. Wait for a new user message before generating again.')
            if getattr(__request__.state, 'newapi_image_size_confirmation', None):
                raise HTTPException(status_code=400, detail='Wait for a new user message confirming image dimensions before generating again.')
            try:
                image_size = bridge_image_generation_size(image_model, __features__, size, aspect_ratio)
            except HTTPException:
                __request__.state.newapi_image_size_confirmation = True
                raise
        images = await image_generations(
            request=__request__,
            form_data=CreateImageForm(prompt=prompt, model=image_model, size=image_size),
            execution=image_request,
            event_emitter=__event_emitter__,
""",
        'builtin request image model',
    )
    source = replace_once(
        path, source,
        "    try:\n        user = UserModel(**__user__) if __user__ else None\n\n        image_model =",
        "    image_request = {}\n    try:\n        user = UserModel(**__user__) if __user__ else None\n\n        image_model =",
        'builtin image execution snapshot',
    )
    source = replace_once(
        path, source,
        """        # Prepare file entries for the images
""",
        """        if __event_emitter__ and image_request:
            await __event_emitter__({'type': 'status', 'data': {
                'description': f"Image ready: {image_request['model']} | {image_request['size']}",
                'done': True,
            }})

        # Prepare file entries for the images
""",
        'completed image execution status',
    )
    source = replace_once(
        path, source,
        """                    'images': images,
""",
        """                    'images': images,
                    'image_request': image_request,
""",
        'emitted image execution result',
    )
    source = replace_once(
        path, source,
        "        return JSONCodec.dumps({'status': 'success', 'images': images}, ensure_ascii=False)",
        "        return JSONCodec.dumps({'status': 'success', 'images': images, 'image_request': image_request}, ensure_ascii=False)",
        'image execution result',
    )
    source = replace_once(
        path, source,
        """        log.exception(f'generate_image error: {e}')
        return JSONCodec.dumps({'error': str(e)})""",
        """        log.exception(f'generate_image error: {e}')
        if __event_emitter__ and image_request:
            await __event_emitter__({'type': 'status', 'data': {
                'description': f"Image failed ({image_request['stage']}): {image_request['model']} | {image_request['size']}",
                'done': True,
            }})
        return JSONCodec.dumps({'error': str(e), 'image_request': image_request})""",
        'failed image execution result',
    )
    return prefix + source + suffix


def patch_tools(source: str, path: Path) -> str:
    return replace_once(
        path,
        source,
        "                '__files__': chat_files,\n",
        "                '__features__': features,\n                '__files__': chat_files,\n",
        'builtin request feature binding',
    )


def patch_middleware(source: str, path: Path) -> str:
    source = replace_once(
        path,
        source,
        "from open_webui.routers.images import (\n",
        "from open_webui.open_webui_bridge import bridge_enabled, bridge_image_generation_model, bridge_legacy_image_size\nfrom open_webui.routers.images import (\n",
        'middleware image bridge imports',
    )
    source = replace_once(
        path,
        source,
        """    features = form_data.pop('features', None) or {}\n    extra_params['__features__'] = features\n    if features:\n""",
        """    features = form_data.pop('features', None) or {}\n    extra_params['__features__'] = features\n    if features:\n""",
        'bridge image selection preflight',
    )
    return replace_once(
        path,
        source,
        """            images = await image_generations(\n                request=request,\n                form_data=CreateImageForm(**{'prompt': prompt}),\n""",
        """            image_model = bridge_image_generation_model(request, extra_params.get('__features__')) if bridge_enabled() else None\n            image_size = bridge_legacy_image_size(image_model, extra_params.get('__features__'), user_message) if bridge_enabled() else None\n            images = await image_generations(\n                request=request,\n                form_data=CreateImageForm(**{'prompt': prompt, 'model': image_model, 'size': image_size}),\n""",
        'legacy request image model',
    )


def patch_models(source: str, path: Path) -> str:
    source = replace_once(
        path,
        source,
        "from open_webui.models.users import UserModel\n",
        "from open_webui.models.users import UserModel\nfrom open_webui.open_webui_bridge import bridge_enabled\n",
        'models bridge import',
    )
    source = replace_once(
        path,
        source,
        """async def get_all_base_models(request: Request, user: UserModel = None):\n    config = await Config.get_many('openai.enable', 'ollama.enable')\n""",
        """async def get_all_base_models(request: Request, user: UserModel = None):\n    if bridge_enabled():\n        return await fetch_openai_models(request, user=user)\n\n    config = await Config.get_many('openai.enable', 'ollama.enable')\n""",
        'bridge base model isolation',
    )
    source = replace_once(
        path,
        source,
        "use_cache = config.get('models.base_models_cache') and not refresh",
        "use_cache = config.get('models.base_models_cache') and not refresh and not bridge_enabled()",
        'bridge base model cache bypass',
    )
    source = replace_once(
        path,
        source,
        """    if base_models is None:\n        base_models = await get_all_base_models(request, user=user)\n        if base_models:\n            request.app.state.BASE_MODELS = base_models\n            if config.get('models.base_models_cache') and redis is not None:\n                await redis.set(BASE_MODELS_CACHE_KEY, JSONCodec.dumps(base_models))\n        else:\n            base_models = request.app.state.BASE_MODELS\n""",
        """    if base_models is None:\n        base_models = await get_all_base_models(request, user=user)\n        if base_models:\n            if not bridge_enabled():\n                request.app.state.BASE_MODELS = base_models\n                if config.get('models.base_models_cache') and redis is not None:\n                    await redis.set(BASE_MODELS_CACHE_KEY, JSONCodec.dumps(base_models))\n        elif bridge_enabled():\n            base_models = []\n        else:\n            base_models = request.app.state.BASE_MODELS\n""",
        'bridge empty base model fail closed',
    )
    source = replace_once(
        path,
        source,
        """    if len(models) == 0:\n        return []\n""",
        """    if len(models) == 0:\n        if bridge_enabled():\n            request.state.newapi_bridge_models = {}\n        return []\n""",
        'bridge empty request registry',
    )
    source = replace_once(
        path,
        source,
        """    # Add arena models\n    if config.get('evaluation.arena.enable'):\n""",
        """    # Add arena models\n    if config.get('evaluation.arena.enable') and not bridge_enabled():\n""",
        'bridge arena model isolation',
    )
    source = replace_once(
        path,
        source,
        """            if base_model:\n                owned_by = base_model.get('owned_by', 'unknown')\n                if 'pipe' in base_model:\n                    pipe = base_model['pipe']\n                connection_type = base_model.get('connection_type', None)\n\n            model = {\n""",
        """            if base_model:\n                owned_by = base_model.get('owned_by', 'unknown')\n                if 'pipe' in base_model:\n                    pipe = base_model['pipe']\n                connection_type = base_model.get('connection_type', None)\n            elif bridge_enabled():\n                continue\n\n            model = {\n""",
        'bridge custom preset authorization',
    )
    return replace_once(
        path,
        source,
        """    if isinstance(request.app.state.MODELS, RedisDict):\n        try:\n            request.app.state.MODELS.set(models_dict)\n        except Exception as e:\n            log.warning(f'Failed to update Redis model cache, using in-process cache: {e}')\n            request.app.state.MODELS = models_dict\n    else:\n        request.app.state.MODELS = models_dict\n\n    return models\n""",
        """    if bridge_enabled():\n        request.state.newapi_bridge_models = models_dict\n        known_models = {**dict(request.app.state.MODELS), **models_dict}\n        if isinstance(request.app.state.MODELS, RedisDict):\n            try:\n                request.app.state.MODELS.set(known_models)\n            except Exception as e:\n                log.warning(f'Failed to update Redis model cache, using in-process cache: {e}')\n                request.app.state.MODELS = known_models\n        else:\n            request.app.state.MODELS = known_models\n    elif isinstance(request.app.state.MODELS, RedisDict):\n        try:\n            request.app.state.MODELS.set(models_dict)\n        except Exception as e:\n            log.warning(f'Failed to update Redis model cache, using in-process cache: {e}')\n            request.app.state.MODELS = models_dict\n    else:\n        request.app.state.MODELS = models_dict\n\n    return models\n""",
        'bridge global model registry merge',
    )


def patch_main(source: str, path: Path) -> str:
    source = replace_once(
        path,
        source,
        "from open_webui.constants import ERROR_MESSAGES, TASKS\n",
        "from open_webui.constants import ERROR_MESSAGES, TASKS\nfrom open_webui.open_webui_bridge import bridge_enabled\n",
        'main bridge import',
    )
    source = replace_once(
        path,
        source,
        """    if not request.app.state.MODELS:\n        await get_all_models(request, user=user)\n\n    model_id = form_data.get('model', None)\n""",
        """    if bridge_enabled() or not request.app.state.MODELS:\n        await get_all_models(request, user=user)\n\n    model_registry = (\n        getattr(request.state, 'newapi_bridge_models', {})\n        if bridge_enabled()\n        else request.app.state.MODELS\n    )\n\n    model_id = form_data.get('model', None)\n""",
        'bridge chat request registry',
    )
    source = replace_once(
        path,
        source,
        """            if model_id not in request.app.state.MODELS:\n                raise Exception('Model not found')\n\n            model = request.app.state.MODELS[model_id]\n            model_info = await Models.get_model_by_id(model_id)\n            missing_base_model = bool(\n                model_info and model_info.base_model_id and model_info.base_model_id not in request.app.state.MODELS\n            )\n""",
        """            if model_id not in model_registry:\n                raise Exception('Model not found')\n\n            model = model_registry[model_id]\n            model_info = await Models.get_model_by_id(model_id)\n            missing_base_model = bool(\n                model_info and model_info.base_model_id and model_info.base_model_id not in model_registry\n            )\n""",
        'bridge chat private model lookup',
    )
    source = replace_once(
        path,
        source,
        "fallback_model = request.app.state.MODELS.get(fallback_model_id)",
        "fallback_model = model_registry.get(fallback_model_id)",
        'bridge chat fallback registry',
    )
    source = replace_once(
        path,
        source,
        """            if not BYPASS_MODEL_ACCESS_CONTROL and (user.role != 'admin' or not BYPASS_ADMIN_ACCESS_CONTROL):\n                try:\n""",
        """            if not bridge_enabled() and not BYPASS_MODEL_ACCESS_CONTROL and (user.role != 'admin' or not BYPASS_ADMIN_ACCESS_CONTROL):\n                try:\n""",
        'bridge main local access bypass',
    )
    source = replace_once(
        path,
        source,
        "resolved_model = request.app.state.MODELS.get(target_model_id, model)",
        "resolved_model = model_registry.get(target_model_id, model)",
        'bridge multi-model registry',
    )
    source = replace_once(
        path,
        source,
        """    passthrough_params = []\n    models = request.app.state.OPENAI_MODELS\n    if not models or model_id not in models:\n        await openai.get_all_models(request, user=user)\n        models = request.app.state.OPENAI_MODELS\n    model = models.get(model_id)\n""",
        """    passthrough_params = []\n    if bridge_enabled():\n        models = {model['id']: model for model in (await openai.get_all_models(request, user=user))['data']}\n    else:\n        models = request.app.state.OPENAI_MODELS\n        if not models or model_id not in models:\n            await openai.get_all_models(request, user=user)\n            models = request.app.state.OPENAI_MODELS\n    model = models.get(model_id)\n""",
        'bridge anthropic request model authorization',
    )
    return replace_once(
        path,
        source,
        """    models = await get_filtered_models(models, user)\n""",
        """    if not bridge_enabled():\n        models = await get_filtered_models(models, user)\n""",
        'bridge models route local filter bypass',
    )


def apply_patch(source_root: Path, helper: Path) -> None:
    package = source_root / 'open_webui'
    if not package.is_dir():
        raise PatchError(f'missing Open WebUI package: {package}')
    if not helper.is_file():
        raise PatchError(f'missing bridge helper: {helper}')

    targets = {
        package / 'config.py': patch_config,
        package / 'routers' / 'auths.py': patch_auths,
        package / 'utils' / 'auth.py': patch_auth,
        package / 'routers' / 'openai.py': patch_openai,
        package / 'routers' / 'images.py': patch_images,
        package / 'tools' / 'builtin.py': patch_builtin_tools,
        package / 'utils' / 'tools.py': patch_tools,
        package / 'utils' / 'middleware.py': patch_middleware,
        package / 'utils' / 'models.py': patch_models,
        package / 'main.py': patch_main,
    }
    updated = {}
    for path, patcher in targets.items():
        if not path.is_file():
            raise PatchError(f'missing expected v0.11.4 source: {path}')
        updated[path] = patcher(path.read_text(encoding='utf-8'), path)

    for path, source in updated.items():
        path.write_text(source, encoding='utf-8')
    shutil.copyfile(helper, package / 'open_webui_bridge.py')


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument('--source-root', type=Path, default=Path('/app/backend'))
    parser.add_argument('--helper', type=Path, default=Path(__file__).with_name('open_webui_bridge.py'))
    args = parser.parse_args(argv)
    try:
        apply_patch(args.source_root, args.helper)
    except PatchError as exc:
        print(f'Open WebUI bridge patch failed: {exc}', file=sys.stderr)
        return 1
    return 0


if __name__ == '__main__':
    raise SystemExit(main())
