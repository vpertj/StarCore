// File type icon definitions based on material-icon-theme (MIT).
// Each icon defines: SVG path, fill color.
export const ICON_THEMES = [
  { id: 'colorful', name: '彩色' },
  { id: 'accent', name: '主题色' },
  { id: 'mono', name: '单色' },
]

export function getIconColor(iconName, iconTheme, themeColors) {
  if (iconTheme === 'accent') return themeColors.accent
  if (iconTheme === 'mono') return themeColors.text
  return FILE_ICONS[iconName]?.color || FILE_ICONS.default.color
}

export function getFolderColor(iconTheme, themeColors, isOpen) {
  if (iconTheme === 'accent') return themeColors.accent
  if (iconTheme === 'mono') return themeColors.text
  return '#dcb67a'
}

export const FILE_ICONS = {
  default: { path: 'M3 1.5h6.5a1 1 0 01.7.3l2.5 2.5a1 1 0 01.3.7v9.5a1 1 0 01-1 1h-8.5a1 1 0 01-1-1v-12a1 1 0 011-1z', color: '#569cd6' },
  folder: { path: 'M1.5 2.5h5.5l1.5 1.5h5.5v9h-12.5z', color: '#dcb67a', openColor: '#dcb67a' },
  go: { path: 'M3 1.5h6.5a1 1 0 01.7.3l2.5 2.5a1 1 0 01.3.7v9.5a1 1 0 01-1 1h-8.5a1 1 0 01-1-1v-12a1 1 0 011-1z', color: '#00add8' },
  javascript: { path: 'M3 1.5h6.5a1 1 0 01.7.3l2.5 2.5a1 1 0 01.3.7v9.5a1 1 0 01-1 1h-8.5a1 1 0 01-1-1v-12a1 1 0 011-1z', color: '#f7df1e' },
  typescript: { path: 'M3 1.5h6.5a1 1 0 01.7.3l2.5 2.5a1 1 0 01.3.7v9.5a1 1 0 01-1 1h-8.5a1 1 0 01-1-1v-12a1 1 0 011-1z', color: '#3178c6' },
  python: { path: 'M3 1.5h6.5a1 1 0 01.7.3l2.5 2.5a1 1 0 01.3.7v9.5a1 1 0 01-1 1h-8.5a1 1 0 01-1-1v-12a1 1 0 011-1z', color: '#3776ab' },
  rust: { path: 'M3 1.5h6.5a1 1 0 01.7.3l2.5 2.5a1 1 0 01.3.7v9.5a1 1 0 01-1 1h-8.5a1 1 0 01-1-1v-12a1 1 0 011-1z', color: '#dea584' },
  java: { path: 'M3 1.5h6.5a1 1 0 01.7.3l2.5 2.5a1 1 0 01.3.7v9.5a1 1 0 01-1 1h-8.5a1 1 0 01-1-1v-12a1 1 0 011-1z', color: '#b07219' },
  cpp: { path: 'M3 1.5h6.5a1 1 0 01.7.3l2.5 2.5a1 1 0 01.3.7v9.5a1 1 0 01-1 1h-8.5a1 1 0 01-1-1v-12a1 1 0 011-1z', color: '#f34b7d' },
  css: { path: 'M3 1.5h6.5a1 1 0 01.7.3l2.5 2.5a1 1 0 01.3.7v9.5a1 1 0 01-1 1h-8.5a1 1 0 01-1-1v-12a1 1 0 011-1z', color: '#563d7c' },
  html: { path: 'M3 1.5h6.5a1 1 0 01.7.3l2.5 2.5a1 1 0 01.3.7v9.5a1 1 0 01-1 1h-8.5a1 1 0 01-1-1v-12a1 1 0 011-1z', color: '#e34c26' },
  json: { path: 'M3 1.5h6.5a1 1 0 01.7.3l2.5 2.5a1 1 0 01.3.7v9.5a1 1 0 01-1 1h-8.5a1 1 0 01-1-1v-12a1 1 0 011-1z', color: '#5c6370' },
  markdown: { path: 'M3 1.5h6.5a1 1 0 01.7.3l2.5 2.5a1 1 0 01.3.7v9.5a1 1 0 01-1 1h-8.5a1 1 0 01-1-1v-12a1 1 0 011-1z', color: '#083fa1' },
  yaml: { path: 'M3 1.5h6.5a1 1 0 01.7.3l2.5 2.5a1 1 0 01.3.7v9.5a1 1 0 01-1 1h-8.5a1 1 0 01-1-1v-12a1 1 0 011-1z', color: '#cb171e' },
  sql: { path: 'M3 1.5h6.5a1 1 0 01.7.3l2.5 2.5a1 1 0 01.3.7v9.5a1 1 0 01-1 1h-8.5a1 1 0 01-1-1v-12a1 1 0 011-1z', color: '#e38c00' },
  shell: { path: 'M3 1.5h6.5a1 1 0 01.7.3l2.5 2.5a1 1 0 01.3.7v9.5a1 1 0 01-1 1h-8.5a1 1 0 01-1-1v-12a1 1 0 011-1z', color: '#89e051' },
  docker: { path: 'M3 1.5h6.5a1 1 0 01.7.3l2.5 2.5a1 1 0 01.3.7v9.5a1 1 0 01-1 1h-8.5a1 1 0 01-1-1v-12a1 1 0 011-1z', color: '#0db7ed' },
  git: { path: 'M3 1.5h6.5a1 1 0 01.7.3l2.5 2.5a1 1 0 01.3.7v9.5a1 1 0 01-1 1h-8.5a1 1 0 01-1-1v-12a1 1 0 011-1z', color: '#f05033' },
  config: { path: 'M3 1.5h6.5a1 1 0 01.7.3l2.5 2.5a1 1 0 01.3.7v9.5a1 1 0 01-1 1h-8.5a1 1 0 01-1-1v-12a1 1 0 011-1z', color: '#6d8086' },
  image: { path: 'M3 1.5h6.5a1 1 0 01.7.3l2.5 2.5a1 1 0 01.3.7v9.5a1 1 0 01-1 1h-8.5a1 1 0 01-1-1v-12a1 1 0 011-1z', color: '#bf3989' },
  font: { path: 'M3 1.5h6.5a1 1 0 01.7.3l2.5 2.5a1 1 0 01.3.7v9.5a1 1 0 01-1 1h-8.5a1 1 0 01-1-1v-12a1 1 0 011-1z', color: '#e2a63b' },
  binary: { path: 'M3 1.5h6.5a1 1 0 01.7.3l2.5 2.5a1 1 0 01.3.7v9.5a1 1 0 01-1 1h-8.5a1 1 0 01-1-1v-12a1 1 0 011-1z', color: '#cc3e44' },
  test: { path: 'M3 1.5h6.5a1 1 0 01.7.3l2.5 2.5a1 1 0 01.3.7v9.5a1 1 0 01-1 1h-8.5a1 1 0 01-1-1v-12a1 1 0 011-1z', color: '#cbcb41' },
  react: { path: 'M3 1.5h6.5a1 1 0 01.7.3l2.5 2.5a1 1 0 01.3.7v9.5a1 1 0 01-1 1h-8.5a1 1 0 01-1-1v-12a1 1 0 011-1z', color: '#61dafb' },
  svelte: { path: 'M3 1.5h6.5a1 1 0 01.7.3l2.5 2.5a1 1 0 01.3.7v9.5a1 1 0 01-1 1h-8.5a1 1 0 01-1-1v-12a1 1 0 011-1z', color: '#ff3e00' },
  vue: { path: 'M3 1.5h6.5a1 1 0 01.7.3l2.5 2.5a1 1 0 01.3.7v9.5a1 1 0 01-1 1h-8.5a1 1 0 01-1-1v-12a1 1 0 011-1z', color: '#4fc08d' },
  readme: { path: 'M3 1.5h6.5a1 1 0 01.7.3l2.5 2.5a1 1 0 01.3.7v9.5a1 1 0 01-1 1h-8.5a1 1 0 01-1-1v-12a1 1 0 011-1z', color: '#083fa1' },
  lock: { path: 'M3 1.5h6.5a1 1 0 01.7.3l2.5 2.5a1 1 0 01.3.7v9.5a1 1 0 01-1 1h-8.5a1 1 0 01-1-1v-12a1 1 0 011-1z', color: '#cc3e44' },
  nodejs: { path: 'M3 1.5h6.5a1 1 0 01.7.3l2.5 2.5a1 1 0 01.3.7v9.5a1 1 0 01-1 1h-8.5a1 1 0 01-1-1v-12a1 1 0 011-1z', color: '#339933' },
  php: { path: 'M3 1.5h6.5a1 1 0 01.7.3l2.5 2.5a1 1 0 01.3.7v9.5a1 1 0 01-1 1h-8.5a1 1 0 01-1-1v-12a1 1 0 011-1z', color: '#777bb4' },
  swift: { path: 'M3 1.5h6.5a1 1 0 01.7.3l2.5 2.5a1 1 0 01.3.7v9.5a1 1 0 01-1 1h-8.5a1 1 0 01-1-1v-12a1 1 0 011-1z', color: '#f05138' },
  ruby: { path: 'M3 1.5h6.5a1 1 0 01.7.3l2.5 2.5a1 1 0 01.3.7v9.5a1 1 0 01-1 1h-8.5a1 1 0 01-1-1v-12a1 1 0 011-1z', color: '#cc342d' },
  csharp: { path: 'M3 1.5h6.5a1 1 0 01.7.3l2.5 2.5a1 1 0 01.3.7v9.5a1 1 0 01-1 1h-8.5a1 1 0 01-1-1v-12a1 1 0 011-1z', color: '#178600' },
  wasm: { path: 'M3 1.5h6.5a1 1 0 01.7.3l2.5 2.5a1 1 0 01.3.7v9.5a1 1 0 01-1 1h-8.5a1 1 0 01-1-1v-12a1 1 0 011-1z', color: '#654ff0' },
  pdf: { path: 'M3 1.5h6.5a1 1 0 01.7.3l2.5 2.5a1 1 0 01.3.7v9.5a1 1 0 01-1 1h-8.5a1 1 0 01-1-1v-12a1 1 0 011-1z', color: '#cc3e44' },
  database: { path: 'M3 1.5h6.5a1 1 0 01.7.3l2.5 2.5a1 1 0 01.3.7v9.5a1 1 0 01-1 1h-8.5a1 1 0 01-1-1v-12a1 1 0 011-1z', color: '#e38c00' },
}

