<script>
  import { fade, scale } from "svelte/transition";
  import { writable } from "svelte/store";
  import { settingsVisible } from "../stores/app.js";
  import { aiConfig } from "../stores/ai.js";
  import { currentTheme, setTheme, themes } from "../stores/theme.js";
  import { currentLang, setLang, t } from "../stores/i18n.js";
  import {
    editorSettings,
    updateEditorSetting,
  } from "../stores/editorSettings.js";
  import MCPManager from "./MCPManager.svelte";
  import TokenUsagePanel from "./TokenUsagePanel.svelte";
  import SettingsAI from "./SettingsAI.svelte";
  import {
    GetSkills,
    SaveSkill,
    DeleteSkill,
    InstallSkillFromURL,
  } from "../../wailsjs/go/main/App.js";

  let activeTab = $state("general");
  let aboutChecking = $state(false);
  let aboutUpdateMsg = $state("");

  // Skills tab state
  let skills = writable(/** @type {Array<any>} */ ([]));
  let showCreateSkillDialog = $state(false);
  let newSkillId = $state("");
  let newSkillName = $state("");
  let newSkillIcon = $state("🔧");
  let newSkillDesc = $state("");
  let newSkillPrompt = $state("");

  async function loadSkills() {
    try {
      const list = await GetSkills();
      skills.set(list || []);
    } catch (/** @type {any} */ e) {
      console.error("Load skills failed:", e);
    }
  }

  async function createSkill() {
    if (!newSkillId || !newSkillName) return;
    try {
      await SaveSkill({
        id: newSkillId.trim().toLowerCase().replace(/\s+/g, "-"),
        name: newSkillName,
        icon: newSkillIcon || "🔧",
        description: newSkillDesc,
        promptTemplate: newSkillPrompt,
        trigger: "manual",
        resultType: "text",
        category: "external",
        associatedAgents: ["universal-assistant"],
      });
      showCreateSkillDialog = false;
      newSkillId = "";
      newSkillName = "";
      newSkillDesc = "";
      newSkillPrompt = "";
      loadSkills();
    } catch (e) {
      console.error("Create skill failed:", e);
    }
  }

  /** @param {string} skillId */
  async function deleteSkill(skillId) {
    if (!confirm("Delete this skill?")) return;
    try {
      await DeleteSkill(skillId);
      loadSkills();
    } catch (e) {
      console.error("Delete skill failed:", e);
    }
  }

  // URL import state
  let showImportSkillDialog = $state(false);
  let importSkillUrl = $state("");
  let importSkillLoading = $state(false);
  let importSkillMsg = $state("");
  let importSkillOk = $state(false);

  async function installSkillFromUrl() {
    if (!importSkillUrl) return;
    importSkillLoading = true;
    importSkillMsg = "";
    try {
      await InstallSkillFromURL(importSkillUrl);
      importSkillOk = true;
      importSkillMsg = "安装成功！技能已添加到列表。";
      importSkillUrl = "";
      loadSkills();
    } catch (/** @type {any} */ e) {
      importSkillOk = false;
      importSkillMsg = "安装失败: " + (e.message || String(e));
    }
    importSkillLoading = false;
  }

  const tabs = [
    {
      id: "general",
      labelKey: "settings.general",
      icon: "M4 6h16M4 12h16M4 18h16",
    },
    {
      id: "appearance",
      labelKey: "settings.appearance",
      icon: "M7 21a4 4 0 01-4-4V5a2 2 0 012-2h4a2 2 0 012 2v12a4 4 0 01-4 4zm0 0h12a2 2 0 002-2v-4a2 2 0 00-2-2h-2.343M11 7.343l1.657-1.657a2 2 0 012.828 0l2.829 2.829a2 2 0 010 2.828l-8.486 8.485M7 17h.01",
    },
    {
      id: "editor",
      labelKey: "settings.editor",
      icon: "M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z",
    },
    {
      id: "terminal",
      labelKey: "settings.terminal",
      icon: "M4 6h16M4 12h16M4 18h16",
    },
    {
      id: "ai",
      labelKey: "settings.ai",
      icon: "M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z",
    },
    { id: "skills", label: "Skills", icon: "M13 10V3L4 14h7v7l9-11h-7z" },
    {
      id: "mcp",
      labelKey: "settings.mcp",
      icon: "M8 9l3 3-3 3m5 0h3M5 20h14a2 2 0 002-2V6a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z",
    },
    {
      id: "tokenUsage",
      labelKey: "settings.tokenUsage",
      icon: "M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10a2 2 0 01-2 2h-2a2 2 0 01-2-2zm0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z",
    },
    {
      id: "about",
      labelKey: "settings.about",
      icon: "M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z",
    },
  ];

  let settings = $state({
    theme: "dark",
    fontSize: 14,
    fontFamily: "Cascadia Code",
    wordWrap: true,
    lineNumbers: true,
    minimap: false,
    autoSave: false,
    terminalFontSize: 14,
    terminalFontFamily: "Cascadia Code",
    provider: "openai",
    apiKey: "",
    model: "gpt-4",
    endpoint: "https://api.openai.com/v1/chat/completions",
    temperature: 0.7,
    maxTokens: 4096,
  });

  /** @param {string} tabId */
  function setTab(tabId) {
    activeTab = tabId;
    if (tabId === "skills") loadSkills();
  }

  function syncAiConfig() {
    aiConfig.set({
      provider: settings.provider,
      apiKey: settings.apiKey,
      model: settings.model,
      endpoint: settings.endpoint,
      temperature: settings.temperature,
      maxTokens: settings.maxTokens,
    });
  }

  function saveSettings() {
    localStorage.setItem("starcore-settings", JSON.stringify(settings));
    syncAiConfig();
  }

  function loadSettings() {
    const saved = localStorage.getItem("starcore-settings");
    if (saved) {
      settings = { ...settings, ...JSON.parse(saved) };
    }
    syncAiConfig();
    applyUIFont();
  }

  // Apply UI font family on load
  applyUIFont();

  function applyUIFont() {
    const family =
      settings.fontFamily === "system"
        ? "-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif"
        : "'" + settings.fontFamily + "', monospace";
    // Set on <html> root so REM-based Tailwind utilities (text-sm, text-xs)
    // in sidebar, file tree, tabs etc. all scale with the user preference.
    document.documentElement.style.fontFamily = family;
    document.documentElement.style.fontSize = settings.fontSize + "px";
    // Also set body for non-REM consumers
    document.body.style.fontFamily = family;
  }

  async function checkUpdate() {
    aboutChecking = true;
    aboutUpdateMsg = "";
    try {
      await new Promise((r) => setTimeout(r, 1500));
      aboutUpdateMsg = "upToDate";
    } catch {
      aboutUpdateMsg = "error";
    }
    aboutChecking = false;
  }

  loadSettings();
