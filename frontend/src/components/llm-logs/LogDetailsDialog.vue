<template>
  <el-dialog title="Log Details" v-model="dialogVisible" width="90%" v-if="log" :close-on-click-modal="false">
    <div class="log-details">
      <div class="log-header">
        <div class="log-item"><span class="label">ID:</span><span class="value">{{ log.id }}</span></div>
        <div class="log-item"><span class="label">Model:</span><el-tag size="small" type="primary">{{ log.modelName }}</el-tag></div>
        <div class="log-item" v-if="log.backendModelName"><span class="label">Backend:</span><el-tag size="small" type="warning">{{ log.backendModelName }}</el-tag></div>
        <div class="log-item"><span class="label">Created At:</span><span class="value">{{ formatDateTime(log.createdAt) }}</span></div>
        <div class="log-item" v-if="log.responseTime"><span class="label">Response Time:</span><el-tag size="small" type="info">{{ log.responseTime }}ms</el-tag></div>
        <div class="log-item"><span class="label">TTFT:</span><el-tag size="small" type="warning">{{ fmt(log.firstTokenLatency) }}</el-tag></div>
        <div class="log-item"><span class="label">Avg Token:</span><el-tag size="small" type="success">{{ fmt(log.avgTokenLatency) }}</el-tag></div>
        <div class="log-item"><span class="label">并发快照:</span><el-tag size="small" type="success">{{ log.activeRequests || 0 }}</el-tag></div>
        <div class="log-item" v-if="log.backendApiBaseUrl"><span class="label">Backend URL:</span><span class="value mono">{{ log.backendApiBaseUrl }}</span></div>
      </div>

      <div class="view-toggle">
        <el-radio-group v-model="viewMode" size="small">
          <el-radio-button value="json">JSON</el-radio-button>
        </el-radio-group>
      </div>

      <div class="log-content">
        <div class="log-section">
          <div class="section-header">
            <h4>Request</h4>
            <el-button size="small" text @click="copy(log.request)">Copy</el-button>
          </div>
          <vue-json-pretty v-if="parse(log.request)" :data="parse(log.request)" :expand-depth="2" :show-length="true" :show-line-number="true" theme="dark" class="json-content" />
          <div v-else class="json-content">Invalid JSON</div>
        </div>
        <div class="log-section">
          <div class="section-header">
            <h4>Response</h4>
            <el-button size="small" text @click="copy(log.response)">Copy</el-button>
          </div>
          <vue-json-pretty v-if="parse(log.response)" :data="parse(log.response)" :expand-depth="2" :show-length="true" :show-line-number="true" theme="dark" class="json-content" />
          <div v-else class="json-content">Invalid JSON</div>
        </div>
      </div>
    </div>
    <template #footer>
      <el-button @click="dialogVisible = false">Close</el-button>
    </template>
  </el-dialog>
</template>

<script setup>
import { ref, computed } from 'vue';
import VueJsonPretty from 'vue-json-pretty';
import 'vue-json-pretty/lib/styles.css';
import { ElMessage } from 'element-plus';
import { formatDateTime } from '@/utils/format.js';

const props = defineProps({ modelValue: Boolean, log: Object });
const emit = defineEmits(['update:modelValue']);
const viewMode = ref('json');

const dialogVisible = computed({
  get: () => props.modelValue,
  set: (v) => emit('update:modelValue', v),
});

const parse = (s) => {
  if (!s) return null;
  try { return JSON.parse(s); } catch { return null; }
};

const fmt = (v) => v ? `${Math.round(v)}ms` : '-';

const copy = (text) => {
  navigator.clipboard.writeText(text || '').then(() => ElMessage.success('Copied'));
};
</script>

<style scoped>
.log-details { max-height: 85vh; overflow-y: auto; }
.log-header { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 12px; margin-bottom: 16px; padding: 16px; background: var(--el-fill-color-light); border-radius: 8px; border: 1px solid var(--el-border-color); }
.log-item { display: flex; align-items: center; gap: 8px; }
.label { font-size: 12px; color: var(--el-text-color-secondary); }
.value { font-size: 13px; color: var(--el-text-color-primary); }
.value.mono { font-family: monospace; font-size: 12px; }
.view-toggle { margin-bottom: 16px; display: flex; justify-content: flex-end; }
.log-content { display: grid; grid-template-columns: 1fr 1fr; gap: 20px; }
@media (max-width: 1200px) { .log-content { grid-template-columns: 1fr; } }
.log-section { border-radius: 8px; border: 1px solid var(--el-border-color); overflow: hidden; }
.section-header { display: flex; justify-content: space-between; align-items: center; padding: 10px 15px; background: var(--el-fill-color-light); border-bottom: 1px solid var(--el-border-color); }
.section-header h4 { margin: 0; font-size: 14px; font-weight: 600; }
.json-content { padding: 15px; max-height: 400px; overflow-y: auto; font-size: 13px; }
</style>
