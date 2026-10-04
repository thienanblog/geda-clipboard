import test from 'node:test'
import assert from 'node:assert/strict'
import { searchSettings } from '../src/lib/settingsSearch.ts'

const desktop = { isMac: true, pasteSupported: true, canPaste: false, canUpdate: true }
const ids = (query, capabilities = desktop) => searchSettings(query, capabilities).map((item) => item.id)

test('search finds cross-category controls with case, accents and multiple words', () => {
  assert.deepEqual(ids('  HiStOrY  LIMIT '), ['history-limit'])
  assert.deepEqual(ids('thư mục'), ['capture-files'])
  assert.deepEqual(ids('phím tắt'), ['hotkey'])
  assert.ok(ids('privacy').includes('notification-preview'))
  assert.deepEqual(ids(''), [])
  assert.deepEqual(ids('   '), [])
  assert.deepEqual(ids('something that does not exist'), [])
})

test('search only exposes controls supported by the current build and platform', () => {
  const appStore = { ...desktop, pasteSupported: false, canUpdate: false }
  assert.ok(!ids('paste', appStore).includes('paste'))
  assert.deepEqual(ids('accessibility', appStore), [])
  assert.deepEqual(ids('updates', appStore), [])
  assert.deepEqual(ids('dock', { ...desktop, isMac: false }), [])
  assert.deepEqual(ids('accessibility', { ...desktop, canPaste: true }), [])
  assert.deepEqual(ids('accessibility'), ['paste-permission'])
})
