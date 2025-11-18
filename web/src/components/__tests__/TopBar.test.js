import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount } from '@vue/test-utils';
import { createRouter, createMemoryHistory } from 'vue-router';
import TopBar from '../TopBar.vue';

function createTestRouter() {
  return createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/', name: 'home', component: { template: '<div>Home</div>' } },
      { path: '/settings', name: 'settings', component: { template: '<div>Settings Page</div>' } }
    ],
  });
}

describe('TopBar', () => {
  let router;

  beforeEach(() => {
    router = createTestRouter();
  });

  it('renders properly', () => {
    const wrapper = mount(TopBar, {
      global: {
        plugins: [router],
      },
    });
    expect(wrapper.text()).toContain('Obsidian Web');
    expect(wrapper.find('.settings-icon').exists()).toBe(true);
  });

  it('calls router push when settings icon is clicked', async () => {
    router.push('/'); // Start at home route
    await router.isReady();

    const pushSpy = vi.spyOn(router, 'push');

    const wrapper = mount(TopBar, {
      global: {
        plugins: [router],
      },
    });

    await wrapper.find('.settings-icon').trigger('click');

    expect(pushSpy).toHaveBeenCalledWith({ name: 'settings' });
  });
});
