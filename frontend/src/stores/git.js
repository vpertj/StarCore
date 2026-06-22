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

export async function stageAll() {
  const proj = get(currentProject)
  if (!proj) return
  try {
    for (const entry of get(gitStatus)) {
      if (!entry.staged) await window.backend.GitStage(proj, entry.path)
    }
    await refreshGitStatus()
  } catch (/** @type {any} */ e) {
    addLog('Git', 'error', 'Stage all failed: ' + (e.message || String(e)))
  }
}

export async function unstageAll() {
  const proj = get(currentProject)
  if (!proj) return
  try {
    for (const entry of get(gitStatus)) {
      if (entry.staged) await window.backend.GitUnstage(proj, entry.path)
    }
    await refreshGitStatus()
  } catch (/** @type {any} */ e) {
    addLog('Git', 'error', 'Unstage all failed: ' + (e.message || String(e)))
  }
}

export async function checkoutBranch(branchName) {
  const proj = get(currentProject)
  if (!proj) return
  try {
    await window.backend.GitCheckout(proj, branchName)
    addLog('Git', 'info', 'Checked out: ' + branchName)
    await refreshGitStatus()
    await refreshGitLog()
  } catch (/** @type {any} */ e) {
    addLog('Git', 'error', 'Checkout failed: ' + (e.message || String(e)))
  }
}

export async function createBranch(branchName) {
  const proj = get(currentProject)
  if (!proj || !branchName.trim()) return
  try {
    await window.backend.GitCreateBranch(proj, branchName.trim())
    addLog('Git', 'info', 'Created branch: ' + branchName)
    await refreshGitStatus()
  } catch (/** @type {any} */ e) {
    addLog('Git', 'error', 'Create branch failed: ' + (e.message || String(e)))
  }
}

export async function pullChanges() {
  const proj = get(currentProject)
  if (!proj) return
  try {
    addLog('Git', 'info', 'Pulling changes...')
    await window.backend.GitPull(proj)
    await refreshGitStatus()
    await refreshGitLog()
    addLog('Git', 'info', 'Pull successful')
  } catch (/** @type {any} */ e) {
    addLog('Git', 'error', 'Pull failed: ' + (e.message || String(e)))
  }
}

export async function pushChanges() {
  const proj = get(currentProject)
  if (!proj) return
  try {
    addLog('Git', 'info', 'Pushing changes...')
    await window.backend.GitPush(proj)
    addLog('Git', 'info', 'Push successful')
  } catch (/** @type {any} */ e) {
    addLog('Git', 'error', 'Push failed: ' + (e.message || String(e)))
  }
}

// Auto-refresh when project changes
currentProject.subscribe(async (proj) => {
  if (proj) {
    await refreshGitStatus()
  }
})
