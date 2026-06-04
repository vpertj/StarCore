<script>
 import { onMount, onDestroy } from 'svelte'
  import { EditorState, Compartment } from '@codemirror/state'
 import { EditorView, keymap, lineNumbers, highlightActiveLine, highlightActiveLineGutter, WidgetType, Decoration } from '@codemirror/view'
 import { defaultKeymap, history, historyKeymap, indentWithTab } from '@codemirror/commands'
 import { syntaxHighlighting, defaultHighlightStyle, HighlightStyle } from '@codemirror/language'
 import { tags } from '@lezer/highlight'
 import { go } from '@codemirror/lang-go'
 import { javascript } from '@codemirror/lang-javascript'
 import { json } from '@codemirror/lang-json'
 import { html } from '@codemirror/lang-html'
 import { css } from '@codemirror/lang-css'
 import { markdown } from '@codemirror/lang-markdown'
 import { python } from '@codemirror/lang-python'
 import { rust } from '@codemirror/lang-rust'
 import { java } from '@codemirror/lang-java'
 import { cpp } from '@codemirror/lang-cpp'
 import { php } from '@codemirror/lang-php'
 import { sql } from '@codemirror/lang-sql'
 import { xml } from '@codemirror/lang-xml'
 import { yaml } from '@codemirror/lang-yaml'
 import { StateField, StateEffect } from '@codemirror/state'
 import { linter, lintGutter } from '@codemirror/lint'
 import { syntaxTree } from '@codemirror/language'
 import { activeFile } from '../stores/app.js'
 import { editorSettings } from '../stores/editorSettings.js'
 import { t } from '../stores/i18n.js'
 import { EventsOn } from '../../wailsjs/runtime/runtime.js'
 import { activeProviderId } from '../stores/provider.js'
 import { debouncedRequestCompletion, requestCompletion, dismissCompletion, cancelPendingCompletion, completionVisible, completionLoading, completionText, completionConfig } from '../stores/completion.js'
 import { activeFileContent, selectedCode } from '../stores/ai.js'
 import { diagnostics as diagnosticsStore, activeFileDiagnostics } from '../stores/diagnostics.js'
 import { currentTheme, getThemeById } from '../stores/theme.js'

  /** @type {HTMLDivElement} */ let editorContainer
  /** @type {EditorView|null} */ let view = null
  const highlightCompartment = new Compartment
  let content = $state('')
  let isDirty = $state(false)
 let lspChangeTimer = null
  let lastFilePath = ''
  /** @type {(() => void)|null} */ let completionUnsubscribe = null
  /** @type {(() => void)|null} */ let activeFileUnsubscribe = null
  /** @type {(() => void)|null} */ let settingsUnsubscribe = null
  /** @type {(() => void)|null} */ let themeUnsub = null
  /** @type {(() => void)|null} */ let fileChangeUnsubscribe = null
 let lspDiagUnsubscribe = null
  let mounted = false
  let lastSettingsJSON = ''

 function getLanguageFromPath(path) {
   if (!path) return 'plaintext'
   const ext = path.split('.').pop()?.toLowerCase() || ''
   switch (ext) {
     case 'go': return 'go'
     case 'js': case 'mjs': case 'cjs': case 'ts': case 'tsx': return 'javascript'
     case 'json': return 'json'
     case 'html': case 'htm': return 'html'
     case 'css': case 'scss': case 'sass': case 'less': return 'css'
     case 'md': case 'markdown': return 'markdown'
     case 'py': case 'python': return 'python'
     case 'rs': case 'rust': return 'rust'
     case 'java': return 'java'
     case 'cpp': case 'c': case 'h': case 'hpp': return 'cpp'
     case 'php': return 'php'
     case 'sql': return 'sql'
     case 'xml': case 'svg': return 'xml'
     case 'yaml': case 'yml': return 'yaml'
     default: return 'plaintext'
   }
 }

  /** @type {import('@codemirror/lint').Diagnostic[]} */ let currentDiagnostics = []

  function updateDiagnostics(view) {
    const tree = syntaxTree(view.state)
    /** @type {import('@codemirror/lint').Diagnostic[]} */ const d = []
    tree.cursor().iterate(node => {
      if (node.name === '⚠' || node.name === '✖' || node.type.isError) {
        const line = view.state.doc.lineAt(node.from)
        d.push({
          from: node.from,
          to: node.to,
          severity: 'error',
          message: 'Syntax error',
          filePath: lastFilePath,
        })
      }
    })
    const merged = []
    let last = null
    for (const diag of d) {
      if (last && Math.abs(last.from - diag.from) < 2) continue
      merged.push(diag)
      last = diag
    }
    currentDiagnostics = merged
    activeFileDiagnostics.set(merged)
  }

  const customLinter = linter(view => {
    updateDiagnostics(view)
    return currentDiagnostics
  })

  class GhostTextWidget extends WidgetType {
   constructor(text) {
     super()
     this.text = text
   }
   toDOM() {
     const span = document.createElement('span')
     span.textContent = this.text
     span.style.color = 'rgba(128,128,128,0.65)'
     span.style.pointerEvents = 'none'
     span.className = 'cm-ghost-completion'
     return span
   }
   eq(other) {
     return other instanceof GhostTextWidget && other.text === this.text
   }
 }

 const setCompletionEffect = StateEffect.define()
 const clearCompletionEffect = StateEffect.define()

 const completionField = StateField.define({
   create() { return Decoration.none },
   update(decorations, tr) {
     decorations = decorations.map(tr.changes)
     for (const effect of tr.effects) {
       if (effect.is(setCompletionEffect)) {
         const text = effect.value
         const pos = tr.state.selection.main.head
         const widget = Decoration.widget({
           widget: new GhostTextWidget(text),
           side: 1
         })
         return Decoration.set([widget.range(pos)])
       }
       if (effect.is(clearCompletionEffect)) {
         return Decoration.none
       }
     }
     return decorations
   },
   provide: f => EditorView.decorations.from(f)
 })

  const TAG_LOOKUP = {
    keyword: tags.keyword, string: tags.string, number: tags.number,
    comment: tags.comment, variableName: tags.variableName, typeName: tags.typeName,
    functionName: tags.functionName, operator: tags.operator, builtin: tags.builtin,
    bracket: tags.bracket, name: tags.name, propertyName: tags.propertyName,
    labelName: tags.labelName, separator: tags.separator, className: tags.className,
    'definition(name)': tags.definition(tags.name),
    'function(variableName)': tags.function(tags.variableName),
    'special(string)': tags.special(tags.string),
    'special(variableName)': tags.special(tags.variableName),
    macroName: tags.macroName, character: tags.character, deleted: tags.deleted,
    inserted: tags.inserted, changed: tags.changed, invalid: tags.invalid,
    meta: tags.meta, heading: tags.heading, link: tags.link, strong: tags.strong,
    emphasis: tags.emphasis, strikethrough: tags.strikethrough, url: tags.url,
    escape: tags.escape, regexp: tags.regexp, annotation: tags.annotation,
    modifier: tags.modifier, namespace: tags.namespace, atom: tags.atom, bool: tags.bool,
    'constant(name)': tags.constant(tags.name),
    'standard(name)': tags.standard(tags.name),
    operatorKeyword: tags.operatorKeyword, self: tags.self,
    processingInstruction: tags.processingInstruction, color: tags.color,
  }

  function resolveTag(name) {
    return TAG_LOOKUP[name] || null
  }

  function getHighlightExtension(themeId) {
    const theme = getThemeById(themeId)
    if (!theme || !theme.syntax) return syntaxHighlighting(defaultHighlightStyle)
    const specs = theme.syntax.map(entry => {
      const tagNames = Array.isArray(entry.tag) ? entry.tag : [entry.tag]
      const resolved = tagNames.map(n => resolveTag(n)).filter(Boolean)
      if (resolved.length === 0) return null
      const spec = { tag: resolved.length === 1 ? resolved[0] : resolved }
      if (entry.color) spec.color = entry.color
      if (entry.fontStyle) spec.fontStyle = entry.fontStyle
      if (entry.fontWeight) spec.fontWeight = entry.fontWeight
      return spec
    }).filter(Boolean)
    return syntaxHighlighting(HighlightStyle.define(specs))
  }

  function getLanguageExtension(path) {
   if (!path) return null
   const ext = path.split('.').pop()?.toLowerCase() || ''
   switch (ext) {
     case 'go': return go()
     case 'js': case 'mjs': case 'cjs': case 'ts': case 'tsx': return javascript()
     case 'json': return json()
     case 'html': case 'htm': return html()
     case 'css': case 'scss': case 'sass': case 'less': return css()
     case 'md': case 'markdown': return markdown()
     case 'py': case 'python': return python()
     case 'rs': case 'rust': return rust()
     case 'java': return java()
     case 'cpp': case 'c': case 'h': case 'hpp': return cpp()
     case 'php': return php()
     case 'sql': return sql()
     case 'xml': case 'svg': return xml()
     case 'yaml': case 'yml': return yaml()
     default: return null
   }
 }

 async function loadFileContent(filePath) {
   if (!filePath) {
     content = ''
     isDirty = false
     activeFileContent.set('')
     selectedCode.set('')
     if (view) {
       view.destroy()
       view = null
     }
     return
   }
   try {
     content = await window.backend.ReadFile(filePath)
     // Notify LSP that this file is open
     if (window.backend?.LSPDidOpen) {
       window.backend.LSPDidOpen(filePath, content).catch(() => {})
     }
     isDirty = false
     activeFileContent.set(content)
     initEditor(filePath)
   } catch (err) {
     console.error('Failed to read file:', err)
     content = ''
     isDirty = false
     activeFileContent.set('')
   }
 }

 async function saveFile() {
   const currentFilePath = getStoreValue(activeFile)
   if (!currentFilePath || !isDirty) return
   try {
     await window.backend.WriteFile(currentFilePath, content)
     // Notify LSP of the change
     if (window.backend?.LSPDidChange) {
       window.backend.LSPDidChange(currentFilePath, content).catch(() => {})
     }
     isDirty = false
   } catch (err) {
     console.error('Failed to save file:', err)
   }
 }

  function initEditor(filePath) {
    try {
    if (!editorContainer) return
    if (completionUnsubscribe) {
     completionUnsubscribe()
     completionUnsubscribe = null
   }
   if (view) {
     view.destroy()
     view = null
   }

   const languageExt = getLanguageExtension(filePath)

   const extensions = [
     lineNumbers(),
     highlightActiveLine(),
     highlightActiveLineGutter(),
     lintGutter(),
     customLinter,
     history(),
     keymap.of([
       ...defaultKeymap,
       ...historyKeymap,
       indentWithTab,
       {
         key: 'Alt-\\',
         run: () => {
           if (!view || !filePath) return false
           const pos = view.state.selection.main.head
           const contentStr = view.state.doc.toString()
           const lang = getLanguageFromPath(filePath)
           const providerId = getStoreValue(activeProviderId) || 'openai'
           requestCompletion(filePath, contentStr, pos, lang, providerId)
           return true
         }
       },
       {
         key: 'Tab',
         run: () => {
           const text = getStoreValue(completionText)
           const visible = getStoreValue(completionVisible)
           if (visible && text && view) {
             const pos = view.state.selection.main.head
             view.dispatch({
               changes: { from: pos, insert: text },
               effects: [clearCompletionEffect.of(undefined)]
             })
             dismissCompletion()
             return true
           }
           return false
         }
       },
       {
         key: 'Escape',
         run: () => {
           const visible = getStoreValue(completionVisible)
           if (visible) {
             dismissCompletion()
             return true
           }
           return false
         }
       }
     ]),
       highlightCompartment.of(getHighlightExtension(getStoreValue(currentTheme))),
     EditorView.updateListener.of((update) => {
       if (update.docChanged) {
         content = update.state.doc.toString()
         isDirty = true
         activeFileContent.set(content)
         triggerCompletion(filePath, false)
       }
       if (update.selectionSet) {
         const sel = update.state.selection.main
         if (sel.from !== sel.to) {
           selectedCode.set(update.state.doc.sliceString(sel.from, sel.to))
         } else {
           selectedCode.set('')
         }
       }
     }),
     completionField,
     EditorView.theme({
       '&': { height: '100%', backgroundColor: 'var(--bg-primary)' },
       '.cm-editor': {
         height: '100%',
         fontSize: getStoreValue(editorSettings).fontSize + 'px',
         fontFamily: getStoreValue(editorSettings).fontFamily,
         fontWeight: '500',
         letterSpacing: '0.01em',
       },
       '.cm-scroller': { overflow: 'auto', fontSmooth: 'always' },
       '.cm-content': { padding: '12px 0', lineHeight: String(getStoreValue(editorSettings).lineHeight) },
       '.cm-line': { padding: '0 12px' },
       '.cm-gutters': {
         backgroundColor: 'var(--bg-primary)',
         borderRight: '1px solid var(--border)',
         color: 'var(--text-muted)',
        minWidth: '28px',
        maxWidth: '40px',
       },
       '.cm-lineNumbers .cm-gutterElement': {
         padding: '0 5px 0 4px',
         color: '#5a5a5a',
         fontSize: '12px',
         fontFamily: "'JetBrains Mono', 'Cascadia Code', monospace",
       },
       '.cm-activeLine': { backgroundColor: 'rgba(255, 255, 255, 0.03)' },
       '.cm-activeLineGutter': { backgroundColor: 'rgba(255, 255, 255, 0.04)', color: '#999999' },
       '.cm-selectionBackground': { backgroundColor: 'rgba(86, 156, 214, 0.25)' },
       '&.cm-focused .cm-selectionBackground': { backgroundColor: 'rgba(86, 156, 214, 0.25)' },
       '&.cm-focused .cm-cursor': { borderLeftColor: getStoreValue(editorSettings).cursorColor, borderLeftWidth: getStoreValue(editorSettings).cursorWidth + 'px' },
       '.cm-cursor': { borderLeftColor: getStoreValue(editorSettings).cursorColor, borderLeftWidth: getStoreValue(editorSettings).cursorWidth + 'px' },
       '.cm-cursorDrop': { borderLeftColor: getStoreValue(editorSettings).cursorColor },
       '.cm-cursorLayer': { animationPlayState: getStoreValue(editorSettings).cursorBlinkStyle === 'solid' ? 'paused' : 'running' },
       '.cm-foldPlaceholder': {
         backgroundColor: 'var(--bg-secondary)',
         borderColor: 'var(--border)',
         color: 'var(--text-primary)'
       },
       '.cm-tooltip': {
         backgroundColor: 'var(--bg-secondary)',
         border: '1px solid var(--border)',
         color: 'var(--text-primary)',
         fontSize: '13px',
         borderRadius: '6px',
         boxShadow: '0 4px 12px rgba(0,0,0,0.5)',
       },
     }),
   ]

   if (languageExt) {
     extensions.push(languageExt)
   }

   const state = EditorState.create({
     doc: content,
     extensions,
   })

    view = new EditorView({
      state,
      parent: editorContainer,
    })

    // Focus the editor DOM directly for reliable cursor display
    requestAnimationFrame(() => {
      if (!view) return
      view.focus()
      // Also try focusing the contentEditable element directly
      const contentEl = view.dom.querySelector('.cm-content')
      if (contentEl) contentEl.focus()
    })

    // Apply initial cursor settings
    requestAnimationFrame(() => { if (view) applyCursorSettings(view, getStoreValue(editorSettings)) })

    completionUnsubscribe = completionText.subscribe(text => {
     if (view && text) {
       view.dispatch({ effects: [setCompletionEffect.of(text)] })
     } else if (view) {
       view.dispatch({ effects: [clearCompletionEffect.of(undefined)] })
     }
   })
   } catch (err) {
     console.error('Failed to init editor:', err)
   }
   return () => { if (typeof settingsUnsub === 'function') settingsUnsub() }
 }

 function triggerCompletion(filePath, instant) {
   if (!view || !filePath) return
   const config = getStoreValue(completionConfig)
   if (!config.enabled) return
   if (config.triggerMode === 'manual') return

   const pos = view.state.selection.main.head
   const contentStr = view.state.doc.toString()
   const lang = getLanguageFromPath(filePath)
   const providerId = getStoreValue(activeProviderId) || 'openai'

   if (instant) { cancelPendingCompletion(); requestCompletion(filePath, contentStr, pos, lang, providerId) } else { debouncedRequestCompletion(filePath, contentStr, pos, lang, providerId, config.debounceMs) }
 }

 function getStoreValue(store) {
   let value = undefined
   store.subscribe(v => value = v)()
   return value
 }

 function handleInsertCode(e) {
   const { code } = e.detail || {}
   if (code === undefined || code === null) return
   if (view) {
     const from = view.state.selection.main.from
     const to = view.state.selection.main.to
     view.dispatch({
       changes: { from, to, insert: code },
       selection: { anchor: from + code.length }
     })
     view.focus()
   }
 }

 function handleKeyDown(e) {
   if ((e.ctrlKey || e.metaKey) && e.key === 's') {
     e.preventDefault()
     saveFile()
   }
 }

  function applyCursorSettings(v, s) {
    const layer = v.dom.querySelector('.cm-cursorLayer')
    const cursor = v.dom.querySelector('.cm-cursor')
    if (!layer || !cursor) return
    // Cursor height
    cursor.style.height = (s.cursorHeight || 100) + '%'
    // Blink style: set inline animation on cursor layer (overrides CM defaults)
    const style = s.cursorBlinkStyle || 'blink'
    if (style === 'solid') {
      layer.style.animation = 'none'
    } else if (style === 'smooth') {
      layer.style.animation = 'cm-smooth 1.06s ease-in-out infinite'
    } else if (style === 'phase') {
      layer.style.animation = 'cm-phase 1.06s ease-in-out infinite'
    } else if (style === 'expand') {
      layer.style.animation = 'cm-expand 1.06s ease-in-out infinite'
    } else {
      layer.style.animation = 'cm-blink 1.06s steps(1) infinite'
    }
    // Style
    if (s.cursorStyle === 'line') {
      cursor.style.borderLeft = s.cursorWidth + 'px solid ' + s.cursorColor
      cursor.style.background = 'none'
      cursor.style.borderBottom = 'none'
      cursor.style.width = '0'
    } else if (s.cursorStyle === 'underline') {
      cursor.style.border = 'none'
      cursor.style.background = 'none'
      cursor.style.borderBottom = s.cursorWidth + 'px solid ' + s.cursorColor
      cursor.style.width = '0.6em'
    } else {
      cursor.style.border = 'none'
      cursor.style.background = s.cursorColor
      cursor.style.width = s.cursorWidth + 'px'
    }
  }

  async function fetchFileContent(filePath) {
    try {
      if (window.backend?.ReadFile) return await window.backend.ReadFile(filePath)
    } catch {}
    return null
  }

  onMount(() => {
    mounted = true
    const filePath = getStoreValue(activeFile)
    if (filePath) {
      loadFileContent(filePath)
    } else {
      initEditor('')
    }

    activeFileUnsubscribe = activeFile.subscribe((filePath) => {
      if (!mounted) return
      if (filePath !== lastFilePath) {
        lastFilePath = filePath || ''
        loadFileContent(filePath || '')
      }
    })

    window.addEventListener('keydown', handleKeyDown)
    window.addEventListener('insert-code', handleInsertCode)

    // Listen for LSP diagnostics from language servers
    if (lspDiagUnsubscribe) { lspDiagUnsubscribe(); lspDiagUnsubscribe = null }
    lspDiagUnsubscribe = EventsOn('lsp:diagnostics', (diags) => {
      if (!view || !Array.isArray(diags)) return
      /** @type {import('@codemirror/lint').Diagnostic[]} */ const cmDiags = []
      for (const d of diags) {
        cmDiags.push({
          from: d.from || d.range?.start?.offset || 0,
          to: d.to || d.range?.end?.offset || 0,
          severity: d.severity === 1 ? 'warning' : 'error',
          message: d.message || 'LSP diagnostic',
        })
      }
      if (cmDiags.length > 0) {
        currentDiagnostics = [...currentDiagnostics.filter(d => d.source !== 'lsp'), ...cmDiags.map(d => ({ ...d, source: 'lsp' }))]
        activeFileDiagnostics.set(currentDiagnostics)
      }
    })

    // Listen for external file changes (AI writes, git operations, etc.)
    fileChangeUnsubscribe = EventsOn('file:change', (change) => {
      onFileChanged(change?.path || change?.Path || '')
    })

    // Also listen for DOM CustomEvent from diffPreview / ProjectExplorer
    const onDomFileChanged = (/** @type {CustomEvent} */ e) => {
      onFileChanged(e.detail?.path || '')
    }
    window.addEventListener('file-changed', onDomFileChanged)

    function onFileChanged(changedPath) {
      if (!changedPath || !view || changedPath !== lastFilePath) return
      fetchFileContent(changedPath).then(newContent => {
        if (newContent !== null && view) {
          const currentContent = view.state.doc.toString()
          if (newContent !== currentContent) {
            view.dispatch({
              changes: { from: 0, to: view.state.doc.length, insert: newContent }
            })
            isDirty = false
          }
        }
      }).catch(() => {})
    }
    // End onFileChanged

    // Apply editor settings changes directly to DOM (fast, no re-init needed)
    settingsUnsubscribe = editorSettings.subscribe(settings => {
      if (!view) return
      const contentEl = view.dom.querySelector('.cm-content')
      const gutterEl = view.dom.querySelector('.cm-gutters')
      if (contentEl) {
        contentEl.style.fontSize = settings.fontSize + 'px'
        contentEl.style.fontFamily = settings.fontFamily
        contentEl.style.lineHeight = String(settings.lineHeight)
      }
      if (gutterEl) {
        gutterEl.style.fontSize = (settings.fontSize - 2) + 'px'
        gutterEl.style.fontFamily = settings.fontFamily
      }
      // Cursor settings - target drawSelection's cursor layer
      applyCursorSettings(view, settings)
    })

    // Update highlight when theme changes
    let prevThemeId = getStoreValue(currentTheme)
    const themeUnsub = currentTheme.subscribe(themeId => {
      if (!view || themeId === prevThemeId) return
      prevThemeId = themeId
      view.dispatch({
        effects: highlightCompartment.reconfigure(getHighlightExtension(themeId))
      })
    })
  })

  onDestroy(() => {
    mounted = false
    if (activeFileUnsubscribe) {
      activeFileUnsubscribe()
      activeFileUnsubscribe = null
    }
    if (settingsUnsubscribe) {
      settingsUnsubscribe()
      settingsUnsubscribe = null
    }
    if (lspDiagUnsubscribe) {
      lspDiagUnsubscribe()
      lspDiagUnsubscribe = null
    }
    if (fileChangeUnsubscribe) {
      fileChangeUnsubscribe()
      fileChangeUnsubscribe = null
    }
    if (completionUnsubscribe) {
      completionUnsubscribe()
      completionUnsubscribe = null
    }
    if (themeUnsub) {
      themeUnsub()
      themeUnsub = null
    }
    if (view) {
      view.destroy()
      view = null
    }
    cancelPendingCompletion()
    window.removeEventListener('keydown', handleKeyDown)
    window.removeEventListener('insert-code', handleInsertCode)
  })

 export function getContent() {
   if (view) {
     return view.state.doc.toString()
   }
   return content
 }

 export function setContent(newContent) {
   if (view) {
     const transaction = view.state.update({
       changes: {
         from: 0,
         to: view.state.doc.length,
         insert: newContent,
       },
     })
     view.dispatch(transaction)
   }
 }
