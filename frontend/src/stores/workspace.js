import { writable } from 'svelte/store'

export const workspaceRoots = writable([])
export const activeRoot = writable('')

export async function loadWorkspaceRoots() {
  if (!window.backend) return
  try {
    const roots = await window.backend.GetWorkspaceRoots()
    workspaceRoots.set(roots || [])
    const active = (roots || []).find(r => r.active)
    activeRoot.set(active ? active.path : '')
  } catch {}
}

export async function addWorkspaceRoot(path) {
  if (!window.backend) return
  await window.backend.AddWorkspaceRoot(path)
  await loadWorkspaceRoots()
}

export async function removeWorkspaceRoot(path) {
  if (!window.backend) return
  await window.backend.RemoveWorkspaceRoot(path)
  await loadWorkspaceRoots()
}

export async function setActiveWorkspaceRoot(path) {
  if (!window.backend) return
  await window.backend.SetActiveWorkspaceRoot(path)
  activeRoot.set(path)
  await loadWorkspaceRoots()
}