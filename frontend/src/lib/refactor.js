export async function lspRename(filePath, position, newName) {
  if (!window.backend?.LSPRename) return null
  try {
    return await window.backend.LSPRename(filePath, position.line, position.col, newName)
  } catch (e) {
    console.error('LSP Rename failed:', e)
    return null
  }
}

export async function lspFormatting(filePath) {
  if (!window.backend?.LSPFormatting) return null
  try {
    return await window.backend.LSPFormatting(filePath)
  } catch (e) {
    console.error('LSP Formatting failed:', e)
    return null
  }
}

export async function lspCodeActions(filePath, position) {
  if (!window.backend?.LSPCodeActions) return []
  try {
    return await window.backend.LSPCodeActions(filePath, position.line, position.col)
  } catch (e) {
    console.error('LSP CodeActions failed:', e)
    return []
  }
}

export async function lspDefinition(filePath, position) {
  if (!window.backend?.LSPDefinition) return null
  try {
    return await window.backend.LSPDefinition(filePath, position.line, position.col)
  } catch (e) {
    console.error('LSP Definition failed:', e)
    return null
  }
}

export async function lspReferences(filePath, position) {
  if (!window.backend?.LSPReferences) return []
  try {
    return await window.backend.LSPReferences(filePath, position.line, position.col)
  } catch (e) {
    console.error('LSP References failed:', e)
    return []
  }
}

export async function extractFunction(filePath, startLine, endLine, functionName) {
  if (!window.backend?.AIChat) return null
  const prompt = `Extract the code from line ${startLine} to line ${endLine} in ${filePath} into a new function named "${functionName}". Return only the refactored code as a diff.`
  try {
    return await window.backend.AIChat({
      messages: [{ role: 'user', content: prompt }],
      mode: 'plan',
      activeFile: filePath,
    })
  } catch (e) {
    console.error('Extract function failed:', e)
    return null
  }
}

export async function inlineVariable(filePath, position, varName) {
  if (!window.backend?.AIChat) return null
  const prompt = `Inline the variable "${varName}" at line ${position.line} in ${filePath}. Replace all usages with the variable's value/expression and remove the variable declaration. Return only the refactored code as a diff.`
  try {
    return await window.backend.AIChat({
      messages: [{ role: 'user', content: prompt }],
      mode: 'plan',
      activeFile: filePath,
    })
  } catch (e) {
    console.error('Inline variable failed:', e)
    return null
  }
}