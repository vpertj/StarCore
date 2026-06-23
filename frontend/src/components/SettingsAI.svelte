<script>
  import {
    providers,
    setProviderConfig,
    builtinProviders,
    providerModels,
    customModels,
    saveCustomModels,
    modelEnabledMap,
    setModelEnabled,
    allAvailableModels,
  } from "../stores/provider.js";
  import { t } from "../stores/i18n.js";
  import {
    SetProviderConfig,
    TestProvider,
  } from "../../wailsjs/go/main/App.js";

  let showAddModelDialog = $state(false);
  let addModelProvider = $state("openai");
  let addModelProviderName = $state("");
  let addModelApiKey = $state("");
  let addModelApiKeyVisible = $state(false);
  let addModelEndpoint = $state("");
  let addModelId = $state("");
  let addModelName = $state("");
  let showEditProviderDialog = $state(false);
  let editProviderId = $state("");
  let editProviderName = $state("");
  let editProviderType = $state("openai");
  let editApiKey = $state("");
  let editEndpoint = $state("");
  let editTesting = $state(false);
  let editTestResult = $state(/** @type {{ok: boolean, msg: string}|null} */ (null));
  let showEditCustomModelDialog = $state(false);
  let editCustomModelOrigId = $state("");
  let editCustomModelId = $state("");
  let editCustomModelName = $state("");
  let editCustomModelProviderName = $state("");
  let editCustomModelApiKey = $state("");
  let editCustomModelApiKeyVisible = $state(false);
  let editCustomModelEndpoint = $state("");
  let editCustomModelProvider = $state("");
  let editFetchingModels = $state(false);
  let expandedProviderId = $state(/** @type {string|null} */ (null));

  let fetchModelError = $state("");
  let fetchingModels = $state(false);
  let fetchedModelList = $state(/** @type {string[]} */ ([]));
  let selectedModelsToAdd = $state(/** @type {string[]} */ ([]));

  // All available providers (builtin + custom) for the dropdown
  let allProviders = $derived([
    ...builtinProviders,
    ...$customModels
      .filter(m => m.groupId && !builtinProviders.some(bp => bp.id === m.groupId))
      .reduce((acc, m) => {
        if (!acc.find(p => p.id === m.groupId)) {
          acc.push({ id: m.groupId, name: m.providerName || m.groupId, defaultEndpoint: m.endpoint || '' });
        }
        return acc;
      }, /** @type {Array<{id:string,name:string,defaultEndpoint:string}>} */ ([]))
  ]);

  let testingModelId = $state(/** @type {string|null} */ (null));
  let testModelResults = $state(
    /** @type {Record<string, {ok: boolean, msg: string}>} */ ({}),
  );

  let editFetchedModelList = $state(/** @type {{id:string,name:string,contextWindow:number}[]} */ ([]));
  let editFetchError = $state("");
  let editSelectedModelsToAdd = $state(/** @type {string[]} */ ([]));
  let editManualModelId = $state("");
  let editManualModelName = $state("");
  let pendingEditModels = $state(/** @type {Array<any>} */ ([]));

  let cardIds = $derived(
    [...new Set($customModels.map(/** @param {any} m */ m => m.groupId || m.providerId || "unknown"))]
  );

  let editExistingModels = $derived(
    $customModels.filter((m) => (m.groupId || m.providerId) === editProviderId),
  );
  /** Combined existing (persisted) + pending (not yet saved) models for dialog display */
  let editAllModels = $derived(
    [...editExistingModels, ...pendingEditModels.map(m => ({ ...m, id: `${editProviderId}:${m.modelId}` }))]
  );
  let editExistingModelIds = $derived(
    new Set(editAllModels.map((m) => m.modelId || m.id.split(":").pop())),
  );
  let editNewModels = $derived(
    editFetchedModelList.filter((m) => !editExistingModelIds.has(m.id)),
  );

  function get(/** @type {any} */ store) {
    let value;
    store.subscribe(/** @type {(v: any) => void} */ ((v) => (value = v)))();
    return value;
  }

  /** Get context window for a model ID (from provider API data or estimate) */
  function getModelContextWindow(/** @type {string} */ modelId) {
    const found = $allAvailableModels.find(m => m.id === modelId || m.modelId === modelId)
    if (found?.contextWindow) return found.contextWindow
    // Fallback: estimate from model name
    const m = (modelId || '').toLowerCase()
    if (m.includes('gpt-4o') || m.includes('o1-') || m.includes('o3-')) return 200000
    if (m.includes('gpt-4')) return 128000
    if (m.includes('claude')) return 200000
    if (m.includes('deepseek-v4') || m.includes('deepseek-r1')) return 1048576
    if (m.includes('deepseek')) return 65536
    if (m.includes('gemini-2') || m.includes('gemma')) return 1048576
    if (m.includes('gemini')) return 32768
    if (m.includes('gpt-3.5')) return 16385
    return 128000
  }

  /** Format context window for display */
  function formatContextWindow(/** @type {number} */ w) {
    if (w >= 1048576) return Math.round(w / 1048576) + 'M'
    if (w >= 1000) return Math.round(w / 1000) + 'K'
    return String(w)
  }

  /** @param {string} providerId */
  function getProviderModelList(providerId) {
    return providerModels[providerId] || [];
  }

  function openAddModelDialog() {
    addModelProvider = "openai";
    addModelProviderName =
      builtinProviders.find((p) => p.id === "openai")?.name || "OpenAI";
    addModelApiKey = "";
    addModelApiKeyVisible = false;
    addModelEndpoint =
      builtinProviders.find((p) => p.id === "openai")?.defaultEndpoint || "";
    addModelId = "";
    addModelName = "";
    fetchedModelList = [];
    showAddModelDialog = true;
  }

  function onAddProviderChange() {
    const p = allProviders.find((bp) => bp.id === addModelProvider);
    addModelEndpoint = p?.defaultEndpoint || "";
    addModelProviderName = p?.name || addModelProvider;
    addModelId = "";
    addModelName = "";
  }

  /** @param {string} modelId */
  function toggleModelSelection(modelId) {
    const idx = selectedModelsToAdd.indexOf(modelId);
    if (idx >= 0) {
      selectedModelsToAdd = selectedModelsToAdd.filter((m) => m !== modelId);
    } else {
      selectedModelsToAdd = [...selectedModelsToAdd, modelId];
    }
  }

  function saveAllSelectedModels() {
    if (selectedModelsToAdd.length === 0) {
      showAddModelDialog = false;
      return;
    }
    const models = /** @type {Array<any>} */ ([...(get(customModels) || [])]);
    const providerName =
      addModelProviderName.trim() ||
      builtinProviders.find((p) => p.id === addModelProvider)?.name ||
      addModelProvider;
    for (const modelId of selectedModelsToAdd) {
      const compositeId = `${addModelProvider}:${modelId}`;
      if (!models.find(/** @param {any} m */ (m) => m.id === compositeId)) {
        models.push({
          id: compositeId,
          modelId,
          name: modelId,
          providerId: addModelProvider,
          groupId: addModelProvider,
          providerName,
          apiKey: addModelApiKey,
          endpoint: addModelEndpoint,
          enabled: true,
          isCustom: true,
        });
      }
    }
    saveCustomModels(models);
    if (addModelApiKey || addModelEndpoint) {
      setProviderConfig(addModelProvider, {
        apiKey: addModelApiKey,
        endpoint: addModelEndpoint,
      });
    }
    selectedModelsToAdd = [];
    showAddModelDialog = false;
  }

  function addCustomModel() {
    if (!addModelId.trim() || !addModelProvider) return;
    const models = /** @type {Array<any>} */ ([...(get(customModels) || [])]);
    const providerName =
      addModelProviderName.trim() ||
      builtinProviders.find((p) => p.id === addModelProvider)?.name ||
      addModelProvider;
    const compositeId = `${addModelProvider}:${addModelId.trim()}`;
    models.push({
      id: compositeId,
      modelId: addModelId.trim(),
      name: addModelName.trim() || addModelId.trim(),
      providerId: addModelProvider,
      groupId: addModelProvider,
      providerName,
      apiKey: addModelApiKey,
      endpoint: addModelEndpoint,
      enabled: true,
      isCustom: true,
    });
    saveCustomModels(models);
    if (addModelApiKey || addModelEndpoint) {
      setProviderConfig(addModelProvider, {
        apiKey: addModelApiKey,
        endpoint: addModelEndpoint,
      });
    }
    showAddModelDialog = false;
  }

  /** @param {string} modelId */
  function deleteCustomModel(modelId) {
    const models = /** @type {Array<any>} */ (
      (get(customModels) || []).filter(
        /** @param {any} m */ (m) => m.id !== modelId,
      )
    );
    saveCustomModels(models);
  }

  /** @param {string} modelId */
  function toggleCustomModel(modelId) {
    const models = /** @type {Array<any>} */ (get(customModels) || []);
    const model = models.find(/** @param {any} m */ (m) => m.id === modelId);
    if (model) {
      model.enabled = !model.enabled;
      saveCustomModels(models);
    }
  }

  /** @param {string} pid */
  function openEditProvider(pid) {
    const isNew = !pid;
    editProviderId = pid || ("new_" + Date.now().toString(36));
    // Look up by groupId first (custom providers), then by providerId (builtin)
    const existing = /** @type {Array<any>} */ (get(customModels) || []).find(
      (m) => (m.groupId || m.providerId) === pid,
    );
    const isBuiltin = builtinProviders.some(bp => bp.id === pid);
    // For new providers, start with empty fields
    if (isNew) {
      editProviderName = "";
      editProviderType = "openai";
      editApiKey = "";
      editEndpoint = "";
    } else {
      editProviderName = existing?.providerName || (isBuiltin ? builtinProviders.find((p) => p.id === pid)?.name : "") || pid || "";
      editProviderType = existing?.providerId === "anthropic" ? "anthropic" : existing?.providerId === "ollama" ? "ollama" : "openai";
      editApiKey = existing?.apiKey || "";
      editEndpoint = existing?.endpoint || (isBuiltin ? builtinProviders.find((p) => p.id === pid)?.defaultEndpoint : "") || "";
    }
    editFetchedModelList = [];
    editSelectedModelsToAdd = [];
    editManualModelId = "";
    editManualModelName = "";
    pendingEditModels = [];
    editFetchError = "";
    editTestResult = null;
    showEditProviderDialog = true;
  }

  /** @param {string} modelId */
  function editToggleModelSelection(modelId) {
    const idx = editSelectedModelsToAdd.indexOf(modelId);
    if (idx >= 0) {
      editSelectedModelsToAdd = editSelectedModelsToAdd.filter(
        (m) => m !== modelId,
      );
    } else {
      editSelectedModelsToAdd = [...editSelectedModelsToAdd, modelId];
    }
  }

  async function editFetchModelsFromAPI() {
    if (!editApiKey && !editEndpoint) return;
    editFetchingModels = true;
    editFetchedModelList = [];
    editSelectedModelsToAdd = [];
    editFetchError = "";
    try {
      await SetProviderConfig(editProviderId, {
        id: editProviderId,
        name: editProviderId,
        apiKey: editApiKey,
        endpoint: editEndpoint,
        enabled: true,
        isDefault: false,
      });
      const models = await (window.backend?.GetModels
        ? window.backend.GetModels(editProviderId)
        : Promise.resolve([]));
      editFetchedModelList = (models || []).map(/** @param {any} m */ m => ({ id: m.id || m.name, name: m.name || m.id, contextWindow: m.contextWindow || 0 })).filter(/** @param {{id:string}} m */ m => !!m.id);
      if (editFetchedModelList.length === 0) editFetchError = "未获取到可用模型";
    } catch (/** @type {any} */ e) {
      editFetchError = e.message || String(e);
      editFetchedModelList = [];
    } finally {
      editFetchingModels = false;
    }
  }

  async function editTestConnection() {
    editTesting = true;
    editTestResult = null;
    try {
      await SetProviderConfig(editProviderId, {
        id: editProviderId,
        name: editProviderName || editProviderId,
        apiKey: editApiKey,
        endpoint: editEndpoint,
        enabled: true,
        isDefault: false,
      });
      await TestProvider(editProviderId);
      editTestResult = { ok: true, msg: "连接正常" };
    } catch (/** @type {any} */ e) {
      editTestResult = { ok: false, msg: e.message || String(e) };
    }
    editTesting = false;
  }

  function editAddManualModel() {
    if (!editManualModelId.trim()) return;
    const modelId = editManualModelId.trim();
    // Check against already-persisted models (by editProviderId) and pending models
    const existing = (get(customModels) || []).filter(m => (m.groupId || m.providerId) === editProviderId);
    if (existing.find(m => m.modelId === modelId || m.id.endsWith(":" + modelId))) return;
    if (pendingEditModels.find(m => m.modelId === modelId)) return;
    pendingEditModels = [...pendingEditModels, {
      modelId,
      name: editManualModelName.trim() || modelId,
      providerId: editProviderId,
      groupId: editProviderId,
      providerName: editProviderName || editProviderId,
      apiKey: editApiKey,
      endpoint: editEndpoint,
      enabled: true,
      isCustom: true,
    }];
    editManualModelId = "";
    editManualModelName = "";
  }

  function editAddSelectedModels() {
    if (editSelectedModelsToAdd.length === 0) return;
    const existing = (get(customModels) || []).filter(m => (m.groupId || m.providerId) === editProviderId);
    const providerName =
      builtinProviders.find((p) => p.id === editProviderId)?.name ||
      editProviderId;
    const toAdd = [];
    for (const modelId of editSelectedModelsToAdd) {
      const compositeId = `${editProviderId}:${modelId}`;
      if (!existing.find(m => m.id === compositeId) && !pendingEditModels.find(m => m.modelId === modelId)) {
        const apiModel = editFetchedModelList.find(m => m.id === modelId)
        toAdd.push({
          modelId,
          name: (apiModel?.name) || modelId,
          providerId: editProviderId,
          groupId: editProviderId,
          providerName,
          apiKey: editApiKey,
          endpoint: editEndpoint,
          contextWindow: apiModel?.contextWindow || 0,
          enabled: true,
          isCustom: true,
        });
      }
    }
    if (toAdd.length > 0) pendingEditModels = [...pendingEditModels, ...toAdd];
    editSelectedModelsToAdd = [];
    editFetchedModelList = [];
  }

  /** @param {any} cm */
  function openEditCustomModel(cm) {
    editCustomModelOrigId = cm.id;
    editCustomModelId = cm.modelId || cm.id;
    editCustomModelName = cm.name;
    editCustomModelProviderName = cm.providerName || "";
    editCustomModelApiKey = cm.apiKey || "";
    editCustomModelApiKeyVisible = false;
    editCustomModelEndpoint = cm.endpoint || "";
    editCustomModelProvider = cm.providerId;
    showEditCustomModelDialog = true;
  }

  function saveEditCustomModel() {
    const currentCustomModels = /** @type {Array<any>} */ (
      [...(get(customModels) || [])]
    );
    const idx = currentCustomModels.findIndex(
      (m) => m.id === editCustomModelOrigId,
    );
    if (idx === -1) return;
    const newId =
      editCustomModelId.trim() ||
      currentCustomModels[idx].modelId ||
      currentCustomModels[idx].id;
    const compositeId = `${editCustomModelProvider}:${newId}`;
    currentCustomModels[idx] = {
      ...currentCustomModels[idx],
      id: compositeId,
      modelId: newId,
      name: editCustomModelName.trim() || newId,
      providerName:
        editCustomModelProviderName.trim() ||
        currentCustomModels[idx].providerName,
      providerId: editCustomModelProvider,
      apiKey: editCustomModelApiKey,
      endpoint: editCustomModelEndpoint,
    };
    saveCustomModels(currentCustomModels);
    showEditCustomModelDialog = false;
  }

  async function fetchModelsFromAPI() {
    if (!addModelApiKey && !addModelEndpoint) return;
    fetchingModels = true;
    fetchedModelList = [];
    fetchModelError = "";
    try {
      // Route through Go backend to avoid CORS issues
      await SetProviderConfig(addModelProvider, {
        id: addModelProvider,
        name: addModelProviderName || addModelProvider,
        apiKey: addModelApiKey,
        endpoint: addModelEndpoint,
        enabled: true,
        isDefault: false,
      });
      const models = await (window.backend?.GetModels
        ? window.backend.GetModels(addModelProvider)
        : Promise.resolve([]));
      fetchedModelList = (models || []).map(/** @param {any} m */ m => m.id || m.name).filter(Boolean);
      if (fetchedModelList.length === 0) fetchModelError = "未获取到可用模型（请检查 API Key 和端点地址）";
    } catch (/** @type {any} */ e) {
      const errMsg = (e.message || String(e)).replace(/sk-[a-zA-Z0-9]{8,}/g, "sk-****");
      if (errMsg.includes("401") || errMsg.includes("403") || errMsg.includes("auth"))
        fetchModelError = "认证失败：API 密钥无效";
      else if (errMsg.includes("timeout") || errMsg.includes("deadline"))
        fetchModelError = "请求超时";
      else
        fetchModelError = `获取失败：${errMsg}`;
    } finally {
      fetchingModels = false;
    }
  }

  /** @param {string} modelId @param {string} providerId @param {string} endpoint @param {string} apiKey */
  async function testModelConnection(modelId, providerId, endpoint, apiKey) {
    testingModelId = modelId;
    try {
      if (apiKey || endpoint) {
        await SetProviderConfig(providerId, {
          id: providerId,
          name: providerId,
          apiKey: apiKey || "",
          endpoint: endpoint || "",
          enabled: true,
          isDefault: false,
        });
      }
      await TestProvider(providerId);
      testModelResults[modelId] = { ok: true, msg: "连接正常" };
    } catch (/** @type {any} */ e) {
      testModelResults[modelId] = { ok: false, msg: e.message || String(e) };
    }
    testModelResults = testModelResults;
    testingModelId = null;
  }
