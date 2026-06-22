import { describe, it, expect } from 'vitest'
import { parseAIDiff, buildApplyEdit, buildApplyEdits } from '../src/lib/diff-apply.js'

describe('parseAIDiff', () => {
  it('parses unified diff', () => {
    const diff = `--- a/file.go
+++ b/file.go
@@ -1,3 +1,3 @@
 line1
-old line
+new line
 line3`
    const hunks = parseAIDiff(diff)
    expect(hunks.length).toBe(1)
    expect(hunks[0].oldLines).toContain('old line')
    expect(hunks[0].newLines).toContain('new line')
  })

  it('handles empty diff', () => {
    expect(parseAIDiff('')).toEqual([])
  })
})

describe('buildApplyEdit', () => {
  it('builds edit from hunk', () => {
    const hunk = {
      filePath: 'test.go',
      oldLines: ['a', 'b'],
      newLines: ['a', 'c'],
      header: '@@ -1,2 +1,2 @@',
    }
    const edit = buildApplyEdit(hunk)
    expect(edit.filePath).toBe('test.go')
    expect(edit.oldText).toBe('a\nb')
    expect(edit.newText).toBe('a\nc')
  })
})

describe('buildApplyEdits', () => {
  it('builds multiple edits', () => {
    const hunks = [
      { filePath: 'a.go', oldLines: ['x'], newLines: ['y'], header: '' },
      { filePath: 'b.go', oldLines: ['1'], newLines: ['2'], header: '' },
    ]
    const edits = buildApplyEdits(hunks)
    expect(edits.length).toBe(2)
  })
})