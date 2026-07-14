<script>
  import { fly, fade } from "svelte/transition";
  import {
    commandPaletteOpen,
    toggleSidebar,
    toggleAIPanel,
    toggleBottomPanel,
    bottomPanelTab,
  } from "../stores/ui.js";
  import { setTheme } from "../stores/theme.js";
  import {
    settingsVisible,
    splitEditor,
    closeSplit,
    currentProject,
    openProjectFolder,
  } from "../stores/app.js";
  import { sendMessage } from "../stores/ai.js";
  import { runTests } from "../stores/testRunner.js";
  import { addWorkspaceRoot } from "../stores/workspace.js";
  import { t } from "../stores/i18n.js";

  // Reactive commands list — rebuilds whenever language changes
  let commands = $derived([
    { id: "file.open-folder", label: $t("command.openFolder"), category: "文件" },
    { id: "file.save", label: $t("command.save"), category: "文件", shortcut: "Ctrl+S" },
    { id: "file.new-file", label: $t("command.newFile"), category: "文件" },
    { id: "file.find", label: $t("command.findInFiles"), category: "文件", shortcut: "Ctrl+Shift+F" },
    { id: "file.replace", label: $t("command.replace"), category: "文件", shortcut: "Ctrl+Shift+H" },
    { id: "view.toggle-sidebar", label: $t("command.toggleSidebar"), category: "视图", shortcut: "Ctrl+B" },
    { id: "view.toggle-ai-panel", label: $t("command.toggleAIPanel"), category: "视图", shortcut: "Ctrl+Shift+A" },
    { id: "view.toggle-terminal", label: $t("command.toggleTerminal"), category: "视图", shortcut: "Ctrl+`" },
    { id: "view.toggle-bottom-panel", label: $t("command.toggleBottomPanel"), category: "视图" },
    { id: "view.split-editor", label: $t("command.splitEditor"), category: "视图", shortcut: "Ctrl+\\" },
    { id: "view.close-split", label: $t("command.closeSplit"), category: "视图" },
    { id: "view.problems", label: $t("command.problems"), category: "视图" },
    { id: "view.output", label: $t("command.output"), category: "视图" },
    { id: "view.tests", label: $t("command.tests"), category: "视图" },
    { id: "ai.new-chat", label: $t("ai.panel.newChat"), category: "AI", shortcut: "Ctrl+L" },
    { id: "ai.agent-selector", label: $t("ai.selectAgent"), category: "AI", shortcut: "Ctrl+Shift+M" },
    { id: "ai.explain", label: $t("editor.ai.explain"), category: "AI" },
    { id: "ai.fix-bug", label: $t("editor.ai.refactor"), category: "AI" },
    { id: "ai.optimize", label: $t("editor.ai.doc"), category: "AI" },
    { id: "refactor.rename", label: $t("explorer.rename"), category: "重构", shortcut: "F2" },
    { id: "editor.format", label: $t("editor.syntaxHighlight"), category: "编辑器", shortcut: "Shift+Alt+F" },
    { id: "editor.goto-definition", label: $t("editor.gotoDefinition"), category: "编辑器", shortcut: "F12" },
    { id: "test.run-all", label: $t("testRunner.runAll"), category: "测试" },
    { id: "test.run-file", label: $t("testRunner.runAll"), category: "测试" },
    { id: "git.status", label: $t("git.status"), category: "Git" },
    { id: "git.commit", label: $t("git.commit"), category: "Git" },
    { id: "git.push", label: $t("git.push"), category: "Git" },
    { id: "git.pull", label: $t("git.pull"), category: "Git" },
    { id: "workspace.add-folder", label: $t("workspace.addRoot"), category: "工作区" },
    { id: "skill.generate-test", label: $t("editor.ai.test"), category: "Skill" },
    { id: "skill.code-review", label: $t("ai.panel.execSkill"), category: "Skill" },
    { id: "theme.dark", label: $t("theme.dark"), category: "外观" },
    { id: "theme.light", label: $t("theme.light"), category: "外观" },
    { id: "theme.hc", label: $t("theme.hc"), category: "外观" },
    { id: "skill.refactor", label: "Skill: 重构建议", category: "Skill" },
    { id: "skill.generate-doc", label: "Skill: 生成文档", category: "Skill" },
    { id: "skill.explain-code", label: "Skill: 解释代码", category: "Skill" },
    { id: "skill.fix-bug", label: "Skill: 修复 Bug", category: "Skill" },
    { id: "skill.commit-message", label: "Skill: 生成 Commit Message", category: "Skill" },
    { id: "skill.sql-optimize", label: "Skill: SQL 优化", category: "Skill" },
    { id: "settings.open", label: "打开设置", category: "偏好" },
  ]);

  let searchQuery = $state("");
  let selectedIndex = $state(0);

  let filteredCommands = $derived(
    commands.filter(
      (cmd) =>
        cmd.label.toLowerCase().includes(searchQuery.toLowerCase()) ||
        cmd.category.toLowerCase().includes(searchQuery.toLowerCase()),
    ),
  );

  $effect(() => {
    if (
      filteredCommands.length > 0 &&
      selectedIndex >= filteredCommands.length
    ) {
      selectedIndex = 0;
    }
  });

  /** @param {KeyboardEvent} e */
  function handleKeydown(e) {
    if (e.key === "Escape") {
      commandPaletteOpen.set(false);
      return;
    }
    if (e.key === "ArrowDown") {
      e.preventDefault();
      selectedIndex = Math.min(selectedIndex + 1, filteredCommands.length - 1);
      return;
    }
    if (e.key === "ArrowUp") {
      e.preventDefault();
      selectedIndex = Math.max(selectedIndex - 1, 0);
      return;
    }
    if (e.key === "Enter") {
      e.preventDefault();
      executeCommand(filteredCommands[selectedIndex]);
      return;
    }
  }

  /** @param {{ id: string }} cmd */
  function executeCommand(cmd) {
    if (!cmd) return;
    commandPaletteOpen.set(false);
    if (cmd.id.startsWith("skill.")) {
      const skillId = cmd.id.replace("skill.", "");
      window.dispatchEvent(
        new CustomEvent("skill-trigger", { detail: { id: skillId } }),
      );
      return;
    }
    if (cmd.id.startsWith("theme.")) {
      const theme = cmd.id.replace("theme.", "");
      setTheme(theme);
      return;
    }
    switch (cmd.id) {
      case "view.toggle-sidebar":
        toggleSidebar();
        break;
      case "view.toggle-ai-panel":
        toggleAIPanel();
        break;
      case "view.toggle-terminal":
      case "view.toggle-bottom-panel":
        toggleBottomPanel();
        break;
      case "view.problems":
        toggleBottomPanel();
        bottomPanelTab.set("problems");
        break;
      case "view.output":
        toggleBottomPanel();
        bottomPanelTab.set("output");
        break;
      case "view.tests":
        toggleBottomPanel();
        bottomPanelTab.set("tests");
        break;
      case "settings.open":
        settingsVisible.update((v) => !v);
        break;
      case "view.split-editor":
        splitEditor();
        break;
      case "view.close-split":
        closeSplit();
        break;
      case "file.open-folder":
        openProjectFolder();
        break;
      case "editor.format":
        sendMessage("/format code");
        break;
      case "ai.explain":
        sendMessage("解释选中的代码");
        break;
      case "ai.fix-bug":
        sendMessage("修复这个Bug");
        break;
      case "ai.optimize":
        sendMessage("优化这段代码的性能");
        break;
      case "test.run-all":
        runTests();
        toggleBottomPanel();
        bottomPanelTab.set("tests");
        break;
      case "test.run-file":
        runTests("./...");
        toggleBottomPanel();
        bottomPanelTab.set("tests");
        break;
      case "refactor.rename":
        document.dispatchEvent(new CustomEvent("refactor-rename"));
        break;
      case "refactor.extract-function":
        document.dispatchEvent(new CustomEvent("refactor-extract-function"));
        break;
      case "refactor.inline":
        document.dispatchEvent(new CustomEvent("refactor-inline"));
        break;
      case "editor.goto-definition":
        document.dispatchEvent(new CustomEvent("goto-definition"));
        break;
      case "editor.find-references":
        document.dispatchEvent(new CustomEvent("find-references"));
        break;
      case "git.status":
        toggleSidebar();
        break;
      case "git.commit":
        sendMessage("生成 commit message 并提交");
        break;
      case "git.push":
        if (window.backend?.GitPush && $currentProject) window.backend.GitPush($currentProject);
        break;
      case "git.pull":
        if (window.backend?.GitPull && $currentProject) window.backend.GitPull($currentProject);
        break;
      case "workspace.add-folder":
        openProjectFolder().then(f => { if (f) addWorkspaceRoot(f); });
        break;
    }
  }

  $effect(() => {
    if ($commandPaletteOpen) {
      searchQuery = "";
      selectedIndex = 0;
    }
  });
