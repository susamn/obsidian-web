import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mount } from '@vue/test-utils';
import { createRouter, createMemoryHistory } from 'vue-router';
import { nextTick } from 'vue';
import TopBar from '../TopBar.vue';

// Mock the useSSE composable
vi.mock('../../composables/useSSE', () => ({
  useSSE: vi.fn(() => ({
    connected: { value: false },
    error: { value: null },
    pendingEvents: { value: 0 },
    connect: vi.fn(),
    disconnect: vi.fn(),
  })),
}));

function createTestRouter() {
  return createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/', name: 'home', component: { template: '<div>Home</div>' } },
      { path: '/vault/:id', name: 'vault', component: { template: '<div>Vault</div>' } },
      { path: '/settings', name: 'settings', component: { template: '<div>Settings Page</div>' } }
    ],
  });
}

describe('TopBar', () => {
  let router;

  beforeEach(() => {
    router = createTestRouter();
    vi.clearAllMocks();
  });

  it('renders properly', () => {
    const wrapper = mount(TopBar, {
      global: {
        plugins: [router],
      },
    });
    expect(wrapper.text()).toContain('Obsidian Web');
    expect(wrapper.find('.settings-icon').exists()).toBe(true);
    expect(wrapper.find('.status-widget').exists()).toBe(true);
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

  it('displays "Connecting" status when not connected', () => {
    const wrapper = mount(TopBar, {
      global: {
        plugins: [router],
      },
    });

    expect(wrapper.find('.status-widget').text()).toContain('Connecting');
    expect(wrapper.find('.status-widget').classes()).toContain('connecting');
  });

  it('displays "Live" status when connected', async () => {
    const { useSSE } = await import('../../composables/useSSE');
    useSSE.mockImplementation(() => ({
      connected: { value: true },
      error: { value: null },
      pendingEvents: { value: 0 },
      connect: vi.fn(),
      disconnect: vi.fn(),
    }));

    const wrapper = mount(TopBar, {
      global: {
        plugins: [router],
      },
    });

    await nextTick();
    await wrapper.vm.$nextTick();

    expect(wrapper.find('.status-widget').text()).toContain('Live');
    expect(wrapper.find('.status-widget').classes()).toContain('connected');
  });

  it('displays "Offline" status when there is an error', async () => {
    const { useSSE } = await import('../../composables/useSSE');
    useSSE.mockImplementation(() => ({
      connected: { value: false },
      error: { value: 'Connection failed' },
      pendingEvents: { value: 0 },
      connect: vi.fn(),
      disconnect: vi.fn(),
    }));

    const wrapper = mount(TopBar, {
      global: {
        plugins: [router],
      },
    });

    await nextTick();
    await wrapper.vm.$nextTick();

    expect(wrapper.find('.status-widget').text()).toContain('Offline');
    expect(wrapper.find('.status-widget').classes()).toContain('error');
  });

  it('displays "Syncing" status with pending events count', async () => {
    const { useSSE } = await import('../../composables/useSSE');
    useSSE.mockImplementation(() => ({
      connected: { value: true },
      error: { value: null },
      pendingEvents: { value: 5 },
      connect: vi.fn(),
      disconnect: vi.fn(),
    }));

    const wrapper = mount(TopBar, {
      global: {
        plugins: [router],
      },
    });

    await nextTick();
    await wrapper.vm.$nextTick();

    expect(wrapper.find('.status-widget').text()).toContain('Syncing (5)');
    expect(wrapper.find('.status-widget').classes()).toContain('syncing');
  });

  it('shows spinning icon when syncing', async () => {
    const { useSSE } = await import('../../composables/useSSE');
    useSSE.mockImplementation(() => ({
      connected: { value: true },
      error: { value: null },
      pendingEvents: { value: 3 },
      connect: vi.fn(),
      disconnect: vi.fn(),
    }));

    const wrapper = mount(TopBar, {
      global: {
        plugins: [router],
      },
    });

    await nextTick();
    await wrapper.vm.$nextTick();

    expect(wrapper.find('.fa-sync').exists()).toBe(true);
    expect(wrapper.find('.fa-spin').exists()).toBe(true);
  });

  it('connects to SSE when vault route changes', async () => {
    const mockConnect = vi.fn();
    const mockDisconnect = vi.fn();

    const { useSSE } = await import('../../composables/useSSE');
    useSSE.mockImplementation(() => ({
      connected: { value: false },
      error: { value: null },
      pendingEvents: { value: 0 },
      connect: mockConnect,
      disconnect: mockDisconnect,
    }));

    await router.push('/');
    await router.isReady();

    const wrapper = mount(TopBar, {
      global: {
        plugins: [router],
      },
    });

    await nextTick();

    // Navigate to vault route
    await router.push('/vault/test-vault-id');
    await nextTick();
    await wrapper.vm.$nextTick();

    // Should call connect with vault ID
    expect(mockConnect).toHaveBeenCalled();
  });
});
