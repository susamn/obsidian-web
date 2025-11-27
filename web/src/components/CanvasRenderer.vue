<template>
  <div class="canvas-viewer w-full h-full overflow-hidden relative bg-[#111]">
    <!-- Loading State -->
    <div v-if="!content || content === 'loading'" class="loading-state">
      <i class="fas fa-spinner fa-spin text-4xl mb-4" />
      <p>Loading canvas...</p>
    </div>

    <!-- Empty State -->
    <div v-else-if="nodes.length === 0 && edges.length === 0" class="empty-state">
      <i class="fas fa-project-diagram text-4xl mb-4 opacity-50" />
      <p>Empty canvas or failed to parse</p>
      <p class="text-sm text-gray-500 mt-2">Check console for details</p>
    </div>

    <!-- Canvas Content -->
    <template v-else>
      <!-- Controls -->
      <div class="controls">
        <button title="Zoom Out" @click="zoomOut">
          <i class="fas fa-minus" />
        </button>
        <div class="flex items-center justify-center w-12 text-sm text-gray-300">
          {{ Math.round(scale * 100) }}%
        </div>
        <button title="Zoom In" @click="zoomIn">
          <i class="fas fa-plus" />
        </button>
        <button title="Reset View" @click="resetView">
          <i class="fas fa-compress-arrows-alt" />
        </button>
      </div>

      <!-- Canvas Wrapper -->
      <div
        class="canvas-container"
        :style="{
          transform: `translate(${pan.x}px, ${pan.y}px) scale(${scale})`,
        }"
        @mousedown="startPan"
        @wheel="handleWheel"
      >
        <!-- Edges Layer (SVG) -->
        <svg :width="canvasDimensions.width" :height="canvasDimensions.height">
          <path
            v-for="edge in processedEdges"
            :key="edge.id"
            :d="edge.path"
            :stroke="getColor(edge.color) || '#444'"
            marker-end="url(#arrowhead)"
          />
          <defs>
            <marker
              id="arrowhead"
              markerWidth="10"
              markerHeight="7"
              refX="9"
              refY="3.5"
              orient="auto"
            >
              <polygon points="0 0, 10 3.5, 0 7" fill="#555" />
            </marker>
          </defs>
        </svg>

        <!-- Nodes Layer -->
        <div
          v-for="node in processedNodes"
          :key="node.id"
          :class="['node', `node-${node.type}`, getNodeColorClass(node.color)]"
          :style="{
            left: node.x + 'px',
            top: node.y + 'px',
            width: node.width + 'px',
            height: node.height + 'px',
            zIndex: node.type === 'group' ? 1 : 10,
          }"
        >
          <!-- Group Label -->
          <div v-if="node.type === 'group'" class="group-label">
            {{ node.label }}
          </div>

          <!-- Text Node Content -->
          <div v-if="node.type === 'text'" class="w-full h-full flex flex-col">
            <div class="prose prose-invert prose-sm max-w-none p-4 overflow-auto">
              {{ node.text }}
            </div>
          </div>

          <!-- File Node Content -->
          <div v-if="node.type === 'file'" class="w-full h-full flex flex-col">
            <div class="file-header">
              <i class="fas fa-file-alt" />
              <span :title="node.file">{{ getFileName(node.file) }}</span>
            </div>
            <div class="file-content bg-[#202020] flex-1">
              <div
                v-if="isImage(node.file)"
                class="w-full h-full flex items-center justify-center overflow-hidden"
              >
                <div class="text-center p-4">
                  <i class="fas fa-image text-4xl mb-2 text-gray-600" />
                  <div class="text-xs text-gray-500">Image: {{ getFileName(node.file) }}</div>
                  <div class="text-[10px] text-gray-600 mt-1">(Image preview)</div>
                </div>
              </div>
              <div v-else class="text-center p-4">
                <i class="fas fa-file-code text-4xl mb-2 text-gray-600" />
                <div class="text-xs text-gray-500">
                  {{ getFileName(node.file) }}
                </div>
              </div>
            </div>
          </div>

          <!-- Link Node Content -->
          <div v-if="node.type === 'link'" class="w-full h-full flex flex-col">
            <div class="file-header bg-red-900/20 border-red-900/30">
              <i class="fas fa-link" />
              <a :href="node.url" target="_blank" class="hover:underline text-blue-400 truncate">{{
                node.url
              }}</a>
            </div>
            <div class="flex-1 bg-black relative">
              <iframe
                v-if="isYoutube(node.url)"
                class="w-full h-full absolute inset-0 border-0"
                :src="getYoutubeEmbed(node.url)"
                allowfullscreen
              />
              <div v-else class="w-full h-full flex items-center justify-center">
                <div class="text-center">
                  <i class="fas fa-external-link-alt text-2xl mb-2" />
                  <div>External Link</div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </template>
  </div>
