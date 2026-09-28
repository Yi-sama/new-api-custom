"""Pure size/ratio policy tests; no WebUI application, network or data volume."""

import unittest

from open_webui_bridge import (
    HTTPException,
    bridge_image_generation_size as resolve,
    bridge_image_size_options as options,
    bridge_legacy_image_size,
)


class ImageSizeTests(unittest.TestCase):
    def test_explicit_request_then_menu_then_admin_default(self):
        self.assertEqual(resolve('gpt-image-2', {'image_generation_size': '9:16'}, aspect_ratio='4:3'), '1152x864')
        self.assertEqual(resolve('custom', {'image_generation_size': '4:3'}, size='1980*1200'), '1980x1200')
        self.assertEqual(resolve('gpt-image-2', {'image_generation_size': '9:16'}), '864x1536')
        self.assertEqual(resolve('custom', {'image_generation_size': '1980x1200'}), '1980x1200')
        for features in (None, {}, {'image_generation_size': 'auto'}):
            self.assertIsNone(resolve('gpt-image-2', features))

    def test_unicode_and_equivalent_ratios_keep_exact_geometry(self):
        self.assertEqual(resolve('gpt-image-2', aspect_ratio='８：６'), '1152x864')
        self.assertEqual(resolve('custom', size='1980 \u00d7 1200'), '1980x1200')
        self.assertEqual(resolve('gpt-image-1', size='1536 X 1024', aspect_ratio='6:4'), '1536x1024')
        width, height = map(int, resolve('gpt-image-2', aspect_ratio='5:4').split('x'))
        self.assertEqual(width * 4, height * 5)
        self.assertEqual((width % 16, height % 16), (0, 0))

    def test_fixed_model_sizes_do_not_silently_change_ratios(self):
        for model in ('gpt-image-1', 'gpt-image-1.5', 'gpt-image-1-mini'):
            self.assertEqual(resolve(model, aspect_ratio='3:2'), '1536x1024')
            with self.assertRaises(HTTPException) as raised:
                resolve(model, aspect_ratio='4:3')
            self.assertEqual(raised.exception.status_code, 400)
            self.assertIn('confirm', raised.exception.detail)
        self.assertEqual(resolve('dall-e-3', aspect_ratio='7:4'), '1792x1024')
        self.assertEqual(resolve('dall-e-2', size='256x256'), '256x256')

    def test_model_unknown_does_not_invent_ratio_support(self):
        self.assertEqual(options('custom-gpt-image-2'), [])
        self.assertEqual(resolve('custom-gpt-image-2', size='1980x1200'), '1980x1200')
        with self.assertRaises(HTTPException):
            resolve('custom-gpt-image-2', aspect_ratio='4:3')

    def test_pixel_sizes_remain_exact_or_fail_before_request(self):
        self.assertEqual(resolve('gpt-image-2', size='1984x1200'), '1984x1200')
        self.assertEqual(resolve('gpt-image-2', size='3840x2160'), '3840x2160')
        for size in ('1980x1200', '512x512', '3840x3840', '4096x1024', '2048x512'):
            with self.subTest(size=size), self.assertRaises(HTTPException) as raised:
                resolve('gpt-image-2', size=size)
            self.assertIn('No image request', raised.exception.detail)

    def test_malformed_oversized_and_conflicting_dimensions_are_rejected(self):
        for size in (False, 1024, [], {}, '0x1024', '-1x1024', '1024.5x512', '100000000x1', '8193x1', '8192x8192', '1x1\nbad'):
            with self.subTest(size=size), self.assertRaises(HTTPException):
                resolve('custom', size=size)
        for ratio in ('0:1', '1:0', '4/3', '3.2:1', False):
            with self.subTest(ratio=ratio), self.assertRaises(HTTPException):
                resolve('gpt-image-2', aspect_ratio=ratio)
        with self.assertRaises(HTTPException):
            resolve('custom', size='1980x1200', aspect_ratio='4:3')

    def test_offered_options_roundtrip_through_the_same_backend_policy(self):
        for model in ('gpt-image-2', 'gpt-image-1', 'dall-e-2', 'dall-e-3'):
            offered = options(model)
            self.assertGreater(len(offered), 0)
            for option in offered:
                self.assertEqual(resolve(model, aspect_ratio=option['value']), option['size'])
                self.assertEqual(resolve(model, size=option['size']), option['size'])

    def test_legacy_numeric_request_overrides_menu_without_guessing_ambiguity(self):
        self.assertEqual(bridge_legacy_image_size('gpt-image-2', {'image_generation_size': '1:1'}, 'draw 4\uff1a3'), '1152x864')
        self.assertEqual(bridge_legacy_image_size('custom', {}, 'draw 1980*1200 pixels'), '1980x1200')
        self.assertEqual(bridge_legacy_image_size('gpt-image-2', {'image_generation_size': '9:16'}, 'draw a picture'), '864x1536')
        with self.assertRaises(HTTPException):
            bridge_legacy_image_size('gpt-image-2', {}, 'not 4:3, make it 16:9')


if __name__ == '__main__':
    unittest.main()
