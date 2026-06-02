// Shared localStorage keys — single source of truth to prevent typos
export const KEYS = {
  // App state
  LAST_PROJECT:    'starcore-last-project',
  OPENED_FILES:    'starcore-opened-files',
  LAST_FILE:       'starcore-last-file',

  // AI / Provider
  AI_CONFIG:       'starcore-ai-config',
  ACTIVE_PROVIDER: 'starcore-active-provider',
  ACTIVE_MODEL:    'starcore-active-model',
  CUSTOM_MODELS:   'starcore-custom-models',
  MODEL_ENABLED:   'starcore-model-enabled',

  // UI
  THEME:           'starcore-theme',
  LANG:            'starcore-lang',
  MASTER_MODE:     'starcore-master-mode',
  SETTINGS:        'starcore-settings',
  EDITOR_SETTINGS: 'starcore-editor-settings',

  // Panel dimensions
  SIDEBAR_WIDTH:   'starcore-sidebar-width',
  AI_PANEL_WIDTH:  'starcore-ai-panel-width',
  BOTTOM_HEIGHT:   'starcore-bottom-height',
  WINDOW_STATE:    'starcore-window-state',
}