</template>

<script setup>
import { ref, computed, watch, onMounted } from 'vue'

const props = defineProps({
  content: {
    type: String,
    required: true,
  },
  vaultId: {
    type: String,
    required: true,
  },
  fileId: {
    type: String,
    required: false,
  },
})

// Canvas state
const scale = ref(0.6)
const pan = ref({ x: 0, y: 0 })
const isDragging = ref(false)
const lastMousePos = ref({ x: 0, y: 0 })

// Canvas Data
const nodes = ref([])
const edges = ref([])

// Parse canvas content
const parseCanvasData = () => {
  try {
    // Skip parsing if content is not ready or is a placeholder
    if (!props.content || props.content === 'loading' || typeof props.content !== 'string') {
      console.log('[CanvasRenderer] Content not ready yet:', props.content)
      nodes.value = []
      edges.value = []
      return
    }

    // Trim whitespace and check if content is actually JSON
    const trimmedContent = props.content.trim()
    if (!trimmedContent || (!trimmedContent.startsWith('{') && !trimmedContent.startsWith('['))) {
      console.warn(
        '[CanvasRenderer] Content does not appear to be JSON:',
        trimmedContent.substring(0, 100)
      )
      nodes.value = []
      edges.value = []
      return
    }

    const data = JSON.parse(trimmedContent)
    nodes.value = data.nodes || []
    edges.value = data.edges || []
    console.log(
      '[CanvasRenderer] Successfully parsed canvas data:',
      nodes.value.length,
      'nodes,',
      edges.value.length,
      'edges'
    )
  } catch (error) {
    console.error('[CanvasRenderer] Failed to parse canvas data:', error)
    console.error('[CanvasRenderer] Content was:', props.content?.substring(0, 200))
    nodes.value = []
    edges.value = []
  }
}

// Watch for content changes
watch(
  () => props.content,
  () => {
    parseCanvasData()
    // Reset view when content changes
    setTimeout(() => resetView(), 100)
  },
  { immediate: true }
)

// Compute Bounding Box to normalize negative coordinates
const bounds = computed(() => {
  if (nodes.value.length === 0) {
    return { minX: 0, minY: 0, width: 1000, height: 1000 }
  }

  let minX = Infinity,
    minY = Infinity,
    maxX = -Infinity,
    maxY = -Infinity
  nodes.value.forEach((node) => {
    if (node.x < minX) minX = node.x
    if (node.y < minY) minY = node.y
    if (node.x + node.width > maxX) maxX = node.x + node.width
    if (node.y + node.height > maxY) maxY = node.y + node.height
  })

  // Add padding
  return {
    minX: minX - 100,
    minY: minY - 100,
    width: maxX - minX + 200,
    height: maxY - minY + 200,
  }
})

// Initialize view to center content
onMounted(() => {
  resetView()
})

const resetView = () => {
  // Calculate shift needed to bring minX/minY to 0,0
  const shiftX = -bounds.value.minX
  const shiftY = -bounds.value.minY

  // Initial Pan to center visually
  pan.value = { x: shiftX * scale.value, y: shiftY * scale.value }
}

// Process Nodes
const processedNodes = computed(() => nodes.value)

