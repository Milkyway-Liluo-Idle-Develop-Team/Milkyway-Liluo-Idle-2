import { test as base } from '@playwright/test'

/**
 * Shared test fixtures / helpers.
 */
export const test = base.extend({
  // Can extend with authenticated page, custom fixtures, etc.
})

export { expect } from '@playwright/test'
