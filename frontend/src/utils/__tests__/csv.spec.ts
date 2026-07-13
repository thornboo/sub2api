import { describe, expect, it } from 'vitest'
import { escapeCSVValue, serializeCSV } from '../csv'

describe('CSV serialization', () => {
  it.each(['=1+1', '+SUM(A1:A2)', '-2+3', '@cmd', '\tformula', '\rformula'])(
    'neutralizes spreadsheet formula prefix %j',
    (value) => {
      expect(escapeCSVValue(value)).toBe(`"'${value}"`)
    },
  )

  it('quotes separators and escapes embedded quotes', () => {
    expect(escapeCSVValue('Finance, "East"')).toBe('"Finance, ""East"""')
  })

  it('applies the same protection to headers and every row', () => {
    expect(serializeCSV(['name'], [['=HYPERLINK("https://example.com")']])).toBe(
      'name\n"\'=HYPERLINK(""https://example.com"")"',
    )
  })
})
