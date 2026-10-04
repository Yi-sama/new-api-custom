"""Bounded Open WebUI helpers for the NewAPI image-studio bridge."""

from __future__ import annotations

import os
import math
import re
import unicodedata
from urllib.parse import unquote, urlsplit

try:
    from fastapi import HTTPException, status
except ModuleNotFoundError:  # Allows the focused stdlib test to run outside the image.
    class HTTPException(Exception):
        def __init__(self, status_code: int, detail: str):
            self.status_code = status_code
            self.detail = detail
            super().__init__(detail)

    class status:
        HTTP_400_BAD_REQUEST = 400
        HTTP_401_UNAUTHORIZED = 401


BRIDGE_ORIGIN_ENV = 'NEW_API_STUDIO_ORIGIN'
BRIDGE_COOKIE = 'new_api_image_studio'
GROUP_HEADER = 'X-NewAPI-Group'
EXPECTED_IDENTITY_HEADER = 'X-NewAPI-Identity'


def bridge_origin() -> str | None:
    value = os.getenv(BRIDGE_ORIGIN_ENV, '').strip().rstrip('/')
    return value or None


def bridge_enabled() -> bool:
    return bridge_origin() is not None


def bridge_base_url() -> str | None:
    origin = bridge_origin()
    return f'{origin}/api/image-studio/v1' if origin else None


def is_bridge_url(url: str | None) -> bool:
    base_url = bridge_base_url()
    if not base_url or not url:
        return False
    actual = urlsplit(url)
    expected = urlsplit(base_url)
    return (
        actual.scheme.lower() == expected.scheme.lower()
        and actual.netloc.lower() == expected.netloc.lower()
        and actual.path.rstrip('/') in (
            expected.path.rstrip('/'),
            f"{expected.path.rstrip('/')}/models",
        )
    )


def canonical_trusted_name(request, fallback: str) -> str:
    value = request.headers.get('X-Studio-Name', fallback)
    try:
        return unquote(value, encoding='utf-8')
    except Exception:
        return fallback


def _safe_header_value(value: str) -> str:
    if '\r' in value or '\n' in value:
        raise HTTPException(status_code=status.HTTP_400_BAD_REQUEST, detail='Invalid bridge header')
    return value


def bridge_image_generation_model(request, features) -> str:
    model = features.get('image_generation_model') if isinstance(features, dict) else None
    if not isinstance(model, str) or not (model := model.strip()):
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail='Select an image generation model before generating an image.',
        )
    model = _safe_header_value(model)

    return model


# Exact model IDs only: custom aliases do not establish upstream capabilities.
# OpenAI Images API size contract, checked 2026-09-28.
_CUSTOM_SIZE_MODELS = {'gpt-image-2'}
_FIXED_IMAGE_SIZES = {
    'gpt-image-1': ('1024x1024', '1536x1024', '1024x1536'),
    'gpt-image-1-mini': ('1024x1024', '1536x1024', '1024x1536'),
    'gpt-image-1.5': ('1024x1024', '1536x1024', '1024x1536'),
    'dall-e-2': ('1024x1024', '512x512', '256x256'),
    'dall-e': ('1024x1024', '512x512', '256x256'),
    'dall-e-3': ('1024x1024', '1792x1024', '1024x1792'),
}
# Web adapter sizes are request hints, not guaranteed output dimensions.
_WEB_IMAGE_MODEL = 'gpt-image-2.5-web'
_WEB_IMAGE_DEFAULT_SIZE = '512x512'


def bridge_image_size_options(model: str) -> list[dict]:
    if model == _WEB_IMAGE_MODEL:
        return [{'value': '1:1', 'size': _WEB_IMAGE_DEFAULT_SIZE}]
    sizes = (
        ('1024x1024', '1536x1024', '1024x1536', '1152x864', '864x1152', '1536x864', '864x1536')
        if model in _CUSTOM_SIZE_MODELS else _FIXED_IMAGE_SIZES.get(model, ())
    )
    options = {}
    for size in sizes:
        width, height = map(int, size.split('x'))
        divisor = math.gcd(width, height)
        ratio = f'{width // divisor}:{height // divisor}'
        options.setdefault(ratio, {'value': ratio, 'size': size})
    return list(options.values())


