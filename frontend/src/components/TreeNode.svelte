<script>
  import { loadDirectoryContents } from '../stores/app.js'
  import TreeNode from './TreeNode.svelte'

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
        <path d="M1.5 2.5h5.5l1.5 1.5h5.5v9h-12.5z" fill="#dcb67a" opacity="0.8"/>
        <path d="M1.5 5h12.5v8h-12.5z" fill="#dcb67a"/>
      {:else}
        <path d="M1.5 2.5h5.5l1.5 1.5h5.5v9h-12.5z" fill="#dcb67a" opacity="0.6"/>
      {/if}
    </svg>
  {:else}
    <span class="tree-chevron-spacer"></span>
    <svg class="tree-icon" viewBox="0 0 16 16">
      <path d="M3 1.5h6.5a1 1 0 01.7.3l2.5 2.5a1 1 0 01.3.7v9.5a1 1 0 01-1 1h-8.5a1 1 0 01-1-1v-12a1 1 0 011-1z" fill="none" stroke="currentColor" stroke-width="1"/>
      <path d="M9.5 1.5v3h3" fill="none" stroke="currentColor" stroke-width="1"/>
    </svg>
  {/if}
  <span class="tree-label" style="color: {item.isDir ? 'var(--text-primary)' : '#6a9955'};" title={item.name}>
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
  padding: 1px 8px 1px 4px;
  height: 22px;
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
  color: #6a9955;
}

.tree-label {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 13px;
  line-height: 22px;
}
</style>
