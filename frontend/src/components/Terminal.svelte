<script>
  import { Terminal } from '@xterm/xterm'
  import '@xterm/xterm/css/xterm.css'
  import { FitAddon } from '@xterm/addon-fit'
  import { WebLinksAddon } from '@xterm/addon-web-links'
  import { onMount, onDestroy } from 'svelte'
  import { EventsOn, EventsOff } from '../../wailsjs/runtime/runtime.js'
  import { currentTheme, themes } from '../stores/theme.js'

  let { terminalId } = $props()

  function getTerminalTheme(themeId) {
    const t = themes.find(th => th.id === (themeId || 'dark')) || themes[0]
    const c = t.colors
    const isLight = themeId === 'light'
    return {
      background: c.bg,
      foreground: c.text,
      cursor: c.accent,
      cursorAccent: c.bg,
      selectionBackground: c.accent,
      selectionForeground: c.bg,
      black: isLight ? '#e5e5e5' : '#000000',
      red: '#cd3131',
      green: '#0dbc79',
      yellow: isLight ? '#795e00' : '#e5e510',
      blue: '#2472c8',
      magenta: '#bc3fbc',
      cyan: '#11a8cd',
      white: isLight ? '#000000' : '#e5e5e5',
      brightBlack: '#666666',
      brightRed: '#f14c4c',
      brightGreen: '#23d18b',
      brightYellow: '#f5f543',
      brightBlue: '#3b8eea',
      brightMagenta: '#d670d6',
      brightCyan: '#29b8db',
      brightWhite: isLight ? '#555555' : '#e5e5e5',
    }
  }

  let terminalContainer = $state()
  let term
  let fitAddon
  let ptyReady = false
  let offOutput = null
  let offExit = null
  let runHandler = null
  let clearHandler = null
  let refitHandler = null

  let exited = $state(false)
  let exitCode = $state(null)

  const RESIZE_DEBOUNCE_MS = 100
  let resizeTimer = null

  let startupPending = true
  let startupTimer = null
  let outputCount = 0

  function fitAndResize() {
    if (!fitAddon || !ptyReady || !window.backend) return
    try {
      fitAddon.fit()
    } catch {}
    if (term.cols && term.rows) {
      window.backend.TerminalResize(terminalId, term.cols, term.rows)
    }
  }

  function doStartupClear() {
    if (!startupPending || !term || !ptyReady) return
    startupPending = false
    if (startupTimer) { clearTimeout(startupTimer); startupTimer = null }
    if (window.backend) {
      window.backend.TerminalWrite(terminalId, 'Clear-Host\r')
    }
  }

  onMount(async () => {
    term = new Terminal({
      fontSize: 14,
      fontFamily: "'Cascadia Code', 'JetBrains Mono', 'Fira Code', 'Consolas', 'Courier New', monospace",
      fontWeight: 'normal',
      fontWeightBold: 'bold',
      lineHeight: 1.15,
      letterSpacing: 0,
      convertEol: true,
      theme: getTerminalTheme($currentTheme),
      cursorBlink: true,
      cursorStyle: 'block',
      scrollback: 10000,
      allowProposedApi: true,
    })

    fitAddon = new FitAddon()
    term.loadAddon(fitAddon)

    try {
      const webLinksAddon = new WebLinksAddon()
      term.loadAddon(webLinksAddon)
    } catch {}

    term.open(terminalContainer)

    requestAnimationFrame(() => {
      try { fitAddon.fit() } catch {}
    })

    const outputEvent = 'terminal:output:' + terminalId
    const exitEvent = 'terminal:exit:' + terminalId

    term.onData((data) => {
      if (!ptyReady || !window.backend) return
      window.backend.TerminalWrite(terminalId, data)
    })

    term.onResize(({ cols, rows }) => {
      if (!ptyReady || !window.backend) return
      window.backend.TerminalResize(terminalId, cols, rows)
    })

    offOutput = EventsOn(outputEvent, (data) => {
      if (!term || typeof data !== 'string') return
      term.write(data)
      if (startupPending) {
        outputCount++
        if (outputCount >= 4) {
          doStartupClear()
        }
      }
    })

    offExit = EventsOn(exitEvent, (code) => {
      ptyReady = false
      startupPending = false
      exited = true
      exitCode = code ?? 0
      if (term) {
        term.write('\r\n\x1b[90m\u276E \u7EC8\u7AEF\u8FDB\u7A0B\u5DF2\u9000\u51FA \u276F\x1b[0m\r\n')
      }
      window.dispatchEvent(new CustomEvent('terminal:exited', { detail: { id: terminalId, exitCode: code ?? 0 } }))
    })

    ptyReady = true

    requestAnimationFrame(() => {
      if (!ptyReady) return
      try { fitAddon.fit() } catch {}
      if (term.cols && term.rows && window.backend) {
        window.backend.TerminalResize(terminalId, term.cols, term.rows)
      }
    })

    if (window.backend) {
      window.backend.ConnectTerminal(terminalId)
    }

    // Update terminal theme when app theme changes
    const themeUnsub = currentTheme.subscribe((id) => {
      if (term) {
        term.options.theme = getTerminalTheme(id)
        term.refresh(0, term.rows)
      }
    })

    startupTimer = setTimeout(() => {
      doStartupClear()
    }, 1500)

    const resizeObserver = new ResizeObserver(() => {
      if (!fitAddon || !ptyReady || !window.backend) return
      if (resizeTimer) clearTimeout(resizeTimer)
      resizeTimer = setTimeout(() => {
        resizeTimer = null
        fitAndResize()
      }, RESIZE_DEBOUNCE_MS)
    })
    resizeObserver.observe(terminalContainer)

    runHandler = (e) => {
      if (!ptyReady || !window.backend || exited) return
      const code = e.detail?.code
      if (code) window.backend.TerminalWrite(terminalId, code + '\r')
    }
    window.addEventListener('run-in-terminal', runHandler)

    clearHandler = () => {
      if (term) term.clear()
    }
    window.addEventListener('terminal:clear:' + terminalId, clearHandler)

    refitHandler = () => {
      fitAndResize()
    }
    window.addEventListener('terminal:refit:' + terminalId, refitHandler)
  })

  onDestroy(() => {
    ptyReady = false
    startupPending = false
    if (resizeTimer) { clearTimeout(resizeTimer); resizeTimer = null }
    if (startupTimer) { clearTimeout(startupTimer); startupTimer = null }
    if (offOutput) { offOutput(); offOutput = null }
    if (offExit) { offExit(); offExit = null }
    EventsOff('terminal:output:' + terminalId)
    EventsOff('terminal:exit:' + terminalId)
    if (runHandler) { window.removeEventListener('run-in-terminal', runHandler); runHandler = null }
    if (clearHandler) { window.removeEventListener('terminal:clear:' + terminalId, clearHandler); clearHandler = null }
    if (refitHandler) { window.removeEventListener('terminal:refit:' + terminalId, refitHandler); refitHandler = null }
    if (typeof themeUnsub === 'function') themeUnsub()
    if (term) { term.dispose(); term = null }
  })

  function handleRestart() {
    exited = false
    exitCode = null
    if (term) {
      term.clear()
    }
    window.dispatchEvent(new CustomEvent('terminal:restart', { detail: { id: terminalId } }))
  }