export const EXT_TO_ICON = {
  go: 'go',
  js: 'javascript', mjs: 'javascript', cjs: 'javascript',
  ts: 'typescript', tsx: 'typescript',
  jsx: 'react',
  py: 'python',
  rs: 'rust',
  java: 'java', class: 'java',
  c: 'cpp', cpp: 'cpp', h: 'cpp', hpp: 'cpp', cc: 'cpp', cxx: 'cpp',
  cs: 'csharp',
  css: 'css', scss: 'css', sass: 'css', less: 'css',
  html: 'html', htm: 'html',
  json: 'json',
  md: 'markdown', markdown: 'markdown',
  yaml: 'yaml', yml: 'yaml',
  sql: 'sql',
  sh: 'shell', bash: 'shell', zsh: 'shell', fish: 'shell',
  ps1: 'shell', psd1: 'shell', psm1: 'shell',
  dockerfile: 'docker',
  gitignore: 'git', gitattributes: 'git', gitmodules: 'git',
  xml: 'config', yml: 'config', toml: 'config', ini: 'config', cfg: 'config', conf: 'config',
  png: 'image', jpg: 'image', jpeg: 'image', gif: 'image', svg: 'image', ico: 'image', webp: 'image',
  woff: 'font', woff2: 'font', ttf: 'font', otf: 'font', eot: 'font',
  exe: 'binary', dll: 'binary', so: 'binary', dylib: 'binary',
  test: 'test', spec: 'test',
  svelte: 'svelte',
  vue: 'vue',
  lock: 'lock',
  php: 'php',
  swift: 'swift',
  rb: 'ruby',
  wasm: 'wasm',
  pdf: 'pdf',
  db: 'database', sqlite: 'database', sqlite3: 'database',
}

export const FILENAME_TO_ICON = {
  'readme.md': 'readme',
  'readme': 'readme',
  'package.json': 'nodejs',
  'package-lock.json': 'lock',
  'yarn.lock': 'lock',
  '.gitignore': 'git',
  '.gitattributes': 'git',
  'dockerfile': 'docker',
  'makefile': 'config',
  'go.mod': 'go',
  'go.sum': 'go',
  'tsconfig.json': 'typescript',
  'vite.config.js': 'javascript',
  'vite.config.ts': 'typescript',
  'svelte.config.js': 'svelte',
  '.env': 'config',
  '.env.example': 'config',
  'compose.yaml': 'docker',
  'compose.yml': 'docker',
}

export function getFileName(name) {
  return name.toLowerCase()
}

export function getFileIcon(fileName, isDir) {
  if (isDir) return 'folder'
  const lower = fileName.toLowerCase()
  if (FILENAME_TO_ICON[lower]) return FILENAME_TO_ICON[lower]
  const ext = lower.split('.').pop()
  return EXT_TO_ICON[ext] || 'default'
}

export function getFolderIcon(isOpen) {
  return isOpen ? { ...FILE_ICONS.folder, open: true } : FILE_ICONS.folder
}