const processedEdges = computed(() => {
  return edges.value
    .map((edge) => {
      const fromNode = nodes.value.find((n) => n.id === edge.fromNode)
      const toNode = nodes.value.find((n) => n.id === edge.toNode)

      if (!fromNode || !toNode) return null

      const start = getAnchorPoint(fromNode, edge.fromSide)
      const end = getAnchorPoint(toNode, edge.toSide)

      // Bezier Curve Logic
      const dist = Math.hypot(end.x - start.x, end.y - start.y) * 0.5

      let cp1 = { x: start.x, y: start.y }
      let cp2 = { x: end.x, y: end.y }

      // Adjust control points based on sides
      switch (edge.fromSide) {
        case 'left':
          cp1.x -= dist
          break
        case 'right':
          cp1.x += dist
          break
        case 'top':
          cp1.y -= dist
          break
        case 'bottom':
          cp1.y += dist
          break
      }

      switch (edge.toSide) {
        case 'left':
          cp2.x -= dist
          break
        case 'right':
          cp2.x += dist
          break
        case 'top':
          cp2.y -= dist
          break
        case 'bottom':
          cp2.y += dist
          break
      }

      return {
        id: edge.id,
        path: `M ${start.x} ${start.y} C ${cp1.x} ${cp1.y}, ${cp2.x} ${cp2.y}, ${end.x} ${end.y}`,
        color: edge.color,
      }
    })
    .filter((e) => e !== null)
})

const canvasDimensions = computed(() => {
  // Ensure SVG covers the entire utilized area
  return {
    width: Math.max(bounds.value.width + Math.abs(bounds.value.minX), 5000),
    height: Math.max(bounds.value.height + Math.abs(bounds.value.minY), 5000),
  }
})

// Helper: Calculate anchor point on a node
function getAnchorPoint(node, side) {
  switch (side) {
    case 'top':
      return { x: node.x + node.width / 2, y: node.y }
    case 'bottom':
      return { x: node.x + node.width / 2, y: node.y + node.height }
    case 'left':
      return { x: node.x, y: node.y + node.height / 2 }
    case 'right':
      return { x: node.x + node.width, y: node.y + node.height / 2 }
    default:
      return { x: node.x, y: node.y }
  }
}

// --- Interaction Handlers ---

const startPan = (e) => {
  isDragging.value = true
  lastMousePos.value = { x: e.clientX, y: e.clientY }
  document.addEventListener('mousemove', handlePan)
  document.addEventListener('mouseup', endPan)
}

const handlePan = (e) => {
  if (!isDragging.value) return
  const dx = e.clientX - lastMousePos.value.x
  const dy = e.clientY - lastMousePos.value.y
  pan.value.x += dx
  pan.value.y += dy
  lastMousePos.value = { x: e.clientX, y: e.clientY }
}

const endPan = () => {
  isDragging.value = false
  document.removeEventListener('mousemove', handlePan)
  document.removeEventListener('mouseup', endPan)
}

const handleWheel = (e) => {
  e.preventDefault()
  const zoomSensitivity = 0.001
  const newScale = Math.min(Math.max(0.1, scale.value - e.deltaY * zoomSensitivity), 3)
  scale.value = newScale
}

const zoomIn = () => (scale.value = Math.min(scale.value + 0.1, 3))
const zoomOut = () => (scale.value = Math.max(scale.value - 0.1, 0.1))

// --- Rendering Helpers ---

const getNodeColorClass = (color) => {
  if (!color) return ''
  // If it's a numeric string (Obsidian index)
  if (['1', '2', '3', '4', '5', '6'].includes(color)) return `color-${color}`
  return ''
}

const getColor = (color) => {
  const map = {
    1: '#ff5555',
    2: '#ffb86c',
    3: '#f1fa8c',
    4: '#50fa7b',
    5: '#8be9fd',
    6: '#bd93f9',
  }
  return map[color] || color
}

const getFileName = (path) => {
  if (!path) return 'Unknown File'
  return path.split('/').pop()
}

const isImage = (path) => {
  return /\.(png|jpg|jpeg|gif|svg|webp)$/i.test(path)
}

const isYoutube = (url) => {
  return url && (url.includes('youtube.com') || url.includes('youtu.be'))
}

