import { writable, derived, get } from 'svelte/store'
import { EventsOn } from '../../wailsjs/runtime/runtime.js'
import { currentProject } from './app.js'
import { activeFile } from './app.js'

export const debugSession = writable(/** @type {string|null} */ (null))
export const debugState = writable(/** @type {{status:string, reason:string, file:string, line:number, stack:any[], variables:any[], goroutines:any[]}|null} */ (null))
export const debugBreakpoints = writable(/** @type {{id:number, file:string, line:number, enabled:boolean, condition?:string}[]} */ ([]))
export const debugRunning = writable(false)
export const watchExpressions = writable(/** @type {{expr:string, result?:any}[]} */ ([]))
export const consoleHistory = writable(/** @type {{input:string, output:string, error?:string}[]} */ ([]))

EventsOn('debug:state-changed', (data) => {
  const state = data?.state || data
  debugState.set(state)
  debugRunning.set(state?.status === 'running')
})

EventsOn('debug:breakpoint-hit', (data) => {
  const state = data?.state || data
  debugState.set(state)
  debugRunning.set(false)
  if (state?.file) activeFile.set(state.file)
})

EventsOn('debug:session-ended', () => {
  debugSession.set(null)
  debugState.set(null)
  debugBreakpoints.set([])
  debugRunning.set(false)
  watchExpressions.set([])
  consoleHistory.set([])
})

export async function startDebug(programPath) {
  try {
    const result = await window.backend.DebugStart(programPath, [])
    const session = result?.id || result
    debugSession.set(session)
    debugRunning.set(true)
    return session
  } catch (e) {
    console.error('Failed to start debug:', e)
    return null
  }
}

export async function stopDebug() {
  const session = get(debugSession)
  if (!session) return
  try { await window.backend.DebugStop(session) } catch {}
  debugSession.set(null)
  debugState.set(null)
  debugBreakpoints.set([])
  debugRunning.set(false)
  watchExpressions.set([])
}

export async function addBreakpoint(file, line, condition = '') {
  const session = get(debugSession)
  if (!session) return
  try {
    const bp = await window.backend.DebugAddBreakpoint(session, file, line, condition)
    if (bp) {
      debugBreakpoints.update(bps => [...bps, bp])
    }
  } catch (e) {
    console.error('Failed to add breakpoint:', e)
  }
}

export async function removeBreakpoint(bpId) {
  const session = get(debugSession)
  if (!session) return
  try {
    await window.backend.DebugRemoveBreakpoint(session, bpId)
    debugBreakpoints.update(bps => bps.filter(b => b.id !== bpId))
  } catch (e) {
    console.error('Failed to remove breakpoint:', e)
  }
}

export async function toggleBreakpoint(file, line) {
  const bps = get(debugBreakpoints)
  const existing = bps.find(b => b.file === file && b.line === line)
  if (existing) {
    await removeBreakpoint(existing.id)
  } else {
    await addBreakpoint(file, line)
  }
}

export async function continueDebug() {
  const session = get(debugSession)
  if (!session) return
  debugRunning.set(true)
  try { await window.backend.DebugContinue(session) } catch {}
}

export async function stepOver() {
  const session = get(debugSession)
  if (!session) return
  debugRunning.set(true)
  try { await window.backend.DebugStepOver(session) } catch {}
}

export async function stepIn() {
  const session = get(debugSession)
  if (!session) return
  debugRunning.set(true)
  try { await window.backend.DebugStepIn(session) } catch {}
}

export async function stepOut() {
  const session = get(debugSession)
  if (!session) return
  debugRunning.set(true)
  try { await window.backend.DebugStepOut(session) } catch {}
}

export async function addWatch(expr) {
  const session = get(debugSession)
  if (!session || !expr.trim()) return
  try {
    const result = await window.backend.DebugGetVariable(session, 0, expr.trim())
    watchExpressions.update(watches => [...watches, { expr: expr.trim(), result }])
  } catch (e) {
    watchExpressions.update(watches => [...watches, { expr: expr.trim(), result: { error: String(e) } }])
  }
}

export async function removeWatch(index) {
  watchExpressions.update(watches => watches.filter((_, i) => i !== index))
}

export async function refreshWatches() {
  const session = get(debugSession)
  if (!session) return
  const watches = get(watchExpressions)
  const updated = []
  for (const w of watches) {
    try {
      const result = await window.backend.DebugGetVariable(session, 0, w.expr)
      updated.push({ expr: w.expr, result })
    } catch (e) {
      updated.push({ expr: w.expr, result: { error: String(e) } })
    }
  }
  watchExpressions.set(updated)
}

export async function executeConsole(expr) {
  const session = get(debugSession)
  if (!session || !expr.trim()) return
  try {
    const result = await window.backend.DebugConsoleExecute(session, expr.trim())
    consoleHistory.update(h => [...h, { input: expr.trim(), output: result?.output || '', error: result?.error || '' }])
  } catch (e) {
    consoleHistory.update(h => [...h, { input: expr.trim(), output: '', error: String(e) }])
  }
}
