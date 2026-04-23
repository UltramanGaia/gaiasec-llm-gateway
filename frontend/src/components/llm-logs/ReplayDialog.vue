<template>
  <el-dialog title="Replay Debug" v-model="dialogVisible" width="95%" v-if="log" :close-on-click-modal="false" top="5vh">
    <div class="replay-container">
      <div class="replay-header">
        <div class="log-item"><span class="label">ID:</span><span class="value">{{ log.id }}</span></div>
        <div class="log-item"><span class="label">Model:</span><el-tag size="small" type="primary">{{ log.modelName }}</el-tag></div>
        <div class="log-item"><span class="label">Created At:</span><span class="value">{{ formatDateTime(log.createdAt) }}</span></div>
      </div>

      <div class="editor-section">
        <div class="editor-header">
          <h4>Request (Editable)</h4>
          <div class="editor-actions">
            <el-button size="small" @click="resetRequest">Reset</el-button>
            <el-button size="small" @click="formatJson">Format JSON</el-button>
          </div>
        </div>
        <div class="codemirror-wrap">
          <codemirror v-model="editableRequest" :style="{ height: '300px' }" :extensions="extensions" :autofocus="true" :indent-with-tab="true" :tab-size="2" />
        </div>
        <div class="replay-actions">
          <el-button type="primary" @click="executeReplay" :loading="replayLoading">
            <el-icon><Refresh /></el-icon>Execute Replay
          </el-button>
        </div>
      </div>

      <div v-if="replayResult" class="replay-result">
        <div class="result-header">
          <h4>Result</h4>
          <el-radio-group v-model="viewMode" size="small">
            <el-radio-button value="compare">Compare</el-radio-button>
            <el-radio-button value="original">Original</el-radio-button>
            <el-radio-button value="new">New</el-radio-button>
          </el-radio-group>
        </div>
        <el-alert v-if="replayResult.error" type="error" :title="replayResult.error" show-icon style="margin-bottom:12px;" />

        <div v-if="viewMode === 'compare'" class="compare-view">
          <div class="compare-section">
            <div class="section-header"><h5>Original</h5><el-tag size="small" type="info" v-if="log.responseTime">{{ log.responseTime }}ms</el-tag></div>
            <div class="response-content">
              <vue-json-pretty v-if="parse(replayResult.originalResponse)" :data="parse(replayResult.originalResponse)" :expand-depth="2" theme="dark" />
              <pre v-else>{{ replayResult.originalResponse }}</pre>
            </div>
          </div>
          <div class="compare-section">
            <div class="section-header"><h5>New Response</h5><el-tag size="small" type="success" v-if="replayResult.responseTime">{{ replayResult.responseTime }}ms</el-tag></div>
            <div class="response-content">
              <vue-json-pretty v-if="parse(replayResult.newResponse)" :data="parse(replayResult.newResponse)" :expand-depth="2" theme="dark" />
              <pre v-else>{{ replayResult.newResponse }}</pre>
            </div>
          </div>
        </div>

        <div v-else class="single-view">
          <vue-json-pretty v-if="parse(viewMode === 'original' ? replayResult.originalResponse : replayResult.newResponse)" :data="parse(viewMode === 'original' ? replayResult.originalResponse : replayResult.newResponse)" :expand-depth="3" theme="dark" />
          <pre v-else>{{ viewMode === 'original' ? replayResult.originalResponse : replayResult.newResponse }}</pre>
        </div>
      </div>
    </div>
    <template #footer>
      <el-button @click="dialogVisible = false">Close</el-button>
    </template>
  </el-dialog>
</template>

<script setup>
import { ref, computed, watch } from 'vue';
import { ElMessage } from 'element-plus';
import { Refresh } from '@element-plus/icons-vue';
import VueJsonPretty from 'vue-json-pretty';
import 'vue-json-pretty/lib/styles.css';
import { Codemirror } from 'vue-codemirror';
import { json } from '@codemirror/lang-json';
import { oneDark } from '@codemirror/theme-one-dark';
import { formatDateTime } from '@/utils/format.js';
import { api } from '@/api/client.js';

