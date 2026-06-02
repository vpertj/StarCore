import { writable, get } from 'svelte/store'
import { currentProject } from './app.js'
import { addLog } from './output.js'

export const gitStatus = writable(/** @type {Array<{path:string,status:string,staged:boolean,added:boolean,deleted:boolean,renamed:boolean}>} */ ([]))
export const gitBranch = writable('')
export const gitLog = writable(/** @type {Array<{hash:string,message:string,author:string,date:string}>} */ ([]))
export const gitLoading = writable(false)
export const commitMessage = writable('')

export async function refreshGitStatus() {
  const proj = get(currentProject)
  if (!proj) return
  gitLoading.set(true)
  try {
    const data = await window.backend.GitStatusAndBranch(proj)
    gitBranch.set(data.branch || '')
    gitStatus.set(data.status || [])
  } catch (/** @type {any} */ e) {
    gitBranch.set('')
    gitStatus.set([])
  } finally {
    gitLoading.set(false)
  }
}

export async function refreshGitLog() {
  const proj = get(currentProject)
  if (!proj) return
  try {
    const log = await window.backend.GitLog(proj, 20)
    gitLog.set(log || [])
  } catch {
    gitLog.set([])
  }
}

export async function stageFile(filePath) {
  const proj = get(currentProject)
  if (!proj) return
  try {
    await window.backend.GitStage(proj, filePath)
    addLog('Git', 'info', 'Staged: ' + filePath)
    await refreshGitStatus()
  } catch (/** @type {any} */ e) {
    addLog('Git', 'error', 'Stage failed: ' + (e.message || String(e)))
  }
}

export async function unstageFile(filePath) {
  const proj = get(currentProject)
  if (!proj) return
  try {
    await window.backend.GitUnstage(proj, filePath)
    addLog('Git', 'info', 'Unstaged: ' + filePath)
    await refreshGitStatus()
  } catch (/** @type {any} */ e) {
    addLog('Git', 'error', 'Unstage failed: ' + (e.message || String(e)))
  }
}

export async function commitChanges(msg) {
  const proj = get(currentProject)
  if (!proj || !msg.trim()) return
  try {
    addLog('Git', 'info', 'Committing: ' + msg)
    await window.backend.GitCommit(proj, msg)
    commitMessage.set('')
    await refreshGitStatus()
    await refreshGitLog()
    addLog('Git', 'info', 'Commit successful')
  } catch (/** @type {any} */ e) {
    addLog('Git', 'error', 'Commit failed: ' + (e.message || String(e)))
  }
}

// Auto-refresh when project changes
currentProject.subscribe(async (proj) => {
  if (proj) {
    await refreshGitStatus()
  }
})