</script>

<svelte:window
  onkeydown={(e) => {
    if (e.ctrlKey && e.shiftKey && e.key === "P") {
      e.preventDefault();
      commandPaletteOpen.update((v) => !v);
    }
  }}
/>

<!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions, a11y_no_noninteractive_element_interactions -->
{#if $commandPaletteOpen}
  <div
    class="dialog-backdrop justify-center"
    style="padding-top: 15vh; align-items: center;"
    transition:fade={{ duration: 100 }}
    onclick={(e) => {
      if (e.target === e.currentTarget) commandPaletteOpen.set(false);
    }}
  >
    <!-- svelte-ignore a11y_no_static_element_interactions -->
    <div
      class="dialog-content w-full max-w-lg overflow-hidden"
      transition:fly={{ y: -16, duration: 150 }}
      onkeydown={handleKeydown}
    >
      <div
        class="flex items-center px-4 border-b"
        style="border-color: var(--border);"
      >
        <svg
          xmlns="http://www.w3.org/2000/svg"
          class="w-4 h-4 shrink-0"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
          style="color: var(--text-muted);"
        >
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"
          />
        </svg>
        <input
          type="text"
          bind:value={searchQuery}
          placeholder="输入命令..."
          class="flex-1 px-3 py-3 text-sm outline-none border-none"
          style="background-color: transparent; color: var(--text-primary);"
        />
      </div>

      <div class="max-h-64 overflow-y-auto py-1">
        {#each filteredCommands as cmd, i}
          <button
            class="dropdown-item justify-between"
            style="background-color: {i === selectedIndex
              ? 'var(--selection)'
              : 'transparent'}; color: {i === selectedIndex
              ? 'var(--text-on-accent)'
              : 'var(--text-primary)'};"
            onclick={() => executeCommand(cmd)}
            onmouseenter={() => (selectedIndex = i)}
          >
            <div class="flex items-center gap-2">
              <span class="chip">{cmd.category}</span>
              <span>{cmd.label}</span>
            </div>
            {#if cmd.shortcut}
              <span class="text-xs" style="color: var(--text-muted);"
                >{cmd.shortcut}</span
              >
            {/if}
          </button>
        {/each}

        {#if filteredCommands.length === 0}
          <div
            class="px-4 py-6 text-center text-sm"
            style="color: var(--text-muted);"
          >
            没有找到匹配的命令
          </div>
        {/if}
      </div>
    </div>
  </div>
{/if}
