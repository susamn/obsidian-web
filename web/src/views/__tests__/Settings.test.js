import { describe, it, expect } from 'vitest';
import { mount } from '@vue/test-utils';
import { createPinia } from 'pinia';
import Settings from '../Settings.vue';

describe('Settings', () => {
  it('renders properly', () => {
    const wrapper = mount(Settings, {
      global: {
        plugins: [createPinia()],
      },
    });
    expect(wrapper.text()).toContain('Settings');
  });
});
