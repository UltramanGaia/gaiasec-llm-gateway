<script setup>
import { ref, computed } from 'vue';
import { View, Document } from '@element-plus/icons-vue';
import VueJsonPretty from 'vue-json-pretty';
import 'vue-json-pretty/lib/styles.css';
import { ElMessage } from 'element-plus';
import ConversationViewer from '@/components/llm-logs/ConversationViewer.vue';
import { formatDateTime } from '@/utils/format.js';

const props = defineProps({ modelValue: Boolean, log: Object });
const emit = defineEmits(['update:modelValue']);

const viewMode = ref('visual');

const dialogVisible = computed({
  get: () => props.modelValue,
  set: (value) => emit('update:modelValue', value),
});

const parseJson = (jsonString) => {
  if (!jsonString) return null;
  try {
    return JSON.parse(jsonString);
  } catch {
    return jsonString;
  }
};

const copyToClipboard = (type) => {
  const content = type === 'request' ? props.log?.request : props.log?.response;
  const formatted = typeof content === 'string' ? content : JSON.stringify(content, null, 2);
  navigator.clipboard.writeText(formatted || '').then(() => ElMessage.success('Copied to clipboard'));
};

const close = () => {
  dialogVisible.value = false;
};

const formatMetric = (value) => {
  if (!value) return '-';
  return `${Math.round(value)}ms`;
};

const semanticEntries = computed(() => {
  const semantic = props.log?.semantic || {};
  return [
    { label: 'Protocol', value: semantic.protocol || '-' },
    { label: 'Status', value: semantic.status || '-' },
    { label: 'Finish', value: semantic.finish_reason || '-' },
    { label: 'Output Types', value: (semantic.output_item_types || []).join(', ') || '-' },
    { label: 'Tool Types', value: (semantic.tool_types || []).join(', ') || '-' },
    { label: 'Tool Names', value: (semantic.tool_names || []).join(', ') || '-' },
    { label: 'Reasoning', value: semantic.reasoning_summary || '-' },
    { label: 'Refusal', value: semantic.refusal || (semantic.has_refusal ? 'yes' : '-') },
    { label: 'Annotations', value: semantic.annotation_count ?? 0 },
    { label: 'Audio', value: semantic.has_audio ? 'yes' : 'no' },
  ];
});
</script>

<template>
  <el-dialog
    title="Log Details"
    v-model="dialogVisible"
    width="90%"
    v-if="log"
    :close-on-click-modal="false"
    class="log-details-dialog"
  >
    <div class="log-details">
      <div class="log-header">
        <div class="log-item">
          <span class="label">ID:</span>
          <span class="value">{{ log.id }}</span>
        </div>
        <div class="log-item">
          <span class="label">Model:</span>
          <el-tag size="small" type="primary">{{ log.modelName }}</el-tag>
        </div>
        <div class="log-item" v-if="log.backendModelName">
          <span class="label">Backend:</span>
          <el-tag size="small" type="warning">{{ log.backendModelName }}</el-tag>
        </div>
        <div class="log-item">
          <span class="label">Created At:</span>
          <span class="value">{{ formatDateTime(log.createdAt) }}</span>
        </div>
        <div class="log-item" v-if="log.responseTime">
          <span class="label">Response Time:</span>
          <el-tag size="small" type="info">{{ log.responseTime }}ms</el-tag>
        </div>
        <div class="log-item">
          <span class="label">TTFT:</span>
          <el-tag size="small" type="warning">{{ formatMetric(log.firstTokenLatency) }}</el-tag>
        </div>
        <div class="log-item">
          <span class="label">Avg Token:</span>
          <el-tag size="small" type="success">{{ formatMetric(log.avgTokenLatency) }}</el-tag>
        </div>
        <div class="log-item">
          <span class="label">并发快照:</span>
          <el-tag size="small" type="success">{{ log.activeRequests || 0 }}</el-tag>
        </div>
        <div class="log-item" v-if="log.backendApiBaseUrl">
          <span class="label">Backend URL:</span>
          <span class="value mono">{{ log.backendApiBaseUrl }}</span>
        </div>
      </div>

      <div class="view-mode-toggle">
        <el-radio-group v-model="viewMode" size="small">
          <el-radio-button value="visual">
            <el-icon><View /></el-icon>
            Visual
          </el-radio-button>
          <el-radio-button value="json">
            <el-icon><Document /></el-icon>
            JSON
          </el-radio-button>
        </el-radio-group>
      </div>

      <div class="semantic-grid" v-if="log.semantic">
        <div class="semantic-item" v-for="entry in semanticEntries" :key="entry.label">
          <span class="label">{{ entry.label }}:</span>
          <span class="value">{{ entry.value }}</span>
        </div>
      </div>

      <template v-if="viewMode === 'visual'">
        <div class="tab-content">
          <ConversationViewer :request="parseJson(log.request)" :response="parseJson(log.response)" />
        </div>
      </template>

      <template v-else>
        <div class="log-content">
          <div class="log-section">
            <div class="section-header">
              <h4>Request</h4>
              <el-button size="small" text @click="copyToClipboard('request')">Copy</el-button>
            </div>
            <vue-json-pretty
              v-if="parseJson(log.request) !== null"
              :data="parseJson(log.request)"
              :expand-depth="2"
              :show-length="true"
              :show-line-number="true"
              :copyable="false"
              theme="dark"
              class="json-content"
            />
            <div v-else class="json-content">Invalid JSON format</div>
          </div>
          <div class="log-section">
            <div class="section-header">
              <h4>Response</h4>
              <el-button size="small" text @click="copyToClipboard('response')">Copy</el-button>
            </div>
            <vue-json-pretty
              v-if="parseJson(log.response) !== null"
              :data="parseJson(log.response)"
              :expand-depth="2"
              :show-length="true"
              :show-line-number="true"
              :copyable="false"
              theme="dark"
              class="json-content"
            />
            <div v-else class="json-content">Invalid JSON format</div>
          </div>
        </div>
      </template>
    </div>
    <template #footer>
      <el-button @click="close">Close</el-button>
    </template>
  </el-dialog>
