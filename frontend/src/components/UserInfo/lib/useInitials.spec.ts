// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { describe, expect, test } from 'vitest'
import { ref } from 'vue'

import { useInitials } from './useInitials'

describe('useInitials', () => {
  test.each`
    source                            | expected
    ${'John Doe'}                     | ${'JD'}
    ${'John'}                         | ${'J'}
    ${'John Michael Doe'}             | ${'JD'}
    ${'  John   Doe  '}               | ${'JD'}
    ${'Mary-Jane Watson'}             | ${'MW'}
    ${'(John) Doe'}                   | ${'D'}
    ${'john.doe@mail.com'}            | ${'JD'}
    ${'john_doe@mail.com'}            | ${'JD'}
    ${'john-doe@mail.com'}            | ${'JD'}
    ${'john@mail.com'}                | ${'J'}
    ${'John Doe (john.doe@mail.com)'} | ${'JD'}
    ${'John Doe <john.doe@mail.com>'} | ${'JD'}
    ${'Иван Петров'}                  | ${'ИП'}
    ${'иван петров'}                  | ${'ИП'}
    ${'Γιώργος Παπαδόπουλος'}         | ${'ΓΠ'}
    ${'José Álvarez'}                 | ${'JÁ'}
    ${'José álvarez'}                 | ${'JÁ'}
    ${'Ölçer Şahin'}                  | ${'ÖŞ'}
    ${''}                             | ${''}
    ${'   '}                          | ${''}
    ${'123 456'}                      | ${''}
    ${'🙂 😀'}                        | ${''}
    ${'@mail.com'}                    | ${''}
  `('$source → $expected', ({ source, expected }) => {
    expect(useInitials(source).value).toBe(expected)
  })

  test('reacts to source changes', () => {
    const source = ref('John Doe')
    const initials = useInitials(source)

    expect(initials.value).toBe('JD')

    source.value = 'Иван Петров'

    expect(initials.value).toBe('ИП')
  })

  test('accepts a getter', () => {
    const name = ref('')
    const email = 'jane.roe@mail.com'
    const initials = useInitials(() => name.value.trim() || email)

    expect(initials.value).toBe('JR')

    name.value = 'John Doe'

    expect(initials.value).toBe('JD')
  })
})