const props = defineProps({ modelValue: Boolean, log: Object });
const emit = defineEmits(['update:modelValue']);

const extensions = [json(), oneDark];
const editableRequest = ref('');
const replayResult = ref(null);
const viewMode = ref('compare');
const replayLoading = ref(false);

const dialogVisible = computed({
  get: () => props.modelValue,
  set: (v) => emit('update:modelValue', v),
});

watch(() => props.log, (log) => {
  if (log) { editableRequest.value = log.request; replayResult.value = null; }
});

const parse = (s) => { try { return JSON.parse(s); } catch { return null; } };

const resetRequest = () => { if (props.log) editableRequest.value = props.log.request; };

const formatJson = () => {
  try {
    editableRequest.value = JSON.stringify(JSON.parse(editableRequest.value), null, 2);
    ElMessage.success('JSON formatted');
  } catch { ElMessage.error('Invalid JSON'); }
};

const executeReplay = async () => {
  if (!editableRequest.value) return;
  try {
    let override = {};
    const orig = JSON.parse(props.log.request);
    const modified = JSON.parse(editableRequest.value);
    for (const key of Object.keys(modified)) {
      if (JSON.stringify(orig[key]) !== JSON.stringify(modified[key])) override[key] = modified[key];
    }
    replayLoading.value = true;
    const res = await api.post(`/request-logs/${props.log.id}/replay`, { override });
    replayResult.value = res.data;
    ElMessage.success('Replay completed');
  } catch (e) {
    ElMessage.error('Replay failed: ' + (e.response?.data?.error || e.message));
  } finally {
    replayLoading.value = false;
  }
};
</script>

<style scoped>
.replay-container { max-height: 85vh; overflow-y: auto; }
.replay-header { display: flex; gap: 20px; margin-bottom: 16px; padding: 16px; background: var(--el-fill-color-light); border-radius: 8px; border: 1px solid var(--el-border-color); }
.log-item { display: flex; align-items: center; gap: 8px; }
.label { font-size: 12px; color: var(--el-text-color-secondary); }
.value { font-size: 13px; color: var(--el-text-color-primary); }
.editor-section { margin-bottom: 20px; }
.editor-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 8px; }
.editor-header h4 { margin: 0; font-size: 14px; font-weight: 600; }
.editor-actions { display: flex; gap: 8px; }
.codemirror-wrap { border: 1px solid var(--el-border-color); border-radius: 8px; overflow: hidden; }
.replay-actions { margin-top: 12px; display: flex; justify-content: center; }
.replay-result { margin-top: 20px; border-top: 1px solid var(--el-border-color); padding-top: 16px; }
.result-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px; }
.result-header h4 { margin: 0; font-size: 14px; font-weight: 600; }
.compare-view { display: grid; grid-template-columns: 1fr 1fr; gap: 16px; }
@media (max-width: 1200px) { .compare-view { grid-template-columns: 1fr; } }
.compare-section { border: 1px solid var(--el-border-color); border-radius: 8px; overflow: hidden; }
.section-header { display: flex; justify-content: space-between; align-items: center; padding: 10px 15px; background: var(--el-fill-color-light); border-bottom: 1px solid var(--el-border-color); }
.section-header h5 { margin: 0; font-size: 13px; font-weight: 600; }
.response-content { padding: 12px; max-height: 400px; overflow-y: auto; }
.response-content pre { margin: 0; font-family: monospace; font-size: 12px; white-space: pre-wrap; word-wrap: break-word; }
.single-view { padding: 16px; background: var(--el-fill-color-light); border-radius: 8px; max-height: 500px; overflow-y: auto; }
.single-view pre { margin: 0; font-family: monospace; font-size: 12px; white-space: pre-wrap; word-wrap: break-word; }
</style>
