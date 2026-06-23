import { writable, get } from 'svelte/store'
import { KEYS } from './constants.js'

export const currentProject = writable(/** @type {string|null} */ (localStorage.getItem(KEYS.LAST_PROJECT) || null))

currentProject.subscribe(v => {
  try {
    if (v) localStorage.setItem(KEYS.LAST_PROJECT, v)
    else localStorage.removeItem(KEYS.LAST_PROJECT)
  } catch {}
})

export const openProjects = writable(/** @type {string[]} */ ([]))

try {
  const saved = localStorage.getItem(KEYS.OPEN_PROJECTS)
  if (saved) openProjects.set(JSON.parse(saved))
} catch {}

openProjects.subscribe(v => {
  try {
    localStorage.setItem(KEYS.OPEN_PROJECTS, JSON.stringify(v))
  } catch {}
})

export async function openProjectFolder() {
  try {
    const folder = await window.backend.OpenFolder()
    if (folder) {
      currentProject.set(folder)
      if (window.backend.SetProjectPath) {
        window.backend.SetProjectPath(folder)
      }
      addRecentProject(folder)
      addOpenProject(folder)
    }
    return folder
  } catch (e) {
    console.error('Failed to open folder:', e)
    return null
  }
}

export function openProjectPath(path) {
  if (!path) return
  currentProject.set(path)
  if (window.backend.SetProjectPath) {
    window.backend.SetProjectPath(path)
  }
  addRecentProject(path)
  addOpenProject(path)
}

function addOpenProject(path) {
  openProjects.update(list => {
    if (list.includes(path)) return list
    return [...list, path]
  })
}

export function switchProject(path) {
  if (!path) return
  currentProject.set(path)
  if (window.backend.SwitchProject) {
    window.backend.SwitchProject(path)
  }
}

export function closeProject(path) {
  openProjects.update(list => list.filter(p => p !== path))
  if (window.backend.CloseProject) {
    window.backend.CloseProject(path)
  }
  if (get(currentProject) === path) {
    const remaining = get(openProjects)
    if (remaining.length > 0) {
      switchProject(remaining[0])
    } else {
      currentProject.set(null)
    }
  }
}

export const recentProjects = writable(/** @type {string[]} */ ([]))

try {
  const saved = localStorage.getItem(KEYS.RECENT_PROJECTS)
  if (saved) recentProjects.set(JSON.parse(saved))
} catch {}

recentProjects.subscribe(v => {
  try {
    localStorage.setItem(KEYS.RECENT_PROJECTS, JSON.stringify(v))
  } catch {}
})

function addRecentProject(path) {
  recentProjects.update(list => {
    const filtered = list.filter(p => p !== path)
    filtered.unshift(path)
    return filtered.slice(0, 10)
  })
}

export function removeRecentProject(path) {
  recentProjects.update(list => list.filter(p => p !== path))
}
export const openedFiles = writable(/** @type {Array<{path: string, name: string, pinned?: boolean}>} */ ([]))

// Restore openedFiles from localStorage
try {
  const savedFiles = localStorage.getItem(KEYS.OPENED_FILES)
  if (savedFiles) {
    const parsed = JSON.parse(savedFiles)
    if (Array.isArray(parsed)) openedFiles.set(parsed)
  }
} catch {}

openedFiles.subscribe(v => {
  try {
    localStorage.setItem(KEYS.OPENED_FILES, JSON.stringify(v))
  } catch {}
})

export const activeFile = writable(/** @type {string|null} */ (localStorage.getItem(KEYS.LAST_FILE) || null))

activeFile.subscribe(v => {
  try {
    if (v) localStorage.setItem(KEYS.LAST_FILE, v)
    else localStorage.removeItem(KEYS.LAST_FILE)
  } catch {}
})

// Ensure activeFile is in openedFiles
activeFile.subscribe((filePath) => {
  if (!filePath) return
  openedFiles.update(files => {
    if (files.some(f => f.path === filePath)) return files
    return [...files, { path: filePath, name: filePath.split(/[\\/]/).pop() || '' }]
  })
})
export const settingsVisible = writable(false)

export const editorGroups = writable(/** @type {Array<{id: string, files: Array<{path: string, name: string, pinned?: boolean}>, activeFile: string|null}>} */ ([
  { id: 'group-1', files: [], activeFile: null }
]))

export const activeGroupId = writable('group-1')

export const fileTree = writable(/** @type {Array<{name: string, path: string, isDir: boolean, children: any[], loaded?: boolean}>} */ ([]))

currentProject.subscribe(async ($currentProject) => {
  if (!$currentProject) {
    fileTree.set([])
    return
  }
  // Wait for Wails backend to be ready
  if (!window.backend) {
    const waitForBackend = new Promise(resolve => {
      const check = setInterval(() => {
        if (window.backend) { clearInterval(check); resolve() }
      }, 50)
    })
    await waitForBackend
  }
  if (window.backend.SetProjectPath) {
    window.backend.SetProjectPath($currentProject)
  }
  buildFileTree($currentProject).then(tree => fileTree.set(tree))
})

