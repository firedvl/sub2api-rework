import { readFileSync, readdirSync, statSync } from 'node:fs'
import { join, relative } from 'node:path'
import { describe, expect, it } from 'vitest'

// Vue's scoped-style compiler replaces a whole selector with the contents of
// :global(), so `:global(.dark) .tooltip` compiles to a bare `.dark` rule that
// restyles <html>. Write `.dark .tooltip` (scoped) or `:global(.dark .tooltip)`.
const root = join(__dirname, '..')

function vueFiles(dir: string): string[] {
  return readdirSync(dir).flatMap((name) => {
    const path = join(dir, name)
    if (statSync(path).isDirectory()) return name === 'node_modules' ? [] : vueFiles(path)
    return path.endsWith('.vue') ? [path] : []
  })
}

describe('scoped :global() selectors', () => {
  it('never put selectors after :global(...)', () => {
    const offenders = vueFiles(root).flatMap((path) =>
      readFileSync(path, 'utf8')
        .split('\n')
        .map((line, index) => ({ line, index }))
        .filter(({ line }) => /:global\([^)]*\)\s+[^\s{,]/.test(line) && !line.trim().startsWith('/*'))
        .map(({ index }) => `${relative(root, path)}:${index + 1}`),
    )
    expect(offenders).toEqual([])
  })
})