const getYoutubeEmbed = (url) => {
  let videoId = ''
  if (url.includes('youtu.be')) {
    videoId = url.split('/').pop()
  } else if (url.includes('youtube.com')) {
    const params = new URLSearchParams(url.split('?')[1])
    videoId = params.get('v')
  }
  return `https://www.youtube.com/embed/${videoId}`
}
</script>

<style scoped>
/* Custom Scrollbar */
::-webkit-scrollbar {
  width: 8px;
  height: 8px;
}
::-webkit-scrollbar-track {
  background: #2f2f2f;
}
::-webkit-scrollbar-thumb {
  background: #555;
  border-radius: 4px;
}
::-webkit-scrollbar-thumb:hover {
  background: #777;
}

.canvas-viewer {
  position: relative;
  background-color: #1e1e1e;
  color: #dcddde;
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif;
}

.loading-state,
.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 100%;
  width: 100%;
  text-align: center;
  color: #dcddde;
}

.canvas-container {
  cursor: grab;
  transform-origin: 0 0;
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
}

.canvas-container:active {
  cursor: grabbing;
}

.node {
  position: absolute;
  box-sizing: border-box;
  border-radius: 8px;
  transition: box-shadow 0.2s;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

/* Node Types */
.node-group {
  background-color: rgba(255, 255, 255, 0.03);
  border: 2px solid rgba(255, 255, 255, 0.1);
  pointer-events: none;
}

.node-group .group-label {
  position: absolute;
  top: -30px;
  left: 0;
  font-weight: 600;
  font-size: 1.2rem;
  color: #999;
  pointer-events: auto;
}

.node-text {
  background-color: #2b2b2b;
  border: 1px solid #444;
  padding: 1rem;
  box-shadow: 0 4px 6px rgba(0, 0, 0, 0.3);
}

.node-file {
  background-color: #2b2b2b;
  border: 1px solid #444;
}

.node-link {
  background-color: #2b2b2b;
  border: 1px solid #444;
}

.file-header {
  padding: 8px 12px;
  background: rgba(255, 255, 255, 0.05);
  border-bottom: 1px solid #444;
  font-size: 0.8rem;
  color: #999;
  display: flex;
  align-items: center;
  gap: 8px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.file-content {
  padding: 1rem;
  font-size: 0.9rem;
  color: #ccc;
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100%;
  flex-direction: column;
}

/* Edges */
svg {
  position: absolute;
  top: 0;
  left: 0;
  pointer-events: none;
  overflow: visible;
  width: 100%;
  height: 100%;
}

path {
  fill: none;
  stroke-width: 2px;
  transition: stroke 0.2s;
}

/* Obsidian Colors */
.color-1 {
  --node-color: #ff5555;
} /* Red */
.color-2 {
  --node-color: #ffb86c;
} /* Orange */
.color-3 {
  --node-color: #f1fa8c;
} /* Yellow */
.color-4 {
  --node-color: #50fa7b;
} /* Green */
.color-5 {
  --node-color: #8be9fd;
} /* Cyan */
.color-6 {
  --node-color: #bd93f9;
} /* Purple */

.node-text[class*='color-'] {
  background-color: color-mix(in srgb, var(--node-color) 20%, #2b2b2b);
  border-color: var(--node-color);
}

.node-group[class*='color-'] {
  background-color: color-mix(in srgb, var(--node-color) 5%, transparent);
  border-color: var(--node-color);
}

.node-group[class*='color-'] .group-label {
  color: var(--node-color);
}

.controls {
  position: fixed;
  bottom: 20px;
  right: 20px;
  background: #333;
  padding: 8px;
  border-radius: 8px;
  display: flex;
  gap: 8px;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.5);
  z-index: 1000;
}

.controls button {
  width: 36px;
  height: 36px;
  background: #444;
  border: none;
  color: white;
  border-radius: 4px;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: background-color 0.2s;
}

.controls button:hover {
  background: #555;
}

.controls button:active {
  background: #666;
}

/* Prose styling for text nodes */
.prose {
  line-height: 1.6;
}

.prose-invert {
  color: #dcddde;
}

.prose-sm {
  font-size: 0.875rem;
}
</style>