/**
 * @param {string} path
 * @returns {Promise<Array<{name: string, path: string, isDir: boolean, children: any[]}>>}
 */
async function buildFileTree(path) {
  const fs = window.backend
  if (!fs) return []
  
  try {
    const files = await fs.ListDir(path)
    const result = []
    for (const file of files) {
      result.push({
        name: file.name,
        path: file.path,
        isDir: file.isDir,
        children: [],
        loaded: !file.isDir
      })
    }
    return result
  } catch {
    return []
  }
}

export async function loadDirectoryContents(node) {
  if (node.loaded || !node.isDir) return
  const children = await buildFileTree(node.path)
  node.children = children
  node.loaded = true
  fileTree.update(tree => [...tree])
}

/**
 * @param {string} filePath
 */
export function openFile(filePath) {
  openedFiles.update(files => {
    if (files.some(f => f.path === filePath)) return files
    return [...files, { path: filePath, name: filePath.split(/[\\/]/).pop() || '' }]
  })
  activeFile.set(filePath)
}

/**
 * @param {string} filePath
 */
export function closeFile(filePath) {
  openedFiles.update(files => {
    const file = files.find(f => f.path === filePath)
    if (file?.pinned) return files
    const remaining = files.filter(f => f.path !== filePath)
    // If closing the active file, switch to the next available file
    const $activeFile = get(activeFile)
    if ($activeFile === filePath) {
      const nextFile = remaining[remaining.length - 1] || null
      activeFile.set(nextFile?.path || null)
    }
    return remaining
  })
}

/**
 * @param {string} filePath
 */
export function setActiveFile(filePath) {
  activeFile.set(filePath)
}

/**
 * @param {string} filePath
 */
export function togglePinTab(filePath) {
  openedFiles.update(files => {
    return files.map(f => f.path === filePath ? { ...f, pinned: !f.pinned } : f)
  })
}

/**
 * @param {number} fromIndex
 * @param {number} toIndex
 */
export function reorderTab(fromIndex, toIndex) {
  openedFiles.update(files => {
    if (fromIndex === toIndex || fromIndex < 0 || toIndex < 0) return files
    const item = files.splice(fromIndex, 1)[0]
    files.splice(toIndex, 0, item)
    return files
  })
}

export function splitEditor() {
  editorGroups.update(groups => {
    if (groups.length >= 2) return groups
    const currentGroup = groups.find(g => g.id === 'group-1')
    const newGroup = {
      id: 'group-2',
      files: currentGroup?.activeFile
        ? [{ path: currentGroup.activeFile, name: currentGroup.activeFile.split(/[\\/]/).pop() || '' }]
        : [],
      activeFile: currentGroup?.activeFile || null
    }
    return [...groups, newGroup]
  })
}

export function closeSplit() {
  editorGroups.update(groups => {
    if (groups.length <= 1) return groups
    const remaining = groups.filter(g => g.id !== 'group-2')
    return remaining
  })
  activeGroupId.set('group-1')
}

/**
 * @param {string} groupId
 * @param {string} filePath
 */
export function setGroupActiveFile(groupId, filePath) {
  editorGroups.update(groups => {
    return groups.map(g => g.id === groupId ? { ...g, activeFile: filePath } : g)
  })
  if (groupId === 'group-1') {
    activeFile.set(filePath)
  }
}

// 文件操作函数

/**
 * @param {string} parentPath
 * @param {string} fileName
 */
export async function createNewFile(parentPath, fileName) {
  const fullPath = `${parentPath}/${fileName}`
  await window.backend.CreateFile(fullPath)
  refreshFileTree()
}

/**
 * @param {string} parentPath
 * @param {string} folderName
 */
export async function createNewFolder(parentPath, folderName) {
  const fullPath = `${parentPath}/${folderName}`
  await window.backend.CreateDir(fullPath)
  refreshFileTree()
}

/**
 * @param {string} path
 */
export async function deleteFileOrFolder(path) {
  await window.backend.DeleteFile(path)
  refreshFileTree()
}

/**
 * @param {string} oldPath
 * @param {string} newName
 */
export async function renameFileOrFolder(oldPath, newName) {
  const parentPath = oldPath.substring(0, oldPath.lastIndexOf('/'))
  const newPath = `${parentPath}/${newName}`
  await window.backend.RenameFile(oldPath, newPath)
  refreshFileTree()
}

function refreshFileTree() {
  const project = get(currentProject)
  if (project) {
    buildFileTree(project).then(tree => fileTree.set(tree))
  }
}