</script>

<div class="h-full w-full flex flex-col">
  {#if $activeFile}
    <div class="flex items-center justify-between px-3 py-1 border-b" style="background-color: var(--bg-secondary); border-color: var(--border);">
      <div class="flex items-center gap-2">
        <span class="text-sm" style="color: var(--text-primary);">
          {$activeFile.split(/[\\/]/).pop()}
        </span>
        {#if isDirty}
          <span class="text-xs" style="color: var(--text-secondary);">●</span>
        {/if}
      </div>
      <div class="flex items-center gap-1">
        <button
          class="px-2 py-1 rounded text-xs transition-colors hover:bg-dark"
          style="color: var(--text-secondary);"
          onclick={saveFile}
          title="Ctrl+S"
        >
          {#if isDirty}● {/if}{$t('common.save')}
        </button>
      </div>
    </div>
  {/if}

  <div
    bind:this={editorContainer}
    class="flex-1 overflow-hidden"
    style="background-color: var(--bg-primary);"
    onclick={() => { view?.focus(); const ce = view?.dom.querySelector('.cm-content'); if (ce) ce.focus() }}
    onkeydown={(e) => { if (e.key === 'Enter' || e.key === ' ') { view?.focus(); const ce = view?.dom.querySelector('.cm-content'); if (ce) ce.focus() }}}
    role="button"
    tabindex="0"
    aria-label="Focus editor"
  ></div>
</div>

<style>
:global(.cm-editor .cm-content) {
  caret-color: transparent;
}



/* Cursor blink animations */
:global(.cm-cursor-blink) { animation: cm-blink 1.06s steps(1) infinite; }
:global(.cm-cursor-smooth) { animation: cm-smooth 1.06s ease-in-out infinite; }
:global(.cm-cursor-phase) { animation: cm-phase 1.06s ease-in-out infinite; }
:global(.cm-cursor-expand) { animation: cm-expand 1.06s ease-in-out infinite; }
:global(.cm-cursor-solid) { animation: none !important; }

@keyframes cm-blink {
  0%, 49% { opacity: 1; }
  50%, 100% { opacity: 0; }
}
@keyframes cm-smooth {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.15; }
}
@keyframes cm-phase {
  0%, 100% { opacity: 1; transform: scaleY(1); }
  50% { opacity: 0.4; transform: scaleY(0.1); }
}
@keyframes cm-expand {
  0%, 90%, 100% { opacity: 1; transform: scaleY(1); }
  95% { opacity: 0.6; transform: scaleY(1.4); }
}
</style>