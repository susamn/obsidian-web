import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import {
  fadeIn,
  fadeOut,
  slideDown,
  slideUp,
  highlight,
  entryAnimation,
  exitAnimation,
} from '../animationUtils'

describe('Animation Utils', () => {
  let element

  beforeEach(() => {
    // Create a mock DOM element
    element = document.createElement('div')
    element.style.opacity = '1'
    element.style.visibility = 'visible'

    // Mock scrollHeight as a getter (read-only property)
    Object.defineProperty(element, 'scrollHeight', {
      get: () => 100,
      configurable: true,
    })

    document.body.appendChild(element)

    // Mock requestAnimationFrame
    vi.useFakeTimers()
  })

  afterEach(() => {
    if (document.body.contains(element)) {
      document.body.removeChild(element)
    }
    vi.runOnlyPendingTimers()
    vi.useRealTimers()
  })

  describe('fadeIn', () => {
    it('should animate from opacity 0 to 1', async () => {
      expect(element.style.opacity).toBe('1')

      const promise = fadeIn(element, 300)

      // After calling fadeIn, it should set opacity to 0
      // Then trigger reflow and set to 1
      // Check the promise resolves after duration
      vi.advanceTimersByTime(300)
      await promise

      expect(element.style.visibility).toBe('visible')
    })

    it('should apply transition CSS', async () => {
      const promise = fadeIn(element, 500)
      expect(element.style.transition).toContain('opacity')
      expect(element.style.transition).toContain('500ms')

      vi.advanceTimersByTime(500)
      await promise
    })

    it('should resolve after duration', async () => {
      const promise = fadeIn(element, 200)
      vi.advanceTimersByTime(200)
      await promise
      // If it gets here without timeout, test passes
      expect(element.style.visibility).toBe('visible')
    })
  })

  describe('fadeOut', () => {
    it('should animate from opacity 1 to 0', async () => {
      const promise = fadeOut(element, 300)
      expect(element.style.opacity).toBe('0')

      vi.advanceTimersByTime(300)
      await promise

      expect(element.style.visibility).toBe('hidden')
    })

    it('should apply transition CSS', async () => {
      const promise = fadeOut(element, 400)
      expect(element.style.transition).toContain('opacity')
      expect(element.style.transition).toContain('400ms')

      vi.advanceTimersByTime(400)
      await promise
    })
  })

  describe('slideDown', () => {
    it('should animate height from 0 to scrollHeight', async () => {
      const promise = slideDown(element, 300)

      expect(element.style.overflow).toBe('hidden')
      expect(element.style.transition).toContain('height')

      vi.advanceTimersByTime(300)
      await promise

      expect(element.style.height).toBe('auto')
      expect(element.style.transition).toBe('')
    })

    it('should set overflow to hidden', async () => {
      const promise = slideDown(element, 300)
      expect(element.style.overflow).toBe('hidden')

      vi.advanceTimersByTime(300)
      await promise
      expect(element.style.height).toBe('auto')
    })
  })

  describe('slideUp', () => {
    it('should animate height to 0', async () => {
      const promise = slideUp(element, 300)

      expect(element.style.overflow).toBe('hidden')
      expect(element.style.transition).toContain('height')

      vi.advanceTimersByTime(300)
      await promise

      expect(element.style.height).toBe('0px')
      expect(element.style.display).toBe('none')
    })
  })

  describe('highlight', () => {
    it('should apply and remove highlight color', async () => {
      const promise = highlight(element, 'rgba(255, 255, 0, 0.3)', 500)

      expect(element.style.transition).toContain('background-color')

      vi.advanceTimersByTime(500)
      await promise

      expect(element.style.transition).toBe('')
    })

    it('should use default color if not specified', async () => {
      const promise = highlight(element, undefined, 300)
      expect(element.style.transition).toContain('background-color')

      vi.advanceTimersByTime(300)
      await promise
      expect(element.style.transition).toBe('')
    })
  })

  describe('entryAnimation', () => {
    it('should combine fade in and slide down', async () => {
      const promise = entryAnimation(element, 300)

      expect(element.style.overflow).toBe('hidden')
      expect(element.style.transition).toContain('opacity')
      expect(element.style.transition).toContain('height')

      vi.advanceTimersByTime(300)
      await promise

      expect(element.style.height).toBe('auto')
      expect(element.style.transition).toBe('')
    })

    it('should apply combined transition', async () => {
      const promise = entryAnimation(element, 400)
      expect(element.style.transition).toContain('opacity')
      expect(element.style.transition).toContain('height')

      vi.advanceTimersByTime(400)
      await promise
      expect(element.style.transition).toBe('')
    })
  })

  describe('exitAnimation', () => {
    it('should combine fade out and slide up', async () => {
      const promise = exitAnimation(element, 300)

      expect(element.style.overflow).toBe('hidden')
      expect(element.style.transition).toContain('opacity')
      expect(element.style.transition).toContain('height')

      vi.advanceTimersByTime(300)
      await promise

      expect(element.style.display).toBe('none')
    })

    it('should apply combined transition', async () => {
      const promise = exitAnimation(element, 350)
      expect(element.style.transition).toContain('opacity')
      expect(element.style.transition).toContain('height')

      vi.advanceTimersByTime(350)
      await promise
      expect(element.style.transition).toBe('')
    })
  })

  describe('Animation timing', () => {
    it('should respect custom duration', async () => {
      const durations = [100, 300, 500, 1000]

      for (const duration of durations) {
        const testElement = document.createElement('div')

        // Mock scrollHeight for this element
        Object.defineProperty(testElement, 'scrollHeight', {
          get: () => 100,
          configurable: true,
        })

        document.body.appendChild(testElement)

        const startTime = Date.now()
        const promise = fadeIn(testElement, duration)

        vi.advanceTimersByTime(duration)
        await promise

        const elapsed = Date.now() - startTime
        // Allow some tolerance for timers
        expect(elapsed).toBeGreaterThanOrEqual(duration - 10)

        document.body.removeChild(testElement)
      }
    })
  })
})