</script>

{#if $settingsVisible}
  <div
    class="dialog-backdrop"
    transition:fade={{ duration: 150 }}
    onclick={(e) => {
      if (e.target === e.currentTarget) settingsVisible.set(false);
    }}
    role="button"
    tabindex="-1"
    onkeydown={(e) => {
      if (e.key === "Escape") settingsVisible.set(false);
    }}
  >
    <div
      class="dialog-content flex"
      transition:scale={{ duration: 200, start: 0.95 }}
      style="width: min(800px, 90vw); height: min(600px, 85vh);"
    >
      <div
        class="w-48 border-r flex flex-col"
        style="background-color: var(--bg-secondary); border-color: var(--border);"
      >
        <div class="p-4 border-b" style="border-color: var(--border);">
          <h2 class="text-lg font-semibold" style="color: var(--text-primary);">
            {$t("settings.title")}
          </h2>
        </div>
        <div class="flex-1 py-2 overflow-y-auto">
          {#each tabs as tab}
            <button
              class="w-full flex items-center gap-3 px-4 py-2.5 text-sm transition-colors"
              style="background-color: {activeTab === tab.id
                ? 'var(--bg-primary)'
                : 'transparent'}; color: {activeTab === tab.id
                ? 'var(--text-primary)'
                : 'var(--text-secondary)'}; border-radius: 0;"
              onclick={() => setTab(tab.id)}
            >
              <svg
                xmlns="http://www.w3.org/2000/svg"
                class="w-4 h-4"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d={tab.icon}
                />
              </svg>
              {tab.label || $t(tab.labelKey || "")}
            </button>
          {/each}
        </div>
      </div>

      <div class="flex-1 flex flex-col">
        <div
          class="flex items-center justify-between px-6 py-4 border-b"
          style="border-color: var(--border);"
        >
          <h3 class="text-lg font-medium" style="color: var(--text-primary);">
            {tabs.find((tb) => tb.id === activeTab)?.label ||
              $t(tabs.find((tb) => tb.id === activeTab)?.labelKey || "")}
          </h3>
          <button
            class="btn btn-ghost btn-icon"
            onclick={() => settingsVisible.set(false)}
            aria-label={$t("common.close")}
          >
            <svg
              xmlns="http://www.w3.org/2000/svg"
              class="w-5 h-5"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M6 18L18 6M6 6l12 12"
              />
            </svg>
          </button>
        </div>

        <div class="flex-1 overflow-y-auto p-6">
          {#if activeTab === "general"}
            <div class="space-y-6">
              <div>
                <label
                  for="settings-autoSave"
                  class="block text-sm font-medium mb-2"
                  style="color: var(--text-primary);"
                  >{$t("settings.general.autoSave")}</label
                >
                <div class="flex items-center gap-2">
                  <input
                    id="settings-autoSave"
                    type="checkbox"
                    bind:checked={settings.autoSave}
                    class="rounded"
                    onchange={saveSettings}
                  />
                  <span class="text-sm" style="color: var(--text-secondary);"
                    >{$t("settings.general.autoSaveDesc")}</span
                  >
                </div>
              </div>
            </div>
          {:else if activeTab === "appearance"}
            <div class="space-y-6">
              <div class="space-y-3">
                <h3
                  class="text-sm font-medium"
                  style="color: var(--text-primary);"
                >
                  {$t("settings.appearance.theme")}
                </h3>
                <div
                  class="grid gap-1.5"
                  style="grid-template-columns: repeat(auto-fill, minmax(90px, 1fr));"
                >
                  {#each themes as theme}
                    <button
                      class="p-2 rounded text-xs transition-all text-left leading-tight"
                      style="background-color: {theme.colors.bg}; color: {theme.colors.text}; border: 2px solid {$currentTheme === theme.id ? theme.colors.accent : theme.colors.border}; min-height: 52px;"
                      onclick={() => setTheme(theme.id)}
                    >
                      <div class="flex gap-1 mb-0.5">
                        <span
                          class="w-2 h-2 rounded-full shrink-0"
                          style="background-color: {theme.colors.accent};"
                        ></span>
                        <span
                          class="w-2 h-2 rounded-full shrink-0"
                          style="background-color: {theme.colors
                            .text}; opacity: 0.5;"
                        ></span>
                        <span
                          class="w-2 h-2 rounded-full shrink-0"
                          style="background-color: {theme.colors
                            .text2}; opacity: 0.5;"
                        ></span>
                      </div>
                      <span class="font-medium truncate block">{theme.name}</span>
                      {#if $currentTheme === theme.id}
                        <span class="ml-0.5" style="color: {theme.colors.accent};"
                          >✓</span
                        >
                      {/if}
                    </button>
                  {/each}
                </div>
              </div>

              <div class="space-y-3">
                <h3
                  class="text-sm font-medium"
                  style="color: var(--text-primary);"
                >
                  {$t("settings.appearance.language")}
                </h3>
                <div class="flex gap-2">
                  {#each [{ id: "zh", key: "lang.zh" }, { id: "en", key: "lang.en" }] as lang}
                    <button
                      class="flex-1 p-3 rounded text-sm transition-colors"
                      style="background-color: {$currentLang === lang.id
                        ? '#094771'
                        : 'var(--bg-secondary)'}; color: var(--text-primary); border: 2px solid {$currentLang ===
                      lang.id
                        ? 'var(--accent)'
                        : 'var(--border)'};"
                      onclick={() => setLang(lang.id)}
                    >
                      {$t(lang.key)}
                    </button>
                  {/each}
                </div>
              </div>

              <div>
                <label
                  for="settings-ui-fontfamily"
                  class="block text-sm font-medium mb-2"
                  style="color: var(--text-primary);"
                  >{$t("settings.appearance.fontFamily")}</label
                >
                <select
                  id="settings-ui-fontfamily"
                  bind:value={settings.fontFamily}
                  class="w-full px-3 py-2 rounded border text-sm"
                  style="background-color: var(--bg-primary); color: var(--text-primary); border-color: var(--border);"
                  onchange={() => {
                    saveSettings();
                    applyUIFont();
                  }}
                >
                  <option value="system">System Default</option>
                  <option value="Lilex">Lilex</option>
                  <option value="StarCore Mono">StarCore Mono</option>
                  <option value="Cascadia Code">Cascadia Code</option>
                  <option value="JetBrains Mono">JetBrains Mono</option>
                  <option value="Fira Code">Fira Code</option>
                  <option value="Consolas">Consolas</option>
                </select>
              </div>

              <div>
                <label
                  for="settings-ui-fontsize"
                  class="block text-sm font-medium mb-2"
                  style="color: var(--text-primary);"
                  >{$t("settings.appearance.fontSize")}</label
                >
                <div class="flex items-center gap-3">
                  <input
                    id="settings-ui-fontsize"
                    type="range"
                    bind:value={settings.fontSize}
                    min="10"
                    max="24"
                    class="flex-1"
                    oninput={() => {
                      saveSettings();
                      applyUIFont();
                    }}
                  />
                  <span
                    class="text-sm font-mono"
                    style="color: var(--text-primary); min-width: 28px;"
                    >{settings.fontSize}px</span
                  >
                </div>
              </div>
            </div>
          {:else if activeTab === "editor"}
            <!-- svelte-ignore a11y_label_has_associated_control -->
            <div class="space-y-6">
              <div>
                <label
                  for="settings-editor-fontsize"
                  class="block text-sm font-medium mb-2"
                  style="color: var(--text-primary);"
                  >{$t("settings.editor.fontSize")}</label
                >
                <div class="flex items-center gap-3">
                  <input
                    type="range"
                    value={$editorSettings.fontSize}
                    min="11"
                    max="28"
                    class="flex-1"
                    oninput={(e) =>
                      updateEditorSetting(
                        "fontSize",
                        parseInt(
                          /** @type {HTMLInputElement} */ (e.target).value,
                        ),
                      )}
                  />
                  <span
                    class="text-sm font-mono"
                    style="color: var(--text-primary); min-width: 28px;"
                    >{$editorSettings.fontSize}px</span
                  >
                </div>
              </div>
              <div>
                <label
                  for="settings-editor-fontfamily"
                  class="block text-sm font-medium mb-2"
                  style="color: var(--text-primary);"
                  >{$t("settings.editor.fontFamily")}</label
                >
                <select
                  id="settings-editor-fontfamily"
                  value={$editorSettings.fontFamily}
                  class="w-full px-3 py-2 rounded border text-sm"
                  style="background-color: var(--bg-primary); color: var(--text-primary); border-color: var(--border);"
                  onchange={(e) =>
                    updateEditorSetting(
                      "fontFamily",
                      /** @type {HTMLSelectElement} */ (e.target).value,
                    )}
                >
                  <option
                    value="'Lilex', 'Cascadia Code', 'JetBrains Mono', 'Consolas', 'monospace'"
                    >Lilex</option
                  >
                  <option
                    value="'StarCore Mono', 'Cascadia Code', 'JetBrains Mono', 'Consolas', 'monospace'"
                    >StarCore Mono</option
                  >
                  <option
                    value="'Cascadia Code', 'JetBrains Mono', 'Fira Code', 'Consolas', 'monospace'"
                    >Cascadia Code</option
                  >
                  <option
                    value="'JetBrains Mono', 'Cascadia Code', 'Fira Code', 'Consolas', 'monospace'"
                    >JetBrains Mono</option
                  >
                  <option
                    value="'Fira Code', 'Cascadia Code', 'JetBrains Mono', 'Consolas', 'monospace'"
                    >Fira Code</option
                  >
                  <option
                    value="'Consolas', 'Cascadia Code', 'JetBrains Mono', 'monospace'"
                    >Consolas</option
                  >
                  <option
                    value="'Source Code Pro', 'Cascadia Code', 'Consolas', 'monospace'"
                    >Source Code Pro</option
                  >
                  <option value="'Monaco', 'Consolas', 'monospace'"
                    >Monaco</option
                  >
                </select>
              </div>

              <div>
                <label
                  class="block text-sm font-medium mb-2"
                  style="color: var(--text-primary);"
                  >{$t("settings.editor.lineHeight")}</label
                >
                <div class="flex items-center gap-3">
                  <input
                    type="range"
                    value={$editorSettings.lineHeight}
                    min="1.2"
                    max="2.4"
                    step="0.1"
                    class="flex-1"
                    oninput={(e) =>
                      updateEditorSetting(
                        "lineHeight",
                        parseFloat(
                          /** @type {HTMLInputElement} */ (e.target).value,
                        ),
                      )}
                  />
                  <span
                    class="text-sm font-mono"
                    style="color: var(--text-primary); min-width: 28px;"
                    >{$editorSettings.lineHeight}</span
                  >
                </div>
              </div>
              <div>
                <label
                  class="block text-sm font-medium mb-2"
                  style="color: var(--text-primary);"
                  >{$t("settings.editor.wordWrap")}</label
                >
                <div class="flex items-center gap-2">
                  <input
                    type="checkbox"
                    checked={$editorSettings.wordWrap}
                    class="rounded"
                    onchange={(e) =>
                      updateEditorSetting(
                        "wordWrap",
                        /** @type {HTMLInputElement} */ (e.target).checked,
                      )}
                  />
                  <span class="text-sm" style="color: var(--text-secondary);"
                    >{$t("settings.editor.wordWrapText")}</span
                  >
                </div>
              </div>

              <div class="pt-4 border-t" style="border-color: var(--border);">
                <h3
                  class="text-sm font-medium mb-4"
                  style="color: var(--text-primary);"
                >
                  {$t("settings.editor.cursor")}
                </h3>
                <div class="space-y-4">
                  <div class="flex gap-4">
                    <div class="flex-1">
                      <label
                        class="block text-xs mb-1.5"
                        style="color: var(--text-secondary);"
                        >{$t("settings.editor.cursorWidth")}</label
                      >
                      <div class="flex items-center gap-2">
                        <input
                          type="range"
                          value={$editorSettings.cursorWidth}
                          min="1"
                          max="6"
                          class="flex-1"
                          oninput={(e) =>
                            updateEditorSetting(
                              "cursorWidth",
                              parseInt(
                                /** @type {HTMLInputElement} */ (e.target)
                                  .value,
                              ),
                            )}
                        />
                        <span
                          class="text-xs font-mono"
                          style="color: var(--text-primary); min-width: 16px;"
                          >{$editorSettings.cursorWidth}px</span
                        >
                      </div>
                    </div>
                    <div class="flex-1">
                      <label
                        class="block text-xs mb-1.5"
                        style="color: var(--text-secondary);"
                        >{$t("settings.editor.cursorColor")}</label
                      >
                      <div class="flex items-center gap-2">
                        <input
                          type="color"
                          value={$editorSettings.cursorColor}
                          class="w-8 h-8 rounded border cursor-pointer"
                          style="border-color: var(--border);"
                          oninput={(e) =>
                            updateEditorSetting(
                              "cursorColor",
                              /** @type {HTMLInputElement} */ (e.target).value,
                            )}
                        />
                        <span
                          class="text-xs font-mono"
                          style="color: var(--text-primary);"
                          >{$editorSettings.cursorColor}</span
                        >
                      </div>
                    </div>
                  </div>
                  <div class="flex gap-4">
                    <div class="flex-1">
                      <label
                        class="block text-xs mb-1.5"
                        style="color: var(--text-secondary);"
                        >{$t("editor.cursorHeight")}</label
                      >
                      <div class="flex items-center gap-2">
                        <input
                          type="range"
                          value={$editorSettings.cursorHeight}
                          min="20"
                          max="100"
                          step="5"
                          class="flex-1"
                          oninput={(e) =>
                            updateEditorSetting(
                              "cursorHeight",
                              parseInt(
                                /** @type {HTMLInputElement} */ (e.target)
                                  .value,
                              ),
                            )}
                        />
                        <span
                          class="text-xs font-mono"
                          style="color: var(--text-primary); min-width: 28px;"
                          >{$editorSettings.cursorHeight}%</span
                        >
                      </div>
                    </div>
                    <div class="flex-1">
                      <label
                        class="block text-xs mb-1.5"
                        style="color: var(--text-secondary);"
                        >{$t("settings.editor.cursorStyle")}</label
                      >
                      <select
                        value={$editorSettings.cursorStyle}
                        class="w-full px-2 py-1.5 rounded border text-xs"
                        style="background-color: var(--bg-primary); color: var(--text-primary); border-color: var(--border);"
                        onchange={(e) =>
                          updateEditorSetting(
                            "cursorStyle",
                            /** @type {HTMLSelectElement} */ (e.target).value,
                          )}
                      >
                        <option value="block"
                          >{$t("settings.editor.cursorStyleBlock")}</option
                        >
                        <option value="line"
                          >{$t("settings.editor.cursorStyleLine")}</option
                        >
                        <option value="underline"
                          >{$t("settings.editor.cursorStyleUnderline")}</option
                        >
                      </select>
                    </div>
                    <div class="flex-1">
                      <label
                        class="block text-xs mb-1.5"
                        style="color: var(--text-secondary);"
                        >{$t("settings.editor.cursorBlinkStyle")}</label
                      >
                      <select
                        value={$editorSettings.cursorBlinkStyle}
                        class="w-full px-2 py-1.5 rounded border text-xs"
                        style="background-color: var(--bg-primary); color: var(--text-primary); border-color: var(--border);"
                        onchange={(e) =>
                          updateEditorSetting(
                            "cursorBlinkStyle",
                            /** @type {HTMLSelectElement} */ (e.target).value,
                          )}
                      >
                        <option value="blink"
                          >{$t("settings.editor.cursorBlinkBlink")}</option
                        >
                        <option value="smooth"
                          >{$t("settings.editor.cursorBlinkSmooth")}</option
                        >
                        <option value="phase"
                          >{$t("settings.editor.cursorBlinkPhase")}</option
                        >
                        <option value="expand"
                          >{$t("settings.editor.cursorBlinkExpand")}</option
                        >
                        <option value="solid"
                          >{$t("settings.editor.cursorBlinkSolid")}</option
                        >
                      </select>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          {:else if activeTab === "terminal"}
            <!-- svelte-ignore a11y_label_has_associated_control -->
            <div class="space-y-6">
              <div>
                <label
                  class="block text-sm font-medium mb-2"
                  style="color: var(--text-primary);"
                  >{$t("settings.terminal.fontSize")}</label
                >
                <input
                  type="range"
                  bind:value={settings.terminalFontSize}
                  min="10"
                  max="24"
                  class="w-full"
                  onchange={saveSettings}
                />
                <span class="text-sm" style="color: var(--text-secondary);"
                  >{settings.terminalFontSize}px</span
                >
              </div>
              <div>
                <label
                  class="block text-sm font-medium mb-2"
                  style="color: var(--text-primary);"
                  >{$t("settings.terminal.fontFamily")}</label
                >
                <select
                  bind:value={settings.terminalFontFamily}
                  class="w-full px-3 py-2 rounded border text-sm"
                  style="background-color: var(--bg-primary); color: var(--text-primary); border-color: var(--border);"
                  onchange={saveSettings}
                >
                  <option value="Cascadia Code">Cascadia Code</option>
                  <option value="JetBrains Mono">JetBrains Mono</option>
                  <option value="Fira Code">Fira Code</option>
                  <option value="Consolas">Consolas</option>
                </select>
              </div>
            </div>
          {:else if activeTab === "ai"}
            <SettingsAI />
          {:else if activeTab === "skills"}
            <div class="space-y-6">
              <div>
                <h3
                  class="text-sm font-medium mb-1"
                  style="color: var(--text-primary);"
                >
                  已安装的扩展技能
                </h3>
                <p class="text-xs" style="color: var(--text-secondary);">
                  系统内置 {$skills.filter((s) => s.category !== "external")
                    .length} 个技能，在对话中用 /技能名 触发。以下是你自己安装的扩展技能：
                </p>
              </div>
              <div class="flex gap-2">
                <button
                  class="px-4 py-2 rounded text-sm font-medium"
                  style="background-color: var(--accent); color: #fff;"
                  onclick={() => (showCreateSkillDialog = true)}
                  >+ 创建新技能</button
                >
                <button
                  class="px-4 py-2 rounded text-sm"
                  style="background-color: var(--border); color: var(--text-primary);"
                  onclick={() => (showImportSkillDialog = true)}
                  >从 URL 安装</button
                >
              </div>
              <div
                class="rounded-lg overflow-hidden border"
                style="border-color: var(--border);"
              >
                {#if $skills.filter((s) => s.category === "external").length > 0}
                  <table
                    class="w-full text-sm"
                    style="border-collapse: collapse;"
                  >
                    <thead>
                      <tr style="background-color: var(--bg-secondary);">
                        <th
                          class="text-left px-4 py-2 font-medium text-xs"
                          style="color: var(--text-secondary);">技能</th
                        >
                        <th
                          class="text-left px-4 py-2 font-medium text-xs"
                          style="color: var(--text-secondary);">说明</th
                        >
                        <th
                          class="text-center px-4 py-2 font-medium text-xs"
                          style="color: var(--text-secondary);">命令</th
                        >
                        <th
                          class="text-center px-4 py-2 font-medium text-xs"
                          style="color: var(--text-secondary); width: 40px;"
                        ></th>
                      </tr>
                    </thead>
                    <tbody>
                      {#each $skills.filter((s) => s.category === "external") as skill}
                        <tr
                          class="border-t"
                          style="border-color: var(--border);"
                        >
                          <td
                            class="px-4 py-2.5"
                            style="color: var(--text-primary);"
                          >
                            <span class="mr-2">{skill.icon || "📋"}</span
                            >{skill.name}
                          </td>
                          <td
                            class="px-4 py-2.5 text-xs"
                            style="color: var(--text-muted);"
                            >{skill.description}</td
                          >
                          <td class="text-center px-4 py-2.5">
                            <code
                              class="text-[11px] px-1.5 py-0.5 rounded"
                              style="background-color: var(--bg-tertiary); color: var(--accent); font-family: monospace;"
                              >/{skill.id}</code
                            >
                          </td>
                          <td class="text-center px-2 py-2.5">
                            <button
                              class="p-1 rounded hover:bg-red-500/10"
                              style="color: var(--text-muted);"
                              onclick={() => deleteSkill(skill.id)}
                              title="删除"
                            >
                              <svg
                                xmlns="http://www.w3.org/2000/svg"
                                class="w-3.5 h-3.5"
                                fill="none"
                                viewBox="0 0 24 24"
                                stroke="currentColor"
                                ><path
                                  stroke-linecap="round"
                                  stroke-linejoin="round"
                                  stroke-width="2"
                                  d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"
                                /></svg
                              >
                            </button>
                          </td>
                        </tr>
                      {/each}
                    </tbody>
                  </table>
                {:else}
                  <div
                    class="text-center py-12 text-xs"
                    style="color: var(--text-muted);"
                  >
                    <div class="text-3xl mb-3">🧩</div>
                    <div>还没有安装扩展技能</div>
                    <div class="mt-1">点击上方按钮创建或从 URL 安装</div>
                  </div>
                {/if}
              </div>
            </div>

            {#if showCreateSkillDialog}
              <div
                class="fixed inset-0 z-60 flex items-center justify-center"
                style="background-color: rgba(0,0,0,0.5);"
                onclick={(e) => {
                  if (e.target === e.currentTarget)
                    showCreateSkillDialog = false;
                }}
                role="button"
                tabindex="-1"
                onkeydown={(e) => {
                  if (e.key === "Escape") showCreateSkillDialog = false;
                }}
              >
                <div
                  class="rounded-lg shadow-xl p-6 overflow-y-auto"
                  style="width: 520px; max-height: 85vh; background-color: var(--bg-primary); border: 1px solid var(--border);"
                >
                  <h3
                    class="text-sm font-medium mb-4"
                    style="color: var(--text-primary);"
                  >
                    创建新 Skill
                  </h3>
                  <!-- svelte-ignore a11y_label_has_associated_control -->
                  <div class="space-y-3">
                    <div>
                      <label
                        class="block text-xs mb-1"
                        style="color: var(--text-secondary);"
                        >ID (英文，如 my-code-review)</label
                      >
                      <input
                        class="w-full px-3 py-1.5 rounded border text-sm"
                        style="background-color: var(--bg-secondary); color: var(--text-primary); border-color: var(--border);"
                        bind:value={newSkillId}
                        placeholder="my-skill"
                      />
                    </div>
                    <div>
                      <label
                        class="block text-xs mb-1"
                        style="color: var(--text-secondary);">名称</label
                      >
                      <input
                        class="w-full px-3 py-1.5 rounded border text-sm"
                        style="background-color: var(--bg-secondary); color: var(--text-primary); border-color: var(--border);"
                        bind:value={newSkillName}
                        placeholder="我的技能"
                      />
                    </div>
                    <div>
                      <label
                        class="block text-xs mb-1"
                        style="color: var(--text-secondary);"
                        >图标 (emoji)</label
                      >
                      <input
                        class="w-full px-3 py-1.5 rounded border text-sm"
                        style="background-color: var(--bg-secondary); color: var(--text-primary); border-color: var(--border);"
                        bind:value={newSkillIcon}
                        placeholder="🔍"
                      />
                    </div>
                    <div>
                      <label
                        class="block text-xs mb-1"
                        style="color: var(--text-secondary);">描述</label
                      >
                      <input
                        class="w-full px-3 py-1.5 rounded border text-sm"
                        style="background-color: var(--bg-secondary); color: var(--text-primary); border-color: var(--border);"
                        bind:value={newSkillDesc}
                        placeholder="这个技能用来做什么..."
                      />
                    </div>
                    <div>
                      <label
                        class="block text-xs mb-1"
                        style="color: var(--text-secondary);"
                        >提示词模板 (支持 &#123;code&#125;, &#123;file&#125;,
                        &#123;input&#125; 等变量)</label
                      >
                      <textarea
                        class="w-full px-3 py-1.5 rounded border text-sm"
                        style="background-color: var(--bg-secondary); color: var(--text-primary); border-color: var(--border); min-height: 120px;"
                        bind:value={newSkillPrompt}
                        placeholder="你是一个代码审查专家，请对以下代码进行审查..."
                      ></textarea>
                    </div>
                  </div>
                  <div class="flex justify-end gap-2 mt-4">
                    <button
                      class="px-4 py-1.5 rounded text-sm"
                      style="background-color: var(--border); color: var(--text-primary);"
                      onclick={() => (showCreateSkillDialog = false)}
                      >取消</button
                    >
                    <button
                      class="px-4 py-1.5 rounded text-sm font-medium"
                      style="background-color: var(--accent); color: #fff;"
                      onclick={createSkill}>创建</button
                    >
                  </div>
                </div>
              </div>
            {/if}

            {#if showImportSkillDialog}
              <div
                class="fixed inset-0 z-60 flex items-center justify-center"
                style="background-color: rgba(0,0,0,0.5);"
                onclick={(e) => {
                  if (e.target === e.currentTarget)
                    showImportSkillDialog = false;
                }}
                role="button"
                tabindex="-1"
                onkeydown={(e) => {
                  if (e.key === "Escape") showImportSkillDialog = false;
                }}
              >
                <div
                  class="rounded-lg shadow-xl p-6"
                  style="width: 440px; background-color: var(--bg-primary); border: 1px solid var(--border);"
                >
                  <h3
                    class="text-sm font-medium mb-2"
                    style="color: var(--text-primary);"
                  >
                    从 URL 安装 Skill
                  </h3>
                  <p class="text-xs mb-3" style="color: var(--text-muted);">
                    粘贴一个 SKILL.md 文件的原始链接，自动下载并安装。
                  </p>
                  <input
                    class="w-full px-3 py-1.5 rounded border text-sm mb-3"
                    style="background-color: var(--bg-secondary); color: var(--text-primary); border-color: var(--border);"
                    bind:value={importSkillUrl}
                    placeholder="https://example.com/skills/my-skill/SKILL.md"
                  />
                  {#if importSkillMsg}
                    <div
                      class="text-xs mb-3 px-3 py-2 rounded"
                      style="background-color: {importSkillOk
                        ? '#2ea04315'
                        : '#e8112315'}; color: {importSkillOk
                        ? '#2ea043'
                        : '#e81123'}; border: 1px solid {importSkillOk
                        ? '#2ea04333'
                        : '#e8112333'};"
                    >
                      {importSkillMsg}
                    </div>
                  {/if}
                  <div class="flex justify-end gap-2">
                    <button
                      class="px-4 py-1.5 rounded text-sm"
                      style="background-color: var(--border); color: var(--text-primary);"
                      onclick={() => {
                        showImportSkillDialog = false;
                        importSkillMsg = "";
                        importSkillUrl = "";
                      }}>取消</button
                    >
                    <button
                      class="px-4 py-1.5 rounded text-sm font-medium"
                      style="background-color: var(--accent); color: #fff;"
                      onclick={installSkillFromUrl}
                      disabled={importSkillLoading}
                    >
                      {importSkillLoading ? "安装中..." : "安装"}
                    </button>
                  </div>
                </div>
              </div>
            {/if}
          {:else if activeTab === "mcp"}
            <MCPManager />
          {:else if activeTab === "tokenUsage"}
            <TokenUsagePanel />
          {:else if activeTab === "about"}
            <div class="space-y-8">
              <div class="flex flex-col items-center pt-6">
                <img
                  src="./src/assets/images/logo-universal.png"
                  alt="StarCore"
                  class="w-16 h-16 mb-4"
                />
                <h2 class="text-2xl font-bold" style="color: var(--accent);">
                  StarCore
                </h2>
                <p class="text-sm mt-2" style="color: var(--text-secondary);">
                  {$t("app.description")}
                </p>
              </div>

              <div
                class="rounded-lg p-5 space-y-4"
                style="background-color: var(--bg-secondary); border: 1px solid var(--border);"
              >
                <div class="flex justify-between items-center">
                  <span class="text-sm" style="color: var(--text-secondary);"
                    >{$t("settings.about.version")}</span
                  >
                  <span
                    class="text-sm font-mono"
                    style="color: var(--text-primary);"
                    >{$t("app.version")}</span
                  >
                </div>
                <div class="flex justify-between items-center">
                  <span class="text-sm" style="color: var(--text-secondary);"
                    >{$t("settings.about.license")}</span
                  >
                  <span class="text-sm" style="color: var(--text-primary);"
                    >{$t("app.license")}</span
                  >
                </div>
                <div class="flex justify-between items-center">
                  <span class="text-sm" style="color: var(--text-secondary);">作者 QQ</span>
                  <span class="text-sm font-mono" style="color: var(--text-primary);">89226782</span>
                </div>

              </div>

              <div class="flex justify-center">
                <button
                  class="px-6 py-2.5 rounded-lg text-sm font-medium transition-colors"
                  style="background-color: var(--accent); color: #ffffff;"
                  onclick={checkUpdate}
                  disabled={aboutChecking}
                >
                  {#if aboutChecking}
                    {$t("settings.about.checking")}
                  {:else if aboutUpdateMsg === "upToDate"}
                    {$t("settings.about.upToDate")} âœ“
                  {:else}
                    {$t("settings.about.checkUpdate")}
                  {/if}
                </button>
              </div>
            </div>
          {/if}
        </div>

        <div
          class="flex items-center justify-end gap-3 px-6 py-4 border-t"
          style="border-color: var(--border);"
        >
          <button
            class="px-4 py-2 rounded text-sm transition-colors"
            style="background-color: var(--border); color: var(--text-primary);"
            onclick={() => settingsVisible.set(false)}
          >
            {$t("settings.cancel")}
          </button>
          <button
            class="px-4 py-2 rounded text-sm transition-colors"
            style="background-color: var(--accent); color: #ffffff;"
            onclick={() => {
              saveSettings();
              settingsVisible.set(false);
            }}
          >
            {$t("settings.save")}
          </button>
        </div>
      </div>
    </div>
  </div>
{/if}
