// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import type { Meta, StoryObj } from '@storybook/vue3-vite'

import TableCell from './TableCell.vue'
import TableRoot from './TableRoot.vue'
import TableRow from './TableRow.vue'

const meta: Meta = {
  component: TableRoot,
  subcomponents: { TableRow, TableCell },
  args: {
    label: 'Label',
  },
}

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {
  render: () => ({
    components: { TableRoot, TableRow, TableCell },
    template: `
      <TableRoot>
        <template #head>
          <TableRow>
            <TableCell th>Cat</TableCell>
            <TableCell th>Food</TableCell>
            <TableCell th>Science</TableCell>
            <TableCell th>Plane</TableCell>
            <TableCell th>Book</TableCell>
            <TableCell th>Product</TableCell>
          </TableRow>
        </template>

        <template #body>
          ${faker.helpers
            .multiple(
              () => `
              <TableRow>
                <TableCell>${faker.animal.cat()}</TableCell>
                <TableCell>${faker.food.dish()}</TableCell>
                <TableCell>${faker.science.chemicalElement().name}</TableCell>
                <TableCell>${faker.airline.airplane().name}</TableCell>
                <TableCell>${faker.book.title()}</TableCell>
                <TableCell>${faker.commerce.productName()}</TableCell>
              </TableRow>
            `,
              { count: 100 },
            )
            .join('')}
        </template>
        </TableRoot>
    `,
  }),
}