</template>

<style scoped>
.log-details-dialog :deep(.el-dialog__body) {
  padding: 16px 20px;
  background-color: var(--el-bg-color);
}

.log-details {
  max-height: 85vh;
  overflow-y: auto;
}

.log-header {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: 12px;
  margin-bottom: 16px;
  padding: 16px;
  background: var(--el-fill-color-light);
  border-radius: 8px;
  border: 1px solid var(--el-border-color);
}

.log-item {
  display: flex;
  align-items: center;
  gap: 8px;
}

.log-item .label {
  font-size: 12px;
  color: var(--el-text-color-secondary);
}

.log-item .value {
  font-size: 13px;
  color: var(--el-text-color-primary);
}

.log-item .value.mono {
  font-family: 'SF Mono', Monaco, 'Courier New', monospace;
  font-size: 12px;
}

.view-mode-toggle {
  margin-bottom: 16px;
  display: flex;
  justify-content: flex-end;
}

.semantic-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 10px;
  margin-bottom: 16px;
  padding: 14px 16px;
  background: var(--el-fill-color-lighter);
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 8px;
}

.semantic-item {
  display: flex;
  gap: 6px;
  font-size: 12px;
  line-height: 1.5;
}

.semantic-item .label {
  color: var(--el-text-color-secondary);
}

.semantic-item .value {
  color: var(--el-text-color-primary);
  word-break: break-word;
}

.view-mode-toggle :deep(.el-radio-button__inner) {
  display: flex;
  align-items: center;
  gap: 4px;
}

.tab-content {
  padding: 16px;
  background: var(--el-bg-color);
  border-radius: 8px;
  border: 1px solid var(--el-border-color);
}

.log-content {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 20px;
}

@media (max-width: 1200px) {
  .log-content {
    grid-template-columns: 1fr;
  }
}

.log-section {
  border-radius: 8px;
  border: 1px solid var(--el-border-color);
  overflow: hidden;
  background: var(--el-fill-color-light);
}

.section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  background-color: var(--el-bg-color);
  padding: 10px 15px;
  border-bottom: 1px solid var(--el-border-color);
}

.section-header h4 {
  margin: 0;
  font-size: 14px;
  font-weight: 600;
  color: var(--el-text-color-primary);
}

.json-content {
  background-color: var(--el-bg-color);
  padding: 15px;
  margin: 0;
  overflow-x: auto;
  max-height: 400px;
  font-family: 'SF Mono', 'Fira Code', 'Courier New', Courier, monospace;
  font-size: 13px;
  line-height: 1.5;
  color: var(--el-text-color-primary);
  white-space: pre-wrap;
  word-wrap: break-word;
}

:deep(.vue-json-pretty) {
  font-family: 'SF Mono', 'Fira Code', 'Courier New', Courier, monospace;
  font-size: 13px;
  line-height: 1.5;
}

:deep(.vjp-key),
:deep(.vjp-string) {
  color: #98c379;
}

:deep(.vjp-number) {
  color: #d19a66;
}

:deep(.vjp-boolean) {
  color: #56b6c2;
  font-weight: bold;
}

:deep(.vjp-null) {
  color: #5c6370;
  font-style: italic;
}
</style>
