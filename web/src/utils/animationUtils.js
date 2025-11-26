/**
 * Animation utilities for smooth file tree updates
 * Provides fade in/out animations for added/deleted files
 */

/**
 * Apply fade-in animation to an element
 * @param {HTMLElement} element - The element to animate
 * @param {number} duration - Duration in milliseconds (default 300ms)
 * @returns {Promise} Resolves when animation completes
 */
export function fadeIn(element, duration = 300) {
  return new Promise((resolve) => {
    // Set initial opacity and visibility
    element.style.opacity = '0'
    element.style.visibility = 'visible'
    element.style.transition = `opacity ${duration}ms ease-in`

    // Trigger reflow to ensure transition is applied
    void element.offsetWidth

    // Animate to full opacity
    element.style.opacity = '1'

    // Resolve when animation completes
    setTimeout(resolve, duration)
  })
}

/**
 * Apply fade-out animation to an element
 * @param {HTMLElement} element - The element to animate
 * @param {number} duration - Duration in milliseconds (default 300ms)
 * @returns {Promise} Resolves when animation completes
 */
export function fadeOut(element, duration = 300) {
  return new Promise((resolve) => {
    element.style.opacity = '1'
    element.style.transition = `opacity ${duration}ms ease-out`

    // Trigger reflow to ensure transition is applied
    void element.offsetWidth

    // Animate to transparent
    element.style.opacity = '0'

    // Resolve when animation completes
    setTimeout(() => {
      element.style.visibility = 'hidden'
      element.style.transition = ''
      resolve()
    }, duration)
  })
}

/**
 * Apply slide-down animation for new items
 * @param {HTMLElement} element - The element to animate
 * @param {number} duration - Duration in milliseconds (default 300ms)
 * @returns {Promise} Resolves when animation completes
 */
export function slideDown(element, duration = 300) {
  return new Promise((resolve) => {
    const startHeight = 0
    const endHeight = element.scrollHeight

    element.style.height = `${startHeight}px`
    element.style.overflow = 'hidden'
    element.style.transition = `height ${duration}ms ease-in`

    // Trigger reflow
    void element.offsetWidth

    element.style.height = `${endHeight}px`

    setTimeout(() => {
      element.style.height = 'auto'
      element.style.transition = ''
      resolve()
    }, duration)
  })
}

/**
 * Apply slide-up animation for removed items
 * @param {HTMLElement} element - The element to animate
 * @param {number} duration - Duration in milliseconds (default 300ms)
 * @returns {Promise} Resolves when animation completes
 */
export function slideUp(element, duration = 300) {
  return new Promise((resolve) => {
    const startHeight = element.scrollHeight
    const endHeight = 0

    element.style.height = `${startHeight}px`
    element.style.overflow = 'hidden'
    element.style.transition = `height ${duration}ms ease-out`

    // Trigger reflow
    void element.offsetWidth

    element.style.height = `${endHeight}px`

    setTimeout(() => {
      element.style.display = 'none'
      element.style.transition = ''
      resolve()
    }, duration)
  })
}

/**
 * Highlight an element briefly to draw attention
 * @param {HTMLElement} element - The element to animate
 * @param {string} highlightColor - CSS color (default: yellow with opacity)
 * @param {number} duration - Duration in milliseconds (default 500ms)
 * @returns {Promise} Resolves when animation completes
 */
export function highlight(element, highlightColor = 'rgba(255, 255, 0, 0.3)', duration = 500) {
  return new Promise((resolve) => {
    const originalBg = element.style.backgroundColor

    element.style.backgroundColor = highlightColor
    element.style.transition = `background-color ${duration}ms ease-out`

    // Trigger reflow
    void element.offsetWidth

    element.style.backgroundColor = originalBg

    setTimeout(() => {
      element.style.transition = ''
      resolve()
    }, duration)
  })
}

/**
 * Combined fade-in + slide-down for new items (entry animation)
 * @param {HTMLElement} element - The element to animate
 * @param {number} duration - Duration in milliseconds (default 300ms)
 * @returns {Promise} Resolves when animation completes
 */
export async function entryAnimation(element, duration = 300) {
  element.style.opacity = '0'
  element.style.height = '0px'
  element.style.overflow = 'hidden'
  element.style.transition = `opacity ${duration}ms ease-in, height ${duration}ms ease-in`

  // Trigger reflow
  void element.offsetWidth

  const endHeight = element.scrollHeight
  element.style.opacity = '1'
  element.style.height = `${endHeight}px`

  return new Promise((resolve) => {
    setTimeout(() => {
      element.style.height = 'auto'
      element.style.transition = ''
      resolve()
    }, duration)
  })
}

/**
 * Combined fade-out + slide-up for removed items (exit animation)
 * @param {HTMLElement} element - The element to animate
 * @param {number} duration - Duration in milliseconds (default 300ms)
 * @returns {Promise} Resolves when animation completes
 */
export async function exitAnimation(element, duration = 300) {
  const startHeight = element.scrollHeight

  element.style.height = `${startHeight}px`
  element.style.overflow = 'hidden'
  element.style.transition = `opacity ${duration}ms ease-out, height ${duration}ms ease-out`
  element.style.opacity = '1'

  // Trigger reflow
  void element.offsetWidth

  element.style.opacity = '0'
  element.style.height = '0px'

  return new Promise((resolve) => {
    setTimeout(() => {
      element.style.display = 'none'
      element.style.transition = ''
      resolve()
    }, duration)
  })
}