</script>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div bind:this={terminalContainer} class="terminal-container">
  {#if exited}
    <div class="terminal-exited-overlay">
      <svg viewBox="0 0 16 16" class="exited-icon" fill="currentColor">
        <path d="M4 4l8 8M12 4l-8 8"/>
      </svg>
      <span class="exited-message">{exitCode === 0 || exitCode === null ? '\u7EC8\u7AEF\u8FDB\u7A0B\u5DF2\u9000\u51FA' : '\u7EC8\u7AEF\u8FDB\u7A0B\u5F02\u5E38\u9000\u51FA (\u9000\u51FA\u7801: ' + exitCode + ')'}</span>
      <button class="restart-btn" onclick={handleRestart}>
        <svg viewBox="0 0 16 16" class="restart-icon" fill="none" stroke="currentColor" stroke-width="1.5">
          <path d="M2 8a6 6 0 0 1 10.5-4M14 8a6 6 0 0 1-10.5 4"/>
          <path d="M12.5 1.5v3h-3M3.5 14.5v-3h3"/>
        </svg>
        {'\u91CD\u65B0\u542F\u52A8'}
      </button>
    </div>
  {/if}
</div>

<style>
.terminal-container {
  position: relative;
  width: 100%;
  height: 100%;
  background-color: var(--bg-primary);
  padding: 0;
  overflow: hidden;
}

:global(.xterm) {
  padding: 4px 2px 0 8px;
  height: 100%;
}

:global(.xterm-screen) {
  caret-color: #aeafad;
}

:global(.xterm-viewport) {
  overflow-y: auto !important;
}

:global(.xterm-viewport::-webkit-scrollbar) {
  width: 10px;
}

:global(.xterm-viewport::-webkit-scrollbar-track) {
  background: transparent;
}

:global(.xterm-viewport::-webkit-scrollbar-thumb) {
  background: rgba(121, 121, 121, 0.4);
  border-radius: 5px;
  background-clip: content-box;
  border: 2px solid transparent;
}

:global(.xterm-viewport::-webkit-scrollbar-thumb:hover) {
  background: rgba(121, 121, 121, 0.7);
  background-clip: content-box;
}

:global(.xterm-viewport::-webkit-scrollbar-corner) {
  background: transparent;
}

.terminal-exited-overlay {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 10px;
  background: rgba(30, 30, 30, 0.75);
  backdrop-filter: blur(2px);
  z-index: 20;
  font-size: 13px;
  color: rgba(212, 212, 212, 0.8);
}

.exited-icon {
  width: 16px;
  height: 16px;
  color: #f14c4c;
  flex-shrink: 0;
}

.exited-message {
  flex-shrink: 0;
}

.restart-btn {
  display: flex;
  align-items: center;
  gap: 5px;
  padding: 4px 12px;
  font-size: 12px;
  color: #d4d4d4;
  background: rgba(82, 139, 255, 0.1);
  border: 1px solid rgba(82, 139, 255, 0.4);
  border-radius: 3px;
  cursor: pointer;
  transition: background-color 0.15s, border-color 0.15s;
}

.restart-btn:hover {
  background: rgba(82, 139, 255, 0.2);
  border-color: rgba(82, 139, 255, 0.6);
}

.restart-icon {
  width: 12px;
  height: 12px;
}
</style>
