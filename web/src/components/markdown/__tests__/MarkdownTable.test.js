
import { mount } from '@vue/test-utils';
import { describe, it, expect } from 'vitest';
import MarkdownTable from '../MarkdownTable.vue';

describe('MarkdownTable', () => {
  const InlineRendererStub = {
    props: ['tokens'],
    template: `
      <div class="inline-renderer-stub">
        <template v-for="token in tokens">
          <span v-if="token.type === 'text'">{{ token.content }}</span>
          <span v-if="token.type === 'wikilink'">{{ token.content }}</span>
        </template>
      </div>
    `
  };

  const node = {
    rows: [
      {
        type: 'header',
        cells: [
          [{ type: 'text', content: 'Header 1' }],
          [{ type: 'text', content: 'Header 2' }]
        ]
      },
      {
        type: 'row',
        cells: [
          [{ type: 'text', content: 'Data 1.1' }],
          [{ type: 'text', content: 'Data 1.2' }]
        ]
      },
      {
        type: 'row',
        cells: [
          [{ type: 'wikilink', content: '[[link]]' }],
          [{ type: 'text', content: 'Data 2.2' }]
        ]
      }
    ]
  };

  it('renders the table with a header and data rows', () => {
    const wrapper = mount(MarkdownTable, {
      props: { node },
      global: {
        stubs: {
          InlineRenderer: InlineRendererStub
        }
      }
    });

    // Check header
    const headers = wrapper.findAll('th');
    expect(headers).toHaveLength(2);
    expect(headers[0].text()).toBe('Header 1');
    expect(headers[1].text()).toBe('Header 2');

    // Check body rows
    const rows = wrapper.findAll('tbody tr');
    expect(rows).toHaveLength(2);

    const firstRowCells = rows[0].findAll('td');
    expect(firstRowCells[0].text()).toBe('Data 1.1');
    expect(firstRowCells[1].text()).toBe('Data 1.2');

    const secondRowCells = rows[1].findAll('td');
    expect(secondRowCells[1].text()).toBe('Data 2.2');
  });

  it('emits a wikilink-click event from a cell', async () => {
    const wrapper = mount(MarkdownTable, {
      props: { node },
      global: {
        stubs: {
          InlineRenderer: {
            // Simple stub that simulates the event emission
            template: '<span @click="$emit(\'wikilink-click\', { href: \'link\' })"></span>',
            props: ['tokens']
          }
        }
      }
    });

    // Find the specific cell that should contain the wikilink and trigger the event
    // In this test case, it's the first cell of the second data row
    const wikilinkCell = wrapper.findAll('tbody tr')[1].findAll('td')[0].find('span');
    await wikilinkCell.trigger('click');

    expect(wrapper.emitted('wikilink-click')).toBeTruthy();
    expect(wrapper.emitted('wikilink-click')[0][0]).toEqual({ href: 'link' });
  });

  it('renders correctly without a header row', () => {
    const nodeWithoutHeader = {
      rows: [
        {
          type: 'row',
          cells: [[{ type: 'text', content: 'Data 1.1' }]]
        }
      ]
    };
    const wrapper = mount(MarkdownTable, {
      props: { node: nodeWithoutHeader },
      global: {
        stubs: { InlineRenderer: InlineRendererStub }
      }
    });

    expect(wrapper.find('thead').exists()).toBe(false);
    expect(wrapper.findAll('tbody tr')).toHaveLength(1);
  });
});
