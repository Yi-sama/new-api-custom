import { defineConfig, mergeRsbuildConfig } from '@rsbuild/core'

import config from './rsbuild.config'

export default defineConfig(async (context) =>
  mergeRsbuildConfig(await config(context), {
    tools: {
      rspack: {
        resolve: { symlinks: false },
      },
    },
  })
)
