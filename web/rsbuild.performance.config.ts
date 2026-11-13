import { defineConfig, mergeRsbuildConfig } from '@rsbuild/core'

import config from './rsbuild.groups.config'

export default defineConfig(async (context) =>
  mergeRsbuildConfig(await config(context), {
    output: {
      assetPrefix: '/site/console-performance-20261113/',
    },
  })
)
