import { ViewPlugin, Decoration, WidgetType } from '@codemirror/view'
import { RangeSetBuilder } from '@codemirror/state'
import { LSPCodeLens, LSPInlayHints, LSPFoldingRanges } from '../../wailsjs/go/main/App.js'

class InlayHintWidget extends WidgetType {
  constructor(text, kind) {
    super()
    this.text = text
    this.kind = kind
  }
  toDOM() {
    const span = document.createElement('span')
    span.className = 'cm-inlay-hint' + (this.kind === 2 ? ' cm-inlay-hint-parameter' : '')
    span.textContent = this.text
    return span
  }
  ignoreEvent() { return true }
}

class CodeLensWidget extends WidgetType {
  constructor(command) {
    super()
    this.command = command
  }
  toDOM() {
    const span = document.createElement('span')
    span.className = 'cm-code-lens'
    span.textContent = this.command?.title || ''
    span.style.cursor = 'pointer'
    span.style.opacity = '0.6'
    span.style.fontSize = '0.85em'
    return span
  }
  ignoreEvent() { return false }
}

export function inlayHintsExtension(filePath) {
  return ViewPlugin.define class InlayHintsPlugin {
    decorations = Decoration.none

    update(update) {
      if (update.docChanged || update.viewportChanged) {
        this.loadHints(update.view)
      }
    }

    async loadHints(view) {
      if (!filePath) return
      try {
        const viewport = view.viewport
        const startLine = view.state.doc.lineAt(viewport.from).number - 1
        const endLine = view.state.doc.lineAt(viewport.to).number - 1
        const hints = await LSPInlayHints(filePath, startLine, 0, endLine, 0)
        if (!hints || !hints.length) {
          this.decorations = Decoration.none
          return
        }
        const builder = new RangeSetBuilder()
        for (const hint of hints) {
          const pos = view.state.doc.line(hint.Position.Line + 1).from + hint.Position.Character
          let label = ''
          if (typeof hint.Label === 'string') label = hint.Label
          else if (Array.isArray(hint.Label)) label = hint.Label.map(p => p.value).join('')
          if (label) {
            builder.add(pos, pos, Decoration.widget({
              widget: new InlayHintWidget(label, hint.Kind),
              side: 1,
            }))
          }
        }
        this.decorations = builder.finish()
      } catch (e) {
        this.decorations = Decoration.none
      }
    }
  }, { decorations: v => v.decorations }
}

export function codeLensExtension(filePath) {
  return ViewPlugin.define class CodeLensPlugin {
    decorations = Decoration.none

    update(update) {
      if (update.docChanged) {
        this.loadLenses(update.view)
      }
    }

    async loadLenses(view) {
      if (!filePath) return
      try {
        const lenses = await LSPCodeLens(filePath)
        if (!lenses || !lenses.length) {
          this.decorations = Decoration.none
          return
        }
        const builder = new RangeSetBuilder()
        for (const lens of lenses) {
          const line = lens.Range.Start.Line + 1
          if (line <= view.state.doc.lines) {
            const pos = view.state.doc.line(line).from
            if (lens.Command) {
              builder.add(pos, pos, Decoration.widget({
                widget: new CodeLensWidget(lens.Command),
                side: -1,
                block: true,
              }))
            }
          }
        }
        this.decorations = builder.finish()
      } catch (e) {
        this.decorations = Decoration.none
      }
    }
  }, { decorations: v => v.decorations }
}

export async function loadFoldingRanges(filePath) {
  if (!filePath) return []
  try {
    const ranges = await LSPFoldingRanges(filePath)
    return (ranges || []).map(r => ({
      from: r.StartLine,
      to: r.EndLine,
      kind: r.Kind || 'region',
    }))
  } catch (e) {
    return []
  }
}