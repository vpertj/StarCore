<script>
  import { loadDirectoryContents } from '../stores/app.js'
  import TreeNode from './TreeNode.svelte'
  import { getFileIcon, FILE_ICONS, getIconColor, getFolderColor } from '../themes/icons.js'
  import { currentTheme, iconTheme, getThemeColors } from '../stores/theme.js'

  let { item, depth = 0, expandedDirs, onToggle, onFileClick, onContextMenu } = $props()

  let loading = $state(false)
  let hovered = $state(false)

  async function handleClick() {
    if (item.isDir) {
      if (expandedDirs.has(item.path)) {
        expandedDirs.delete(item.path)
      } else {
        expandedDirs.add(item.path)
        if (!item.loaded) {
          loading = true
          await loadDirectoryContents(item)
          loading = false
        }
      }
      onToggle()
    } else {
      onFileClick(item.path)
    }
  }

  function isExpanded(path) {
    return expandedDirs.has(path)
  }

  function getIcon() {
    const it = $iconTheme
    const tc = getThemeColors($currentTheme ?? 'one-dark')
    if (item.isDir) {
      const open = isExpanded(item.path)
      const f = FILE_ICONS.folder
      return {
        path: f.path,
        color: open ? getFolderColor(it, tc, true) : getFolderColor(it, tc, false),
        open
      }
    }
    const iconName = getFileIcon(item.name, false)
    const icon = FILE_ICONS[iconName] || FILE_ICONS.default
    return { path: icon.path, color: getIconColor(iconName, it, tc), open: false }
  }

</script>

<!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
<div
  class="tree-row"
  class:tree-row-hovered={hovered}
  style="padding-left: {depth * 16 + 4}px;"
  role="treeitem"
  tabindex="0"
  aria-selected="false"
  onclick={handleClick}
  oncontextmenu={(e) => onContextMenu(e, item)}
  onpointerenter={() => hovered = true}
  onpointerleave={() => hovered = false}
>
  {#if item.isDir}
    <div class="tree-chevron" style="transform: rotate({isExpanded(item.path) ? '90deg' : '0deg'});">
      <svg viewBox="0 0 10 10" width="10" height="10">
        <polygon points="2,1 8,5 2,9" fill="currentColor"/>
      </svg>
    </div>
    <svg class="tree-icon" viewBox="0 0 16 16">
      {#if isExpanded(item.path)}
        <path d={getIcon().path} fill={getIcon().color} opacity="0.8"/>
        <path d="M1.5 5h12.5v8h-12.5z" fill={getIcon().color}/>
      {:else}
        <path d={getIcon().path} fill={getIcon().color} opacity="0.6"/>
      {/if}
    </svg>
  {:else}
    <span class="tree-chevron-spacer"></span>
    <svg class="tree-icon" viewBox="0 0 16 16">
      <path d={getIcon().path} fill={getIcon().color}/>
    </svg>
  {/if}
  <span class="tree-label">
    {#if loading}
      {item.name}...
    {:else}
      {item.name}
    {/if}
  </span>
</div>
{#if item.isDir && isExpanded(item.path) && item.children && item.children.length > 0}
  {#each item.children as child (child.path)}
    <TreeNode item={child} depth={depth + 1} {expandedDirs} {onToggle} {onFileClick} {onContextMenu} />
  {/each}
{/if}

<style>
.tree-row {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 2px 8px 2px 4px;
  min-height: 28px;
  cursor: pointer;
  color: var(--text-primary);
  border-radius: 3px;
  transition: background-color 0.1s ease;
  user-select: none;
}
.tree-row-hovered {
  background-color: rgba(255, 255, 255, 0.04);
}
.tree-row:active {
  background-color: rgba(255, 255, 255, 0.08);
}

.tree-chevron {
  width: 16px;
  height: 16px;
  flex-shrink: 0;
  color: #c0c0c0;
  transition: transform 0.15s cubic-bezier(0.25, 0.1, 0.25, 1);
  display: flex;
  align-items: center;
  justify-content: center;
}
.tree-row-hovered .tree-chevron {
  color: #e0e0e0;
}
.tree-chevron-spacer {
  width: 16px;
  flex-shrink: 0;
}

.tree-icon {
  width: 16px;
  height: 16px;
  flex-shrink: 0;
}

.tree-label {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 13px;
  line-height: 28px;
}
</style>
