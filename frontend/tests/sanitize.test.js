import { describe, it, expect } from 'vitest'
import { sanitizeHtml, sanitizeMarkdownHtml } from '../src/lib/sanitize.js'

describe('sanitizeHtml', () => {
  it('removes script tags', () => {
    const result = sanitizeHtml('<script>alert("xss")</script><p>safe</p>')
    expect(result).not.toContain('<script>')
    expect(result).toContain('<p>safe</p>')
  })

  it('removes onerror attributes', () => {
    const result = sanitizeHtml('<img src="x" onerror="alert(1)">')
    expect(result).not.toContain('onerror')
  })

  it('removes iframe tags', () => {
    const result = sanitizeHtml('<iframe src="evil.com"></iframe>')
    expect(result).not.toContain('<iframe')
  })

  it('preserves safe links', () => {
    const result = sanitizeHtml('<a href="https://example.com">link</a>')
    expect(result).toContain('href="https://example.com"')
    expect(result).toContain('rel="noopener noreferrer"')
  })

  it('preserves code blocks', () => {
    const result = sanitizeHtml('<pre><code>const x = 1</code></pre>')
    expect(result).toContain('<code>')
  })

  it('handles empty input', () => {
    expect(sanitizeHtml('')).toBe('')
    expect(sanitizeHtml(null)).toBe('')
  })
})

describe('sanitizeMarkdownHtml', () => {
  it('preserves div and span', () => {
    const result = sanitizeMarkdownHtml('<div class="highlight"><span>code</span></div>')
    expect(result).toContain('<div')
    expect(result).toContain('<span>')
  })

  it('removes form elements', () => {
    const result = sanitizeMarkdownHtml('<form action="/evil"><input type="text"></form>')
    expect(result).not.toContain('<form')
    expect(result).not.toContain('<input')
  })
})