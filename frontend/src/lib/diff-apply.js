export function parseAIDiff(diffText) {
  const hunks = []
  const lines = diffText.split('\n')
  let currentHunk = null
  let filePath = ''

  for (const line of lines) {
    if (line.startsWith('--- ') || line.startsWith('+++ ')) {
      filePath = line.slice(4).trim()
      continue
    }
    if (line.startsWith('@@')) {
      if (currentHunk) hunks.push(currentHunk)
      currentHunk = { filePath, oldLines: [], newLines: [], header: line }
      continue
    }
    if (!currentHunk) continue
    if (line.startsWith('+') || line.startsWith('>') ) {
      currentHunk.newLines.push(line.slice(1))
    } else if (line.startsWith('-') || line.startsWith('<')) {
      currentHunk.oldLines.push(line.slice(1))
    } else {
      const content = line.startsWith(' ') ? line.slice(1) : line
      currentHunk.oldLines.push(content)
      currentHunk.newLines.push(content)
    }
  }
  if (currentHunk) hunks.push(currentHunk)
  return hunks
}

export function buildApplyEdit(hunk) {
  if (!hunk.oldLines.length && !hunk.newLines.length) return null
  return {
    filePath: hunk.filePath,
    oldText: hunk.oldLines.join('\n'),
    newText: hunk.newLines.join('\n'),
  }
}

export function buildApplyEdits(hunks) {
  return hunks.map(buildApplyEdit).filter(Boolean)
}