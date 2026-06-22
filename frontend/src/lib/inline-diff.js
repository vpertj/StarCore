import { Decoration, EditorView, WidgetType } from '@codemirror/view'
import { parseAIDiff, buildApplyEdits } from './diff-apply.js'

class DiffLineWidget extends WidgetType {
  constructor(text, type) {
    super()
    this.text = text
    this.type = type
  }
  toDOM() {
    const span = document.createElement('div')
    span.className = `cm-diff-line cm-diff-${this.type}`
    span.textContent = this.text
    return span
  }
  ignoreEvent() { return true }
}

export function inlineDiffExtension(getDiffText) {
  return EditorView.decorations.compute([], (state) => {
    const diffText = typeof getDiffText === 'function' ? getDiffText() : ''
    if (!diffText) return Decoration.none

    const hunks = parseAIDiff(diffText)
    if (!hunks || hunks.length === 0) return Decoration.none

    const decorations = []
    const doc = state.doc

    for (const hunk of hunks) {
      const startLine = Math.min(hunk.oldStart || 1, doc.lines) - 1
      const endLine = Math.min(startLine + (hunk.oldCount || 1), doc.lines)

      for (let i = startLine; i < endLine && i < doc.lines; i++) {
        const line = doc.line(i + 1)
        const lineText = line.text
        const oldIdx = i - startLine
        const oldLine = hunk.oldLines?.[oldIdx]
        const newLine = hunk.newLines?.[oldIdx]

        if (oldLine !== undefined && newLine !== undefined && oldLine !== newLine) {
          decorations.push(
            Decoration.line({ class: 'cm-diff-changed' }).range(line.from)
          )
        } else if (oldLine !== undefined && newLine === undefined) {
          decorations.push(
            Decoration.line({ class: 'cm-diff-deleted' }).range(line.from)
          )
        }
      }

      if (hunk.newLines) {
        for (let j = (hunk.oldLines?.length || 0); j < hunk.newLines.length; j++) {
          const insertLine = Math.min(endLine, doc.lines)
          if (insertLine >= 0 && insertLine < doc.lines) {
            const pos = doc.line(insertLine + 1).from
            decorations.push(
              Decoration.widget({
                widget: new DiffLineWidget(hunk.newLines[j], 'added'),
                side: 1,
              }).range(pos)
            )
          }
        }
      }
    }

    return Decoration.set(decorations.sort((a, b) => a.from - b.from), true)
  })
}

export async function applyInlineDiff(filePath, diffText) {
  const hunks = parseAIDiff(diffText)
  if (!hunks || hunks.length === 0) return false

  const edits = buildApplyEdits(hunks)
  if (!edits || edits.length === 0) return false

  if (!window.backend?.ApplyDiff) return false

  try {
    await window.backend.ApplyDiff({ filePath, hunks })
    return true
  } catch (e) {
    console.error('Apply inline diff failed:', e)
    return false
  }
}