def bridge_image_generation_size(model: str, features=None, size=None, aspect_ratio=None) -> str | None:
    """Explicit tool dimensions override the menu; None leaves the admin default."""
    def parse(value, pattern, label):
        if not isinstance(value, str) or len(value) > 40:
            raise HTTPException(status_code=400, detail=f'Invalid image {label}.')
        match = re.fullmatch(pattern, unicodedata.normalize('NFKC', value).strip())
        if not match:
            raise HTTPException(status_code=400, detail=f'Invalid image {label}: {value}.')
        return tuple(map(int, match.groups()))

    if model == _WEB_IMAGE_MODEL and size == 'auto':
        size = None
    explicit = size not in (None, '') or aspect_ratio not in (None, '')
    if not explicit:
        selection = features.get('image_generation_size') if isinstance(features, dict) else None
        if selection in (None, '', 'auto'):
            return _WEB_IMAGE_DEFAULT_SIZE if model == _WEB_IMAGE_MODEL else None
        if isinstance(selection, str) and (':' in selection or '\uff1a' in selection):
            aspect_ratio = selection
        else:
            size = selection

    ratio = None
    if aspect_ratio not in (None, ''):
        ratio = parse(aspect_ratio, r'([1-9][0-9]{0,4})\s*:\s*([1-9][0-9]{0,4})', 'aspect ratio')
        divisor = math.gcd(*ratio)
        ratio = (ratio[0] // divisor, ratio[1] // divisor)

    alternatives = ', '.join(f'{item["value"]} ({item["size"]})' for item in bridge_image_size_options(model))
    confirmation = ' No image request was sent. Ask the user to confirm a supported size or another model; do not change dimensions or retry automatically.'
    if size not in (None, ''):
        width, height = parse(size, r'([1-9][0-9]{0,4})\s*[xX*\u00d7]\s*([1-9][0-9]{0,4})', 'size')
        if ratio and width * ratio[1] != height * ratio[0]:
            raise HTTPException(status_code=400, detail='Image size and aspect ratio conflict.' + confirmation)
    elif ratio:
        matched = next((item for item in bridge_image_size_options(model) if item['value'] == f'{ratio[0]}:{ratio[1]}'), None)
        if matched:
            width, height = map(int, matched['size'].split('x'))
        elif model in _CUSTOM_SIZE_MODELS and max(ratio) <= 3 * min(ratio):
            # Reduced ratios require a common multiple of 16 to preserve both edges exactly.
            unit_pixels = ratio[0] * ratio[1] * 256
            multiplier = max(1, round(math.sqrt(1048576 / unit_pixels)), math.ceil(math.sqrt(655360 / unit_pixels)))
            width, height = ratio[0] * 16 * multiplier, ratio[1] * 16 * multiplier
        else:
            detail = f'Aspect ratio {ratio[0]}:{ratio[1]} is not established for {model}.'
            detail += f' Supported choices: {alternatives}.' if alternatives else ' Specify exact pixel dimensions; model capabilities are unknown.'
            raise HTTPException(status_code=400, detail=detail + confirmation)
    else:
        return None

    normalized = f'{width}x{height}'
    if width > 8192 or height > 8192 or width * height > 33554432:
        raise HTTPException(status_code=400, detail='Image size exceeds Studio limits (8192 per edge, 33554432 total pixels).' + confirmation)
    if model in _CUSTOM_SIZE_MODELS:
        if (width % 16 or height % 16 or max(width, height) > 3840
                or max(width, height) > 3 * min(width, height)
                or not 655360 <= width * height <= 8294400):
            raise HTTPException(
                status_code=400,
                detail=f'{model} cannot use {normalized}: edges must be divisible by 16 and at most 3840; aspect ratio must be 1:3 to 3:1; pixels must total 655360 to 8294400. Supported examples: {alternatives}.' + confirmation,
            )
    elif model in _FIXED_IMAGE_SIZES and normalized not in _FIXED_IMAGE_SIZES[model]:
        raise HTTPException(status_code=400, detail=f'{model} does not support {normalized}. Supported sizes: {", ".join(_FIXED_IMAGE_SIZES[model])}.' + confirmation)
    return normalized


def bridge_legacy_image_size(model: str, features, user_message: str) -> str | None:
    """Legacy mode has no tool arguments; accept only unambiguous numeric requests."""
    text = unicodedata.normalize('NFKC', user_message or '')
    matches = list(re.finditer(r'(?<![\d.])([1-9][0-9]{0,4})\s*([xX*\u00d7:])\s*([1-9][0-9]{0,4})(?![\d.])', text))
    if len(matches) > 1:
        raise HTTPException(status_code=400, detail='Multiple image dimensions were mentioned. Confirm one size or use native function calling; no image request was sent.')
    if matches:
        width, separator, height = matches[0].groups()
        if separator == ':':
            return bridge_image_generation_size(model, features, aspect_ratio=f'{width}:{height}')
        return bridge_image_generation_size(model, features, size=f'{width}x{height}')
    return bridge_image_generation_size(model, features)


def bridge_chat_group_allowed(request, image_group: str) -> bool:
    if not image_group:
        return True
    selected_group = request.headers.get(GROUP_HEADER, '').strip()
    try:
        selected_group = unquote(selected_group, encoding='utf-8')
    except Exception:
        return False
    return bool(selected_group) and selected_group != image_group


def bridge_headers_and_cookies(
    request,
    user,
    content_type: str | None = 'application/json',
    group_override: str | None = None,
) -> tuple[dict, dict]:
    cookie = request.cookies.get(BRIDGE_COOKIE)
    if not cookie:
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail='NewAPI image studio session expired. Please sign in again.',
        )

    email = _safe_header_value(str(getattr(user, 'email', '')).strip().lower())
    if not email:
        raise HTTPException(status_code=status.HTTP_401_UNAUTHORIZED, detail='Not authenticated')

    headers = {EXPECTED_IDENTITY_HEADER: email}
    if content_type:
        headers['Content-Type'] = content_type

    group = group_override if group_override is not None else request.headers.get(GROUP_HEADER, '')
    group = group.strip()
    if group:
        headers[GROUP_HEADER] = _safe_header_value(group)

    return headers, {BRIDGE_COOKIE: cookie}
