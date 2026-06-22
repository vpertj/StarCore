import { writable, derived } from 'svelte/store'

export const testRunning = writable(false)
export const testSuites = writable([])
export const testError = writable(null)

export const testSummary = derived(testSuites, ($suites) => {
  let total = 0, passed = 0, failed = 0, skipped = 0
  for (const suite of $suites) {
    total += suite.total
    passed += suite.passed
    failed += suite.failed
    skipped += suite.skipped
  }
  return { total, passed, failed, skipped }
})

export async function runTests(testPath = '') {
  testRunning.set(true)
  testError.set(null)
  try {
    if (!window.backend) throw new Error('Backend not available')
    const results = await window.backend.RunTests(testPath)
    testSuites.set(results || [])
  } catch (e) {
    testError.set(e.message || String(e))
    testSuites.set([])
  } finally {
    testRunning.set(false)
  }
}

export function clearTestResults() {
  testSuites.set([])
  testError.set(null)
}