</script>

<div class="space-y-4">
  <p class="text-xs" style="color: var(--text-secondary);">
    {$t("settings.ai.configHint") || "配置 API Key 添加更多可用模型"}
  </p>

  <!-- Migration hint: fix models grouped under wrong provider -->
  {#if $customModels.length > 0 && (() => {
    // Check if any models have providerId that doesn't match their groupId
    const misgrouped = $customModels.filter(m => m.groupId && m.providerId !== m.groupId && !builtinProviders.some(bp => bp.id === m.providerId));
    return misgrouped.length > 0;
  })()}
    <div class="p-3 rounded-lg text-xs flex items-center gap-2" style="background-color: color-mix(in srgb, var(--warning) 10%, transparent); border: 1px solid var(--warning);">
      <svg viewBox="0 0 24 24" class="w-4 h-4 shrink-0" fill="none" stroke="currentColor" style="color: var(--warning);"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-2.5L13.732 4c-.77-.833-1.964-.833-2.732 0L4.082 16.5c-.77.833.192 2.5 1.732 2.5z"/></svg>
      <span style="color: var(--text-secondary);">发现模型分组不正确，点击</span>
      <button class="underline font-medium" style="color: var(--warning);" onclick={() => {
        const models = [...$customModels];
        // Move models from wrong providerId to correct groupId
        for (const m of models) {
          if (m.groupId && m.providerId !== m.groupId && !builtinProviders.some(bp => bp.id === m.providerId)) {
            m.providerId = m.groupId;
          }
        }
        saveCustomModels(models);
      }}>一键修复</button>
    </div>
  {/if}

  <div class="rounded-lg border overflow-hidden" style="border-color: var(--border); background-color: var(--bg-secondary);">
    {#each cardIds as pid}
      {@const providerModels = $customModels.filter(/** @param {any} m */ m => (m.groupId || m.providerId) === pid)}
      {@const customName = providerModels[0]?.providerName}
      {@const bp = builtinProviders.find(p => p.id === pid) || { id: pid, name: pid }}
      {@const displayName = customName || bp.name || pid}
      {@const providerModelList = providerModels}
      {@const bpCount = providerModelList.length}
      {@const isExpanded = expandedProviderId === pid}
      {#if bpCount > 0}
        <div class="border-b" style="border-color: var(--border);">
          <!-- Provider row header -->
          <!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
          <div
            class="flex items-center gap-2 px-4 py-2.5 cursor-pointer transition-colors hover:bg-white/5"
            onclick={() => expandedProviderId = isExpanded ? null : pid}
          >
            <svg xmlns="http://www.w3.org/2000/svg" class="w-3 h-3 shrink-0 transition-transform" style="color: var(--text-muted); transform: {isExpanded ? 'rotate(90deg)' : 'rotate(0)'};" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" /></svg>
            <span class="text-sm font-medium flex-1 truncate" style="color: var(--text-primary);">{displayName}</span>
            <span class="text-[11px] font-medium" style="color: var(--text-muted);">{bpCount} 模型</span>
            <button
              class="p-1 rounded hover:bg-white/10 shrink-0"
              style="color: var(--text-muted);"
              onclick={(e) => { e.stopPropagation(); openEditProvider(pid) }}
              title="编辑 {displayName}"
            >
              <svg xmlns="http://www.w3.org/2000/svg" class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15.232 5.232l3.536 3.536m-2.036-5.036a2.5 2.5 0 113.536 3.536L6.5 21.036H3v-3.572L16.732 3.732z" /></svg>
            </button>
          </div>
          <!-- Expanded model list -->
          {#if isExpanded}
            <div style="background-color: var(--bg-primary);">
              {#each providerModelList as cm}
                <div class="flex items-center gap-3 px-4 py-1.5 text-xs border-t" style="border-color: var(--border); color: var(--text-primary);">
                  <span class="flex-1 truncate">{cm.name || cm.modelId}</span>
                  <span class="shrink-0 text-[10px] font-medium px-1.5 py-0.5 rounded" style="background-color: var(--bg-secondary); color: var(--text-muted);">{formatContextWindow(getModelContextWindow(cm.modelId || cm.id))}</span>
                  <button class="p-0.5 rounded hover:bg-white/10 shrink-0" style="color: #f14c4c;" onclick={() => deleteCustomModel(cm.id)} title="删除">
                    <svg xmlns="http://www.w3.org/2000/svg" class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" /></svg>
                  </button>
                </div>
              {/each}
            </div>
          {/if}
        </div>
      {/if}
    {/each}
  </div>

  <div class="flex justify-end pt-2">
    <button class="px-4 py-2 rounded text-sm font-medium transition-colors" style="background-color: var(--accent); color: #ffffff;" onclick={() => openEditProvider("")}>
      + 添加供应商
    </button>
  </div>
</div>

{#if showAddModelDialog}
  <div
    class="fixed inset-0 z-60 flex items-center justify-center"
    style="background-color: rgba(0, 0, 0, 0.5);"
    onkeydown={(e) => {
      if (e.key === "Escape") showAddModelDialog = false;
    }}
  >
    <div
      class="rounded-lg shadow-xl p-6 flex flex-col"
      style="width: 480px; max-height: 85vh; background-color: var(--bg-primary); border: 1px solid var(--border);"
    >
      <div class="flex items-center justify-between mb-4">
        <h3 class="text-base font-medium" style="color: var(--text-primary);">
          {$t("settings.ai.addModel") || "添加模型"}
        </h3>
        <button
          class="p-1 rounded hover:bg-white/10"
          style="color: var(--text-secondary);"
          onclick={() => (showAddModelDialog = false)}
          aria-label="关闭"
        >
          <svg
            xmlns="http://www.w3.org/2000/svg"
            class="w-4 h-4"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
            ><path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M6 18L18 6M6 6l12 12"
            /></svg
          >
        </button>
      </div>
      <div class="space-y-4 overflow-y-auto flex-1" style="min-height: 0;">
        <div class="flex gap-3">
          <div class="flex-1">
            <label for="add-model-provider" class="block text-xs mb-1.5 font-medium" style="color: var(--text-secondary);">{$t("settings.ai.provider") || "服务商"}</label>
            <select id="add-model-provider" bind:value={addModelProvider} class="w-full px-3 py-2 rounded border text-sm" style="background-color: var(--bg-secondary); color: var(--text-primary); border-color: var(--border);" onchange={onAddProviderChange}>
              {#each allProviders as bp}
                <option value={bp.id}>{bp.name}</option>
              {/each}
            </select>
          </div>
          <div class="flex-1">
            <label for="add-model-provider-name" class="block text-xs mb-1.5 font-medium" style="color: var(--text-secondary);">供应商名称</label>
            <input id="add-model-provider-name" type="text" bind:value={addModelProviderName} placeholder="OpenAI" class="w-full px-3 py-2 rounded border text-sm" style="background-color: var(--bg-secondary); color: var(--text-primary); border-color: var(--border);" />
          </div>
        </div>
        <div>
          <label
            for="add-model-apikey"
            class="block text-xs mb-1.5 font-medium"
            style="color: var(--text-secondary);"
            >{$t("settings.ai.apiKey")}</label
          >
          <div class="relative">
            <input
              id="add-model-apikey"
              type={addModelApiKeyVisible ? "text" : "password"}
              bind:value={addModelApiKey}
              placeholder="sk-..."
              class="w-full px-3 py-2 pr-9 rounded border text-sm"
              style="background-color: var(--bg-secondary); color: var(--text-primary); border-color: var(--border);"
            />
            <button
              class="absolute right-2 top-1/2 -translate-y-1/2 p-1 rounded transition-colors hover:bg-white/10"
              style="color: var(--text-secondary);"
              onclick={() => (addModelApiKeyVisible = !addModelApiKeyVisible)}
            >
              {#if addModelApiKeyVisible}
                <svg
                  xmlns="http://www.w3.org/2000/svg"
                  class="w-4 h-4"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                  ><path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M13.875 18.825A10.05 10.05 0 0112 19c-4.478 0-8.268-2.943-9.543-7a9.97 9.97 0 011.563-3.029m5.858.908a3 3 0 114.243 4.243M9.878 9.878l4.242 4.242M9.88 9.88l-3.29-3.29m7.532 7.532l3.29 3.29M3 3l3.59 3.59m0 0A9.953 9.953 0 0112 5c4.478 0 8.268 2.943 9.543 7a10.025 10.025 0 01-4.132 5.411m0 0L21 21"
                  /></svg
                >
              {:else}
                <svg
                  xmlns="http://www.w3.org/2000/svg"
                  class="w-4 h-4"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                  ><path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"
                  /><path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z"
                  /></svg
                >
              {/if}
            </button>
          </div>
        </div>
        <div>
          <label
            for="add-model-endpoint"
            class="block text-xs mb-1.5 font-medium"
            style="color: var(--text-secondary);"
            >{$t("settings.ai.endpoint")}</label
          >
          <input
            id="add-model-endpoint"
            type="text"
            bind:value={addModelEndpoint}
            placeholder="https://api.example.com/v1/chat/completions"
            class="w-full px-3 py-2 rounded border text-sm"
            style="background-color: var(--bg-secondary); color: var(--text-primary); border-color: var(--border);"
          />
        </div>
        <div class="flex gap-2">
          <button
            class="flex-1 px-3 py-1.5 rounded text-xs font-medium transition-colors"
            style="background-color: #094771; color: #ffffff;"
            onclick={fetchModelsFromAPI}
            disabled={fetchingModels || (!addModelApiKey && !addModelEndpoint)}
          >
            {#if fetchingModels}{$t("settings.ai.fetching") ||
                "获取中..."}{:else}{$t("settings.ai.fetchModels") ||
                "获取模型列表"}{/if}
          </button>
        </div>
        {#if fetchModelError}<p class="text-xs mt-1" style="color: #f14c4c;">
            {fetchModelError}
          </p>{/if}
        {#if fetchedModelList.length > 0}
          <div>
            <!-- svelte-ignore a11y_label_has_associated_control -->
            <label
              class="block text-xs mb-1.5 font-medium"
              style="color: var(--text-secondary);"
              >可用模型 ({fetchedModelList.length})</label
            >
            <div
              class="rounded border overflow-y-auto"
              style="background-color: var(--bg-secondary); border-color: var(--border); max-height: 180px;"
            >
              {#each fetchedModelList as fm}
                <!-- svelte-ignore a11y_click_events_have_key_events -->
                <div
                  class="w-full flex items-center gap-2 px-3 py-1.5 text-xs transition-colors cursor-pointer"
                  style="background-color: {selectedModelsToAdd.includes(fm)
                    ? '#094771'
                    : 'transparent'}; color: {selectedModelsToAdd.includes(fm)
                    ? '#ffffff'
                    : 'var(--text-primary)'}; border-bottom: 1px solid var(--border);"
                  onclick={() => toggleModelSelection(fm)}
                  role="checkbox"
                  aria-checked={selectedModelsToAdd.includes(fm)}
                  tabindex="0"
                  onkeydown={(e) => {
                    if (e.key === "Enter" || e.key === " ") {
                      e.preventDefault();
                      toggleModelSelection(fm);
                    }
                  }}
                >
                  <span
                    class="`shrink-0"
                    style="width: 16px; text-align: center;"
                    >{selectedModelsToAdd.includes(fm) ? "✓" : ""}</span
                  >
                  <span class="truncate">{fm}</span>
                </div>
              {/each}
            </div>
            {#if selectedModelsToAdd.length > 0}
              <div
                class="mt-2 p-2 rounded text-xs"
                style="background-color: #09477120; border: 1px solid var(--accent); color: var(--text-primary);"
              >
                已选: {selectedModelsToAdd.join(", ")}
              </div>
            {/if}
          </div>
        {:else if getProviderModelList(addModelProvider).length > 0}
          <div>
            <label
              for="add-model-select"
              class="block text-xs mb-1.5 font-medium"
              style="color: var(--text-secondary);"
              >{$t("settings.ai.selectModel") || "选择模型"}</label
            >
            <select
              id="add-model-select"
              bind:value={addModelId}
              class="w-full px-3 py-2 rounded border text-sm"
              style="background-color: var(--bg-secondary); color: var(--text-primary); border-color: var(--border);"
              onchange={() => {
                const m = getProviderModelList(addModelProvider).find(
                  (pm) => pm.id === addModelId,
                );
                if (m) addModelName = m.name;
              }}
            >
              <option value=""
                >-- {$t("settings.ai.selectModel") || "选择模型"} --</option
              >
              {#each getProviderModelList(addModelProvider) as pm}
                <option value={pm.id}>{pm.name}</option>
              {/each}
            </select>
          </div>
        {:else}
          <div>
            <label
              for="add-model-id"
              class="block text-xs mb-1.5 font-medium"
              style="color: var(--text-secondary);"
              >{$t("settings.ai.modelId") || "模型 ID"}</label
            >
            <input
              id="add-model-id"
              type="text"
              bind:value={addModelId}
              placeholder="model-name"
              class="w-full px-3 py-2 rounded border text-sm"
              style="background-color: var(--bg-secondary); color: var(--text-primary); border-color: var(--border);"
            />
          </div>
          <div>
            <label
              for="add-model-name"
              class="block text-xs mb-1.5 font-medium"
              style="color: var(--text-secondary);"
              >{$t("settings.ai.modelName") || "模型名称"}</label
            >
            <input
              id="add-model-name"
              type="text"
              bind:value={addModelName}
              placeholder="My Model"
              class="w-full px-3 py-2 rounded border text-sm"
              style="background-color: var(--bg-secondary); color: var(--text-primary); border-color: var(--border);"
            />
          </div>
        {/if}
      </div>
      <div class="flex justify-end gap-3 pt-3 border-t shrink-0" style="border-color: var(--border);">
        <button
          class="px-4 py-2 rounded text-sm"
          style="background-color: var(--border); color: var(--text-primary);"
          onclick={() => (showAddModelDialog = false)}
          >{$t("settings.cancel")}</button
        >
        <button
          class="px-4 py-2 rounded text-sm font-medium"
          style="background-color: var(--accent); color: #ffffff;"
          onclick={saveAllSelectedModels}
          disabled={selectedModelsToAdd.length === 0}
          >{$t("settings.ai.addModel") || "添加模型"}</button
        >
      </div>
    </div>
  </div>
{/if}

{#if showEditProviderDialog}
  <div
    class="fixed inset-0 z-60 flex items-center justify-center"
    style="background-color: rgba(0, 0, 0, 0.5);"
    role="button"
    tabindex="-1"
    onkeydown={(e) => {
      if (e.key === "Escape") showEditProviderDialog = false;
    }}
  >
    <div
      class="rounded-lg shadow-xl p-6 overflow-y-auto"
      style="width: 480px; max-height: 85vh; background-color: var(--bg-primary); border: 1px solid var(--border);"
    >
      <div class="flex items-center justify-between mb-4">
        <h3 class="text-base font-medium" style="color: var(--text-primary);">
          {editProviderName || builtinProviders.find((p) => p.id === editProviderId)?.name ||
            (editProviderId.startsWith("new_") ? "新建模型供应商" : editProviderId)}
        </h3>
        <button
          class="p-1 rounded hover:bg-white/10"
          style="color: var(--text-secondary);"
          onclick={() => { pendingEditModels = []; showEditProviderDialog = false; }}
          aria-label="关闭"
        >
          <svg
            xmlns="http://www.w3.org/2000/svg"
            class="w-4 h-4"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
            ><path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M6 18L18 6M6 6l12 12"
            /></svg
          >
        </button>
      </div>
      <div class="space-y-4">
        <div class="flex gap-3">
          <div class="flex-1">
            <label for="edit-provider-name" class="block text-xs mb-1.5 font-medium" style="color: var(--text-secondary);">供应商名称</label>
            <input id="edit-provider-name" type="text" bind:value={editProviderName} class="w-full px-3 py-2 rounded border text-sm" style="background-color: var(--bg-secondary); color: var(--text-primary); border-color: var(--border);" />
          </div>
          <div class="flex-1">
            <label class="block text-xs mb-1.5 font-medium" style="color: var(--text-secondary);">接口协议</label>
            <select bind:value={editProviderType} class="w-full px-3 py-2 rounded border text-sm" style="background-color: var(--bg-secondary); color: var(--text-primary); border-color: var(--border);">
              <option value="openai">OpenAI 兼容</option>
              <option value="anthropic">Anthropic</option>
              <option value="ollama">Ollama</option>
            </select>
          </div>
        </div>
        <div>
          <label
            for="edit-apikey"
            class="block text-xs mb-1.5 font-medium"
            style="color: var(--text-secondary);"
            >{$t("settings.ai.apiKey")}</label
          >
          <input
            id="edit-apikey"
            type="password"
            bind:value={editApiKey}
            placeholder="sk-..."
            class="w-full px-3 py-2 rounded border text-sm"
            style="background-color: var(--bg-secondary); color: var(--text-primary); border-color: var(--border);"
          />
        </div>
        <div>
          <label
            for="edit-endpoint"
            class="block text-xs mb-1.5 font-medium"
            style="color: var(--text-secondary);"
            >{$t("settings.ai.endpoint")}</label
          >
          <input
            id="edit-endpoint"
            type="text"
            bind:value={editEndpoint}
            class="w-full px-3 py-2 rounded border text-sm"
            style="background-color: var(--bg-secondary); color: var(--text-primary); border-color: var(--border);"
          />
        </div>
        {#if editAllModels.length > 0}
          <div>
            <!-- svelte-ignore a11y_label_has_associated_control -->
            <label
              class="block text-xs mb-1.5 font-medium"
              style="color: var(--text-secondary);"
              >已添加 ({editAllModels.length})</label
            >
            <div
              class="rounded border overflow-y-auto"
              style="background-color: var(--bg-secondary); border-color: var(--border); max-height: 120px;"
            >
              {#each editAllModels as em (em.id || em.modelId)}
                <div
                  class="flex items-center justify-between px-3 py-1.5 text-xs border-b"
                  style="color: var(--text-primary); border-color: var(--border);"
                >
                  <span class="truncate">{em.name}</span>
                  <button
                    class="p-1 rounded hover:bg-white/10 shrink-0"
                    style="color: #f14c4c;"
                    onclick={() => {
                      if (pendingEditModels.some(m => m.modelId === em.modelId)) {
                        // Pending model — remove from pendingEditModels
                        pendingEditModels = pendingEditModels.filter(m => m.modelId !== em.modelId);
                      } else {
                        deleteCustomModel(em.id);
                      }
                    }}
                    title="Remove"
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
                        d="M6 18L18 6M6 6l12 12"
                      /></svg
                    >
                  </button>
                </div>
              {/each}
            </div>
          </div>
        {/if}
        <div class="flex gap-2 items-end">
          <div class="flex-1">
            <label class="block text-xs mb-1.5 font-medium" style="color: var(--text-secondary);">模型 ID</label>
            <input type="text" bind:value={editManualModelId} placeholder="model-name" class="w-full px-3 py-2 rounded border text-sm" style="background-color: var(--bg-secondary); color: var(--text-primary); border-color: var(--border);" />
          </div>
          <div class="flex-1">
            <label class="block text-xs mb-1.5 font-medium" style="color: var(--text-secondary);">模型名称</label>
            <input type="text" bind:value={editManualModelName} placeholder="可选" class="w-full px-3 py-2 rounded border text-sm" style="background-color: var(--bg-secondary); color: var(--text-primary); border-color: var(--border);" />
          </div>
          <button class="px-3 py-2 rounded text-xs font-medium shrink-0" style="background-color: #2ea043; color: #fff;" onclick={editAddManualModel}>添加</button>
        </div>
        <div class="flex gap-2">
          <button
            class="flex-1 px-3 py-1.5 rounded text-xs font-medium transition-colors"
            style="background-color: #094771; color: #ffffff;"
            onclick={editFetchModelsFromAPI}
            disabled={editFetchingModels || (!editApiKey && !editEndpoint)}
          >
            {#if editFetchingModels}{$t("settings.ai.fetching") ||
                "获取中..."}{:else}{$t("settings.ai.fetchModels") ||
                "获取模型列表"}{/if}
          </button>
        </div>
        {#if editFetchError}<p class="text-xs" style="color: #f14c4c;">
            {editFetchError}
          </p>{/if}
        {#if editFetchedModelList.length > 0 && editNewModels.length > 0}
          <div>
            <!-- svelte-ignore a11y_label_has_associated_control -->
            <label
              class="block text-xs mb-1.5 font-medium"
              style="color: var(--text-secondary);"
              >可供添加 ({editNewModels.length})</label
            >
            <div
              class="rounded border overflow-y-auto"
              style="background-color: var(--bg-secondary); border-color: var(--border); max-height: 150px;"
            >
              {#each editNewModels as fm}
                {@const checked = editSelectedModelsToAdd.includes(fm.id)}
                <div
                  class="flex items-center gap-2 px-3 py-1.5 text-xs transition-colors cursor-pointer border-b"
                  style="background-color: {checked ? '#094771' : 'transparent'}; color: {checked ? '#ffffff' : 'var(--text-primary)'}; border-color: var(--border);"
                  onclick={() => editToggleModelSelection(fm.id)}
                  role="checkbox"
                  aria-checked={checked}
                  tabindex="0"
                  onkeydown={(e) => {
                    if (e.key === "Enter" || e.key === " ") {
                      e.preventDefault();
                      editToggleModelSelection(fm.id);
                    }
                  }}
                >
                  <span style="width: 16px; text-align: center;">{checked ? "✓" : ""}</span>
                  <span class="truncate">{fm.name || fm.id}</span>
                </div>
              {/each}
            </div>
            {#if editSelectedModelsToAdd.length > 0}
              <button
                class="mt-2 w-full px-3 py-1.5 rounded text-xs font-medium"
                style="background-color: #2ea043; color: #ffffff;"
                onclick={editAddSelectedModels}
                >添加 {editSelectedModelsToAdd.length} 个模型</button
              >
            {/if}
          </div>
        {:else if editFetchedModelList.length > 0}
          <p class="text-xs" style="color: var(--text-muted);">
            所有可用模型已添加。
          </p>
        {/if}
        <div
          class="flex justify-end gap-3 pt-2 border-t"
          style="border-color: var(--border);"
        >
          {#if editTestResult}
            <span class="text-xs flex-1" style="color: {editTestResult.ok ? '#2ea043' : '#f14c4c'};">
              {editTestResult.ok ? '✓' : '✗'} {editTestResult.msg}
            </span>
          {/if}
          {#if editProviderId.startsWith("custom_")}
            <button class="px-3 py-1.5 rounded text-xs font-medium" style="background-color: #f14c4c; color: #fff;" onclick={() => {
              if (confirm('确定要删除此供应商及其所有模型吗？')) {
                const models = (get(customModels) || []).filter(m => (m.groupId || m.providerId) !== editProviderId);
                saveCustomModels(models);
                showEditProviderDialog = false;
              }
            }}>删除供应商</button>
          {/if}
          <button
            class="px-3 py-1.5 rounded text-xs font-medium transition-colors"
            style="background-color: #094771; color: #ffffff;"
            onclick={editTestConnection}
            disabled={editTesting}
          >
            {editTesting ? "测试中..." : "测试连接"}
          </button>
          <button
            class="px-4 py-2 rounded text-sm"
            style="background-color: var(--border); color: var(--text-primary);"
            onclick={() => {
              pendingEditModels = [];
              showEditProviderDialog = false;
            }}
            >{$t("settings.cancel")}</button
          >
          <button
            class="px-4 py-2 rounded text-sm font-medium"
            style="background-color: var(--accent); color: #ffffff;"
            disabled={editAllModels.length === 0 && !editProviderName && !editApiKey}
            onclick={async () => {
              if (!editProviderName && editAllModels.length === 0) { alert('请先填写供应商名称并添加至少一个模型'); return; }
              const isBuiltin = builtinProviders.some(bp => bp.id === editProviderId);
              const groupId = isBuiltin ? editProviderId : editProviderId;
              const backendPid = editProviderType === "anthropic" ? "anthropic" : editProviderType === "ollama" ? "ollama" : "openai";

              // Persist provider config to Go backend using groupId (not backendPid)
              // so each custom provider has its own config entry
              try {
                await setProviderConfig(groupId, {
                  name: editProviderName || editProviderId,
                  apiKey: editApiKey,
                  endpoint: editEndpoint,
                  enabled: true,
                });
              } catch (e) {
                console.error('Failed to save provider config:', e);
                alert('保存供应商配置失败: ' + (e.message || e));
                return;
              }

              // Build final model list: keep models NOT for this provider,
              // then add updated existing models + pending models with final groupId/providerId
              const allModels = [...(get(customModels) || [])];
              const otherModels = allModels.filter(m => (m.groupId || m.providerId) !== editProviderId);
              const providerModels = [];

              // Update existing models for this provider
              for (const m of allModels) {
                if ((m.groupId || m.providerId) === editProviderId) {
                  providerModels.push({
                    ...m,
                    id: `${groupId}:${m.modelId || m.id.split(':').pop()}`,
                    modelId: m.modelId || m.id.split(':').pop(),
                    providerId: backendPid,
                    groupId: groupId,
                    providerName: editProviderName || m.providerName,
                    apiKey: editApiKey || m.apiKey,
                    endpoint: editEndpoint || m.endpoint,
                  });
                }
              }

              // Add pending models with final IDs
              for (const pm of pendingEditModels) {
                const finalId = `${groupId}:${pm.modelId}`;
                if (!providerModels.find(m => m.id === finalId)) {
                  providerModels.push({
                    id: finalId,
                    modelId: pm.modelId,
                    name: pm.name,
                    providerId: backendPid,
                    groupId: groupId,
                    providerName: editProviderName || pm.providerName,
                    apiKey: editApiKey || pm.apiKey,
                    endpoint: editEndpoint || pm.endpoint,
                    contextWindow: pm.contextWindow || 0,
                    enabled: true,
                    isCustom: true,
                  });
                }
              }

              // If nothing added but user filled provider name and this is a new provider,
              // create a placeholder model so the card appears.
              if (providerModels.length === 0 && !isBuiltin && editProviderName) {
                const compositeId = `${groupId}:${editProviderName || 'default'}`;
                providerModels.push({
                  id: compositeId,
                  modelId: editProviderName || 'default',
                  name: editProviderName || 'default',
                  providerId: backendPid,
                  groupId: groupId,
                  providerName: editProviderName,
                  apiKey: editApiKey || '',
                  endpoint: editEndpoint || '',
                  enabled: true,
                  isCustom: true,
                });
              }

              // Single save with the final combined list
              saveCustomModels([...otherModels, ...providerModels]);
              pendingEditModels = [];
              showEditProviderDialog = false;
            }}>{$t("settings.save")}</button
          >
        </div>
      </div>
    </div>
  </div>
{/if}

{#if showEditCustomModelDialog}
  <div
    class="fixed inset-0 z-60 flex items-center justify-center"
    style="background-color: rgba(0, 0, 0, 0.5);"
    role="button"
    tabindex="-1"
    onkeydown={(e) => {
      if (e.key === "Escape") showEditCustomModelDialog = false;
    }}
  >
    <div
      class="rounded-lg shadow-xl p-6"
      style="width: 420px; background-color: var(--bg-primary); border: 1px solid var(--border);"
    >
      <div class="flex items-center justify-between mb-4">
        <h3 class="text-base font-medium" style="color: var(--text-primary);">
          {$t("settings.ai.editModel") || "编辑模型"}
        </h3>
        <button
          class="p-1 rounded hover:bg-white/10"
          style="color: var(--text-secondary);"
          onclick={() => (showEditCustomModelDialog = false)}
          aria-label="关闭"
        >
          <svg
            xmlns="http://www.w3.org/2000/svg"
            class="w-4 h-4"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
            ><path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M6 18L18 6M6 6l12 12"
            /></svg
          >
        </button>
      </div>
      <div class="space-y-4">
        <div class="flex gap-3">
          <div class="flex-1">
            <label for="edit-cm-provider" class="block text-xs mb-1.5 font-medium" style="color: var(--text-secondary);">{$t("settings.ai.provider") || "服务商"}</label>
            <select id="edit-cm-provider" bind:value={editCustomModelProvider} class="w-full px-3 py-2 rounded border text-sm" style="background-color: var(--bg-secondary); color: var(--text-primary); border-color: var(--border);">
              {#each builtinProviders as bp}<option value={bp.id}>{bp.name}</option>{/each}
            </select>
          </div>
          <div class="flex-1">
            <label for="edit-cm-provider-name" class="block text-xs mb-1.5 font-medium" style="color: var(--text-secondary);">供应商名称</label>
            <input id="edit-cm-provider-name" type="text" bind:value={editCustomModelProviderName} placeholder="OpenAI" class="w-full px-3 py-2 rounded border text-sm" style="background-color: var(--bg-secondary); color: var(--text-primary); border-color: var(--border);" />
          </div>
        </div>
        <div class="flex gap-3">
          <div class="flex-1">
            <label for="edit-cm-id" class="block text-xs mb-1.5 font-medium" style="color: var(--text-secondary);">{$t("settings.ai.modelId") || "模型 ID"}</label>
            <input id="edit-cm-id" type="text" bind:value={editCustomModelId} class="w-full px-3 py-2 rounded border text-sm" style="background-color: var(--bg-secondary); color: var(--text-primary); border-color: var(--border);" />
          </div>
          <div class="flex-1">
            <label for="edit-cm-name" class="block text-xs mb-1.5 font-medium" style="color: var(--text-secondary);">{$t("settings.ai.modelName") || "模型名称"}</label>
            <input id="edit-cm-name" type="text" bind:value={editCustomModelName} class="w-full px-3 py-2 rounded border text-sm" style="background-color: var(--bg-secondary); color: var(--text-primary); border-color: var(--border);" />
          </div>
        </div>
        <div>
          <label
            for="edit-cm-apikey"
            class="block text-xs mb-1.5 font-medium"
            style="color: var(--text-secondary);"
            >{$t("settings.ai.apiKey")}</label
          >
          <div class="relative">
            <input
              id="edit-cm-apikey"
              type={editCustomModelApiKeyVisible ? "text" : "password"}
              bind:value={editCustomModelApiKey}
              placeholder="sk-..."
              class="w-full px-3 py-2 pr-9 rounded border text-sm"
              style="background-color: var(--bg-secondary); color: var(--text-primary); border-color: var(--border);"
            />
            <button
              class="absolute right-2 top-1/2 -translate-y-1/2 p-1 rounded transition-colors hover:bg-white/10"
              style="color: var(--text-secondary);"
              onclick={() =>
                (editCustomModelApiKeyVisible = !editCustomModelApiKeyVisible)}
            >
              {#if editCustomModelApiKeyVisible}
                <svg
                  xmlns="http://www.w3.org/2000/svg"
                  class="w-4 h-4"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                  ><path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M13.875 18.825A10.05 10.05 0 0112 19c-4.478 0-8.268-2.943-9.543-7a9.97 9.97 0 011.563-3.029m5.858.908a3 3 0 114.243 4.243M9.878 9.878l4.242 4.242M9.88 9.88l-3.29-3.29m7.532 7.532l3.29 3.29M3 3l3.59 3.59m0 0A9.953 9.953 0 0112 5c4.478 0 8.268 2.943 9.543 7a10.025 10.025 0 01-4.132 5.411m0 0L21 21"
                  /></svg
                >
              {:else}
                <svg
                  xmlns="http://www.w3.org/2000/svg"
                  class="w-4 h-4"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                  ><path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"
                  /><path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z"
                  /></svg
                >
              {/if}
            </button>
          </div>
        </div>
        <div>
          <label
            for="edit-cm-endpoint"
            class="block text-xs mb-1.5 font-medium"
            style="color: var(--text-secondary);"
            >{$t("settings.ai.endpoint")}</label
          >
          <input
            id="edit-cm-endpoint"
            type="text"
            bind:value={editCustomModelEndpoint}
            class="w-full px-3 py-2 rounded border text-sm"
            style="background-color: var(--bg-secondary); color: var(--text-primary); border-color: var(--border);"
          />
        </div>
        <div class="flex gap-2">
          <button
            class="flex-1 px-3 py-1.5 rounded text-xs font-medium transition-colors"
            style="background-color: #094771; color: #ffffff;"
            onclick={async () => {
              if (!editCustomModelApiKey && !editCustomModelEndpoint) return;
              editFetchingModels = true;
              editFetchedModelList = [];
              editFetchError = "";
              try {
                let url =
                  editCustomModelEndpoint ||
                  builtinProviders.find((p) => p.id === editCustomModelProvider)
                    ?.defaultEndpoint ||
                  "";
                if (editCustomModelProvider === "ollama") {
                  url = url.replace(/\/+$/, "") + "/api/tags";
                } else {
                  url =
                    url
                      .replace(/\/chat\/completions\/?$/, "")
                      .replace(/\/completions\/?$/, "")
                      .replace(/\/+$/, "") + "/models";
                }
                const headers = /** @type {Record<string, string>} */ ({
                  "Content-Type": "application/json",
                });
                if (editCustomModelApiKey)
                  headers["Authorization"] = `Bearer ${editCustomModelApiKey}`;
                const resp = await fetch(url, {
                  headers,
                  signal: AbortSignal.timeout(15000),
                });
                if (!resp.ok) {
                  editFetchError = `Failed (HTTP ${resp.status})`;
                  return;
                }
                const data = await resp.json();
                const items = data.data || data.models || [];
                editFetchedModelList = items
                  .map(
                    /** @type {(m: any) => string} */ (
                      (m) => (typeof m === "string" ? m : m.id || m.name)
                    ),
                  )
                  .filter(Boolean);
                if (editFetchedModelList.length === 0)
                  editFetchError = "No models found";
              } catch (/** @type {any} */ e) {
                editFetchError = e.message || String(e);
              } finally {
                editFetchingModels = false;
              }
            }}
            disabled={editFetchingModels ||
              (!editCustomModelApiKey && !editCustomModelEndpoint)}
          >
            {editFetchingModels
              ? $t("settings.ai.fetching") || "获取中..."
              : $t("settings.ai.fetchModels") || "获取模型列表"}
          </button>
        </div>
        {#if editFetchedModelList.length > 0}
          <div
            class="rounded border overflow-y-auto"
            style="background-color: var(--bg-secondary); border-color: var(--border); max-height: 120px;"
          >
            {#each editFetchedModelList as fm}
              <div
                class="flex items-center gap-2 px-3 py-1.5 text-xs cursor-pointer border-b"
                style="color: var(--text-primary); border-color: var(--border); background-color: {editCustomModelId ===
                fm
                  ? '#094771'
                  : 'transparent'};"
                onclick={() => {
                  editCustomModelId = fm;
                  editCustomModelName = fm;
                }}
                role="button"
                tabindex="0"
                onkeydown={(e) => {
                  if (e.key === "Enter" || e.key === " ") {
                    e.preventDefault();
                    editCustomModelId = fm;
                    editCustomModelName = fm;
                  }
                }}
              >
                <span style="width: 16px; text-align: center; color: #2ea043;"
                  >{editCustomModelId === fm ? "✓" : ""}</span
                >
                <span class="truncate">{fm}</span>
              </div>
            {/each}
          </div>
        {/if}
        {#if editFetchError}<p class="text-xs" style="color: #f14c4c;">
            {editFetchError}
          </p>{/if}
        <div class="flex justify-end gap-3 pt-2">
          <button
            class="px-4 py-2 rounded text-sm"
            style="background-color: var(--border); color: var(--text-primary);"
            onclick={() => (showEditCustomModelDialog = false)}
            >{$t("settings.cancel")}</button
          >
          <button
            class="px-4 py-2 rounded text-sm font-medium"
            style="background-color: var(--accent); color: #ffffff;"
            onclick={saveEditCustomModel}>{$t("settings.save")}</button
          >
        </div>
      </div>
    </div>
  </div>
{/if}
