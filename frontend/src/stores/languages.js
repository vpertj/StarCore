import { writable } from 'svelte/store'

const builtinLanguageMap = {
  '.go': 'go',
  '.js': 'javascript', '.jsx': 'javascript', '.mjs': 'javascript', '.cjs': 'javascript',
  '.ts': 'typescript', '.tsx': 'typescript',
  '.json': 'json',
  '.html': 'html', '.htm': 'html',
  '.css': 'css', '.scss': 'css', '.sass': 'css', '.less': 'css',
  '.md': 'markdown', '.markdown': 'markdown',
  '.py': 'python', '.pyw': 'python',
  '.rs': 'rust',
  '.java': 'java',
  '.cpp': 'cpp', '.c': 'cpp', '.h': 'cpp', '.hpp': 'cpp',
  '.php': 'php',
  '.sql': 'sql',
  '.xml': 'xml', '.svg': 'xml',
  '.yaml': 'yaml', '.yml': 'yaml',
}

const customLanguageMap = writable(/** @type {Record<string, string>} */ ({}))

export function registerCustomLanguage(extensions, languageId) {
  customLanguageMap.update(map => {
    for (const ext of extensions) {
      const normalized = ext.startsWith('.') ? ext.toLowerCase() : '.' + ext.toLowerCase()
      map[normalized] = languageId
    }
    return { ...map }
  })
}

export function unregisterCustomLanguage(extensions) {
  customLanguageMap.update(map => {
    for (const ext of extensions) {
      const normalized = ext.startsWith('.') ? ext.toLowerCase() : '.' + ext.toLowerCase()
      delete map[normalized]
    }
    return { ...map }
  })
}

export function getLanguageIdFromPath(path) {
  if (!path) return 'plaintext'
  const ext = '.' + (path.split('.').pop()?.toLowerCase() || '')
  const $customMap = get(customLanguageMap)
  if ($customMap[ext]) return $customMap[ext]
  return builtinLanguageMap[ext] || 'plaintext'
}

function get(store) {
  let value = undefined
  store.subscribe(v => value = v)()
  return value
}