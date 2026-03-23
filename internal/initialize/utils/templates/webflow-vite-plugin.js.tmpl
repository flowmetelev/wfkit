import { existsSync } from 'fs'
import fg from 'fast-glob'
import { resolve, dirname, relative } from 'path'

function normalizePath(value) {
  return value.replace(/\\/g, '/')
}

function pageKeyFromFile(pagesDir, file) {
  const relativeDir = normalizePath(relative(pagesDir, dirname(file)))
  return relativeDir === '' ? 'index' : relativeDir
}

export default function webflowVitePlugin(options = {}) {
  const pagesDir = options.pagesDir || 'src/pages'
  const globalEntry = options.globalEntry || 'src/global/index.ts'

  return {
    name: 'webflow-vite-plugin',
    config() {
      const input = {}

      if (existsSync(globalEntry)) {
        input.global = resolve(globalEntry)
      }

      const pageFiles = fg.sync(`${pagesDir}/**/index.{ts,js}`, { onlyFiles: true })
      for (const file of pageFiles) {
        const key = pageKeyFromFile(pagesDir, file)
        input[`pages/${key}`] = resolve(file)
      }

      return {
        build: {
          rollupOptions: {
            input,
            output: {
              entryFileNames(chunkInfo) {
                if (chunkInfo.name === 'global') {
                  return 'global/index-[hash].js'
                }

                if (chunkInfo.name.startsWith('pages/')) {
                  const pageKey = chunkInfo.name.slice('pages/'.length)
                  return `pages/${pageKey}/index-[hash].js`
                }

                return 'chunks/[name]-[hash].js'
              },
              chunkFileNames: 'chunks/[name]-[hash].js',
              assetFileNames: 'assets/[name]-[hash][extname]'
            }
          }
        }
      }
    },

    generateBundle(_, bundle) {
      const manifest = {
        global: '',
        pages: {}
      }

      for (const item of Object.values(bundle)) {
        if (item.type !== 'chunk' || !item.isEntry) {
          continue
        }

        if (item.name === 'global') {
          manifest.global = item.fileName
          continue
        }

        if (item.name.startsWith('pages/')) {
          manifest.pages[item.name.slice('pages/'.length)] = item.fileName
        }
      }

      this.emitFile({
        type: 'asset',
        fileName: 'wfkit-manifest.json',
        source: JSON.stringify(manifest, null, 2)
      })
    }
  }
}
