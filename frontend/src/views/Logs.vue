<script setup>
import { ref, onMounted, reactive, computed, onUnmounted, shallowRef } from 'vue';
import { ElTable, ElTableColumn, ElButton, ElInput, ElSelect, ElOption, ElDatePicker, ElPagination, ElMessage, ElDialog, ElForm, ElFormItem, ElSwitch, ElRadioGroup, ElRadioButton, ElTabs, ElTabPane, ElAlert } from 'element-plus';
import { View, Document, ChatDotRound, Cpu, Refresh } from '@element-plus/icons-vue';
import { logsAPI, modelMappingsAPI } from '../api';
import VueJsonPretty from 'vue-json-pretty';
import 'vue-json-pretty/lib/styles.css';
import MessageViewer from '../components/MessageViewer.vue';
import ResponseViewer from '../components/ResponseViewer.vue';
import { Codemirror } from 'vue-codemirror';
import { json } from '@codemirror/lang-json';
import { oneDark } from '@codemirror/theme-one-dark';

const logs = ref([]);
const totalLogs = ref(0);
const loading = ref(false);
const filters = reactive({
  model: '',
  userToken: '',
  startDate: '',
  endDate: '',
  page: 1,
  pageSize: 10
});

const autoRefresh = ref(true);
const refreshInterval = ref(5000);
let refreshTimer = null;

const mockLogs = [
  {
    id: 1,
    modelName: 'gpt-3.5-turbo',
    userToken: 'user_123456',
    createdAt: new Date().toISOString(),
    responseTime: 1250,
    request: JSON.stringify({
      model: 'gpt-3.5-turbo',
      messages: [
        { role: 'system', content: 'You are a helpful assistant.' },
        { role: 'user', content: 'Hello, how are you?' }
      ],
      max_tokens: 100
    }),
    response: JSON.stringify({
      id: 'chatcmpl-123',
      object: 'chat.completion',
      created: Math.floor(Date.now() / 1000),
      model: 'gpt-3.5-turbo',
      choices: [
        {
          index: 0,
          message: {
            role: 'assistant',
            content: 'I am doing well, thank you! How can I assist you today?'
          },
          finish_reason: 'stop'
        }
      ],
      usage: {
        prompt_tokens: 26,
        completion_tokens: 19,
        total_tokens: 45
      }
    })
  },
  {
    id: 2,
    modelName: 'claude-2',
    userToken: 'user_654321',
    createdAt: new Date(Date.now() - 3600000).toISOString(),
    responseTime: 2100,
    request: JSON.stringify({
      model: 'claude-2',
      prompt: 'Explain quantum computing in simple terms',
      max_tokens_to_sample: 200
    }),
    response: JSON.stringify({
      completion: 'Quantum computing is a type of computing that uses quantum mechanical phenomena, such as superposition and entanglement, to perform operations on data. Unlike classical computers that use bits (0s and 1s), quantum computers use quantum bits or qubits. This allows quantum computers to process a vast number of possibilities simultaneously, making them potentially much faster for certain types of problems.',
      stop_reason: 'stop_sequence',
      model: 'claude-2',
      usage: {
        prompt_tokens: 20,
        completion_tokens: 91,
        total_tokens: 111
      }
    })
  }
];
const dialogVisible = ref(false);
const currentLog = ref(null);
const models = ref([]);
const viewMode = ref('visual');
const activeTab = ref('request');

const replayDialogVisible = ref(false);
const replayLoading = ref(false);
const replayLog = ref(null);
const editableRequest = ref('');
const replayResult = ref(null);
const replayViewMode = ref('compare');

const editorExtensions = [json(), oneDark];

const fetchLogs = async () => {
  try {
    loading.value = true;
    const params = { ...filters };
    Object.keys(params).forEach(key => {
      if (!params[key]) delete params[key];
    });

    const resp = await logsAPI.getLogs(params);
    logs.value = resp.logs;
    totalLogs.value = resp.total;

    if (logs.value.length === 0) {
      logs.value = mockLogs;
      totalLogs.value = mockLogs.length;
      ElMessage.info('Showing sample logs for demonstration');
    }
  } catch (error) {
    console.error('Failed to fetch logs:', error);
    logs.value = mockLogs;
    totalLogs.value = mockLogs.length;
    ElMessage.warning('Using sample logs due to API error');
  } finally {
    loading.value = false;
  }
};

const toggleAutoRefresh = (value) => {
  autoRefresh.value = value;
  if (value) {
    startAutoRefresh();
    ElMessage.success(`Auto-refresh enabled (every ${refreshInterval.value / 1000} seconds)`);
  } else {
    stopAutoRefresh();
    ElMessage.info('Auto-refresh disabled');
  }
};

const startAutoRefresh = () => {
  stopAutoRefresh();
  refreshTimer = setInterval(() => {
    fetchLogs();
  }, refreshInterval.value);
};

const stopAutoRefresh = () => {
  if (refreshTimer) {
    clearInterval(refreshTimer);
    refreshTimer = null;
  }
};

const parseJson = (jsonString) => {
  if (!jsonString) return null;
  try {
    return JSON.parse(jsonString);
  } catch (error) {
    console.error('Failed to parse JSON:', error);
    return jsonString;
  }
};

const copyToClipboard = (type) => {
  const content = type === 'request' ? currentLog.value?.request : currentLog.value?.response;
  const formatted = typeof content === 'string' ? content : JSON.stringify(content, null, 2);
  navigator.clipboard.writeText(formatted).then(() => {
    ElMessage.success('Copied to clipboard');
  }).catch(err => {
    console.error('Failed to copy:', err);
    ElMessage.error('Failed to copy');
  });
};

const fetchModels = async () => {
  try {
    const data = await modelMappingsAPI.getModelMappings();
    models.value = data.map(m => m.alias);
  } catch (error) {
    console.error('Failed to fetch models:', error);
  }
};

const formatDateTime = (dateTime) => {
  if (!dateTime) return '';
  const date = new Date(dateTime);
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, '0');
  const day = String(date.getDate()).padStart(2, '0');
  const hours = String(date.getHours()).padStart(2, '0');
  const minutes = String(date.getMinutes()).padStart(2, '0');
  const seconds = String(date.getSeconds()).padStart(2, '0');
  return `${year}-${month}-${day} ${hours}:${minutes}:${seconds}`;
};

const getLastMessageContent = (requestString) => {
  if (!requestString) return 'N/A';
  try {
    const request = JSON.parse(requestString);
    
    if (request.messages && Array.isArray(request.messages) && request.messages.length > 0) {
      const lastUserMessage = request.messages
        .filter(msg => msg.role === 'user')
        .pop();
      const contentRaw = lastUserMessage.content
      let result = '';
    if (Array.isArray(contentRaw)) {
        const validTextItems = contentRaw.filter(item =>
          item?.type === 'text' && typeof item.text === 'string'
        );
        if (validTextItems.length > 0) {
          result = validTextItems[validTextItems.length - 1].text;
        }
      } else if (typeof contentRaw === 'string') {
        result = contentRaw;
      }
      const content = result.replace(/\n/g, '')
      console.log(content)
      if(content.length > 50) {
        return content.substring(0, 50) + '...';
      }
      return content;
    }
    
    if (request.prompt) {
      return request.prompt;
    }
    
    return 'Request content not available';
  } catch (error) {
    console.error('Failed to parse request:', error);
    return 'Invalid request format';
  }
};

const viewLogDetails = (log) => {
  currentLog.value = log;
  dialogVisible.value = true;
  activeTab.value = 'request';
};

const openReplayDialog = (log) => {
  replayLog.value = log;
  editableRequest.value = log.request;
  replayResult.value = null;
  replayViewMode.value = 'compare';
  replayDialogVisible.value = true;
};

const executeReplay = async () => {
  if (!editableRequest.value) {
    ElMessage.warning('Request cannot be empty');
    return;
  }

  try {
    let override = {};
    try {
      const originalRequest = JSON.parse(replayLog.value.request);
      const modifiedRequest = JSON.parse(editableRequest.value);
      
      for (const key of Object.keys(modifiedRequest)) {
        if (JSON.stringify(originalRequest[key]) !== JSON.stringify(modifiedRequest[key])) {
          override[key] = modifiedRequest[key];
        }
      }
    } catch (e) {
      ElMessage.error('Invalid JSON format');
      return;
    }

    replayLoading.value = true;
    const result = await logsAPI.replayLog(replayLog.value.id, override);
    replayResult.value = result;
    ElMessage.success('Replay completed');
  } catch (error) {
    console.error('Replay failed:', error);
    ElMessage.error('Replay failed: ' + (error.response?.data?.error || error.message));
  } finally {
    replayLoading.value = false;
  }
};

const resetReplayRequest = () => {
  if (replayLog.value) {
    editableRequest.value = replayLog.value.request;
  }
};

const formatEditorContent = () => {
  try {
    const obj = JSON.parse(editableRequest.value);
    editableRequest.value = JSON.stringify(obj, null, 2);
    ElMessage.success('JSON formatted');
  } catch (e) {
    ElMessage.error('Invalid JSON format');
  }
};

const formatJsonString = (str) => {
  try {
    const obj = JSON.parse(str);
    return JSON.stringify(obj, null, 2);
  } catch {
    return str;
  }
};

const getResponseContent = (responseStr) => {
  if (!responseStr) return 'No response';
  try {
    const resp = JSON.parse(responseStr);
    if (resp.choices && resp.choices[0]) {
      return resp.choices[0].message?.content || resp.choices[0].delta?.content || 'No content';
    }
    return responseStr;
  } catch {
    return responseStr;
  }
};

const resetFilters = () => {
  Object.keys(filters).forEach(key => {
    if (key !== 'page' && key !== 'pageSize') {
      filters[key] = '';
    }
  });
  filters.page = 1;
  fetchLogs();
};

const handleSizeChange = (size) => {
  filters.pageSize = size;
  filters.page = 1;
  fetchLogs();
};

const handleCurrentChange = (current) => {
  filters.page = current;
  fetchLogs();
};

const hasTools = (requestString) => {
  try {
    const data = JSON.parse(requestString);
    return data.tools && data.tools.length > 0;
  } catch {
    return false;
  }
};

const getToolCount = (requestString) => {
  try {
    const data = JSON.parse(requestString);
    return data.tools?.length || 0;
  } catch {
    return 0;
  }
};

const getMessageCount = (requestString) => {
  try {
    const data = JSON.parse(requestString);
    return data.messages?.length || (data.prompt ? 1 : 0);
  } catch {
    return 0;
  }
};

onMounted(() => {
  fetchLogs();
  fetchModels();
});

onUnmounted(() => {
  stopAutoRefresh();
});
</script>

<template>
  <div class="logs">
    <div class="filter-form">
      <el-form :model="filters" inline>
        <el-form-item label="Model">
          <el-select v-model="filters.model" placeholder="Select model">
            <el-option v-for="model in models" :key="model" :label="model" :value="model"></el-option>
          </el-select>
        </el-form-item>
        <el-form-item label="Start Date">
          <el-date-picker v-model="filters.startDate" type="datetime" placeholder="Select start date"></el-date-picker>
        </el-form-item>
        <el-form-item label="End Date">
          <el-date-picker v-model="filters.endDate" type="datetime" placeholder="Select end date"></el-date-picker>
        </el-form-item>
        <el-form-item label="Auto Refresh" style="margin-left: auto;">
          <el-switch v-model="autoRefresh" @change="toggleAutoRefresh"></el-switch>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="fetchLogs">Search</el-button>
          <el-button @click="resetFilters">Reset</el-button>
        </el-form-item>
      </el-form>
    </div>

    <el-table :data="logs" style="width: 100%" :loading="loading">
      <el-table-column prop="id" label="ID" width="80"></el-table-column>
      <el-table-column prop="modelName" label="Model" width="150"></el-table-column>
      <el-table-column prop="createdAt" label="Created At" width="180">
        <template #default="{ row }">
          <span>{{ formatDateTime(row.createdAt) }}</span>
        </template>
      </el-table-column>
      <el-table-column label="Message Content" min-width="200">
        <template #default="{ row }">
          <div class="message-content">
            {{ getLastMessageContent(row.request).length > 50 ? 
                getLastMessageContent(row.request).substring(0, 50) + '...' : 
                getLastMessageContent(row.request) }}
          </div>
        </template>
      </el-table-column>
      <el-table-column label="Info" width="140" align="center">
        <template #default="{ row }">
          <div class="info-badges">
            <el-tooltip content="Messages" placement="top">
              <el-tag size="small" type="success" class="info-tag">
                <el-icon><ChatDotRound /></el-icon>
                {{ getMessageCount(row.request) }}
              </el-tag>
            </el-tooltip>
            <el-tooltip v-if="hasTools(row.request)" content="Tools" placement="top">
              <el-tag size="small" type="warning" class="info-tag">
                <el-icon><Cpu /></el-icon>
                {{ getToolCount(row.request) }}
              </el-tag>
            </el-tooltip>
          </div>
        </template>
      </el-table-column>
      <el-table-column label="Actions" width="160" fixed="right">
        <template #default="{ row }">
          <el-button size="small" @click="viewLogDetails(row)">View</el-button>
          <el-button size="small" type="warning" @click="openReplayDialog(row)">
            <el-icon><Refresh /></el-icon>
          </el-button>
        </template>
      </el-table-column>
    </el-table>

    <div class="pagination-container">
      <el-pagination
        v-model:current-page="filters.page"
        v-model:page-size="filters.pageSize"
        :page-sizes="[10, 20, 50, 100]"
        layout="total, sizes, prev, pager, next, jumper"
        :total="totalLogs"
        @size-change="handleSizeChange"
        @current-change="handleCurrentChange"
      ></el-pagination>
    </div>

    <el-dialog 
      title="Log Details" 
      v-model="dialogVisible" 
      width="90%" 
      v-if="currentLog"
      :close-on-click-modal="false"
    >
      <div class="log-details">
        <div class="log-header">
          <div class="log-item"><strong>ID:</strong> {{ currentLog.id }}</div>
          <div class="log-item"><strong>Model:</strong> {{ currentLog.modelName }}</div>
          <div class="log-item"><strong>Created At:</strong> {{ formatDateTime(currentLog.createdAt) }}</div>
          <div class="log-item" v-if="currentLog.responseTime">
            <strong>Response Time:</strong> 
            <el-tag size="small" type="info">{{ currentLog.responseTime }}ms</el-tag>
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

        <template v-if="viewMode === 'visual'">
          <el-tabs v-model="activeTab" class="log-tabs">
            <el-tab-pane label="Request" name="request">
              <div class="tab-content">
                <MessageViewer :data="parseJson(currentLog.request)" />
              </div>
            </el-tab-pane>
            <el-tab-pane label="Response" name="response">
              <div class="tab-content">
                <ResponseViewer 
                  :data="parseJson(currentLog.response)" 
                  :response-time="currentLog.responseTime"
                />
              </div>
            </el-tab-pane>
          </el-tabs>
        </template>

        <template v-else>
          <div class="log-content">
            <div class="log-section">
              <div class="section-header">
                <h4>Request</h4>
                <el-button size="small" type="text" @click="copyToClipboard('request')">
                  Copy
                </el-button>
              </div>
              <vue-json-pretty 
                v-if="parseJson(currentLog.request) !== null" 
                :data="parseJson(currentLog.request)"
                :expand-depth="2"
                :show-length="true"
                :show-line-number="true"
                :copyable="false"
                theme="light"
                class="json-content"
              />
              <div v-else class="json-content">Invalid JSON format</div>
            </div>
            <div class="log-section">
              <div class="section-header">
                <h4>Response</h4>
                <el-button size="small" type="text" @click="copyToClipboard('response')">
                  Copy
                </el-button>
              </div>
              <vue-json-pretty 
                v-if="parseJson(currentLog.response) !== null" 
                :data="parseJson(currentLog.response)"
                :expand-depth="2"
                :show-length="true"
                :show-line-number="true"
                :copyable="false"
                theme="light"
                class="json-content"
              />
              <div v-else class="json-content">Invalid JSON format</div>
            </div>
          </div>
        </template>
      </div>
      <template #footer>
        <el-button @click="dialogVisible = false">Close</el-button>
      </template>
    </el-dialog>

    <el-dialog 
      title="Replay Debug" 
      v-model="replayDialogVisible" 
      width="95%" 
      v-if="replayLog"
      :close-on-click-modal="false"
      top="5vh"
    >
      <div class="replay-container">
        <div class="replay-header">
          <div class="log-item"><strong>ID:</strong> {{ replayLog.id }}</div>
          <div class="log-item"><strong>Model:</strong> {{ replayLog.modelName }}</div>
          <div class="log-item"><strong>Created At:</strong> {{ formatDateTime(replayLog.createdAt) }}</div>
        </div>

        <div class="replay-editor">
          <div class="editor-header">
            <h4>Request (Editable)</h4>
            <div class="editor-actions">
              <el-button size="small" @click="resetReplayRequest">Reset</el-button>
              <el-button size="small" @click="formatEditorContent">Format JSON</el-button>
            </div>
          </div>
          <div class="codemirror-container">
            <codemirror
              v-model="editableRequest"
              :style="{ height: '300px' }"
              :extensions="editorExtensions"
              :autofocus="true"
              :indent-with-tab="true"
              :tab-size="2"
            />
          </div>
          <div class="replay-actions">
            <el-button type="primary" @click="executeReplay" :loading="replayLoading">
              <el-icon><Refresh /></el-icon>
              Execute Replay
            </el-button>
          </div>
        </div>

        <div v-if="replayResult" class="replay-result">
          <div class="result-header">
            <h4>Result</h4>
            <el-radio-group v-model="replayViewMode" size="small">
              <el-radio-button value="compare">Compare</el-radio-button>
              <el-radio-button value="original">Original</el-radio-button>
              <el-radio-button value="new">New Response</el-radio-button>
            </el-radio-group>
          </div>

          <div v-if="replayResult.error" class="error-message">
            <el-alert type="error" :title="replayResult.error" show-icon />
          </div>

          <div v-if="replayViewMode === 'compare'" class="compare-view">
            <div class="compare-section">
              <div class="section-header">
                <h5>Original Response</h5>
                <el-tag size="small" type="info" v-if="replayLog.responseTime">{{ replayLog.responseTime }}ms</el-tag>
              </div>
              <div class="response-content">
                <vue-json-pretty 
                  v-if="parseJson(replayResult.originalResponse)" 
                  :data="parseJson(replayResult.originalResponse)"
                  :expand-depth="2"
                  :show-length="true"
                  theme="light"
                />
                <pre v-else>{{ replayResult.originalResponse }}</pre>
              </div>
            </div>
            <div class="compare-section">
              <div class="section-header">
                <h5>New Response</h5>
                <el-tag size="small" type="success" v-if="replayResult.responseTime">{{ replayResult.responseTime }}ms</el-tag>
              </div>
              <div class="response-content">
                <vue-json-pretty 
                  v-if="parseJson(replayResult.newResponse)" 
                  :data="parseJson(replayResult.newResponse)"
                  :expand-depth="2"
                  :show-length="true"
                  theme="light"
                />
                <pre v-else>{{ replayResult.newResponse }}</pre>
              </div>
            </div>
          </div>

          <div v-else-if="replayViewMode === 'original'" class="single-view">
            <vue-json-pretty 
              v-if="parseJson(replayResult.originalResponse)" 
              :data="parseJson(replayResult.originalResponse)"
              :expand-depth="3"
              :show-length="true"
              theme="light"
            />
            <pre v-else>{{ replayResult.originalResponse }}</pre>
          </div>

          <div v-else class="single-view">
            <vue-json-pretty 
              v-if="parseJson(replayResult.newResponse)" 
              :data="parseJson(replayResult.newResponse)"
              :expand-depth="3"
              :show-length="true"
              theme="light"
            />
            <pre v-else>{{ replayResult.newResponse }}</pre>
          </div>

          <div v-if="replayResult.actualModelName" class="result-info">
            <el-tag size="small">Actual Model: {{ replayResult.actualModelName }}</el-tag>
          </div>
        </div>
      </div>
      <template #footer>
        <el-button @click="replayDialogVisible = false">Close</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.logs {
  margin: 0;
  padding: 20px;
  width: 100%;
}

.filter-form {
  margin-bottom: 20px;
  padding: 15px;
  background-color: #f5f5f5;
  border-radius: 4px;
}

.pagination-container {
  margin-top: 20px;
  display: flex;
  justify-content: flex-end;
}

.log-details {
  max-height: 85vh;
  overflow-y: auto;
}

.log-header {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: 10px;
  margin-bottom: 16px;
  padding-bottom: 12px;
  border-bottom: 1px solid #eee;
}

.log-item {
  margin-bottom: 5px;
}

.view-mode-toggle {
  margin-bottom: 16px;
  display: flex;
  justify-content: flex-end;
}

.view-mode-toggle .el-radio-button :deep(.el-radio-button__inner) {
  display: flex;
  align-items: center;
  gap: 4px;
}

.log-tabs {
  min-height: 400px;
}

.tab-content {
  padding: 16px;
  background: #fff;
  border-radius: 8px;
  border: 1px solid #ebeef5;
}

.log-content {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 20px;
}

.log-section {
  border-radius: 4px;
  border: 1px solid #eee;
  overflow: hidden;
}

.section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  background-color: #f9f9f9;
  padding: 10px 15px;
  border-bottom: 1px solid #eee;
}

.section-header h4 {
  margin: 0;
  font-size: 14px;
  font-weight: 600;
}

.json-content {
  background-color: #f5f5f5;
  padding: 15px;
  margin: 0;
  overflow-x: auto;
  max-height: 400px;
  font-family: 'Courier New', Courier, monospace;
  font-size: 13px;
  line-height: 1.5;
  color: #333;
  white-space: pre-wrap;
  word-wrap: break-word;
}

:deep(.vue-json-pretty) {
  font-family: 'Courier New', Courier, monospace;
  font-size: 13px;
  line-height: 1.5;
  color: #333;
}

:deep(.vjp-key) {
  color: #a52a2a;
  font-weight: bold;
}

:deep(.vjp-string) {
  color: #008000;
}

:deep(.vjp-number) {
  color: #0000ff;
}

:deep(.vjp-boolean) {
  color: #b22222;
  font-weight: bold;
}

:deep(.vjp-null) {
  color: #808080;
  font-style: italic;
}

.message-content {
  color: #666;
  font-size: 14px;
  line-height: 1.4;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  cursor: pointer;
  transition: color 0.2s;
}

.message-content:hover {
  color: #409eff;
}

.info-badges {
  display: flex;
  gap: 4px;
  justify-content: center;
  flex-wrap: wrap;
}

.info-tag {
  display: flex;
  align-items: center;
  gap: 2px;
}

.info-tag .el-icon {
  font-size: 12px;
}

.replay-container {
  max-height: 85vh;
  overflow-y: auto;
}

.replay-header {
  display: flex;
  gap: 20px;
  margin-bottom: 16px;
  padding-bottom: 12px;
  border-bottom: 1px solid #eee;
}

.replay-editor {
  margin-bottom: 20px;
}

.editor-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
}

.editor-header h4 {
  margin: 0;
  font-size: 14px;
  font-weight: 600;
}

.editor-actions {
  display: flex;
  gap: 8px;
}

.codemirror-container {
  border: 1px solid #dcdfe6;
  border-radius: 4px;
  overflow: hidden;
  text-align: left;
}

.codemirror-container :deep(.cm-editor) {
  font-size: 13px;
}

.codemirror-container :deep(.cm-scroller) {
  font-family: 'Fira Code', 'Consolas', 'Monaco', monospace;
}

.replay-actions {
  margin-top: 12px;
  display: flex;
  justify-content: center;
}

.replay-result {
  margin-top: 20px;
  border-top: 1px solid #eee;
  padding-top: 16px;
}

.result-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
}

.result-header h4 {
  margin: 0;
  font-size: 14px;
  font-weight: 600;
}

.error-message {
  margin-bottom: 16px;
}

.compare-view {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
}

.compare-section {
  border: 1px solid #eee;
  border-radius: 8px;
  overflow: hidden;
}

.compare-section .section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 10px 15px;
  background-color: #f9f9f9;
  border-bottom: 1px solid #eee;
}

.compare-section .section-header h5 {
  margin: 0;
  font-size: 13px;
  font-weight: 600;
}

.response-content {
  padding: 12px;
  max-height: 400px;
  overflow-y: auto;
  background-color: #fafafa;
}

.response-content pre {
  margin: 0;
  font-family: 'Courier New', Courier, monospace;
  font-size: 12px;
  white-space: pre-wrap;
  word-wrap: break-word;
}

.single-view {
  padding: 16px;
  background-color: #fafafa;
  border-radius: 8px;
  max-height: 500px;
  overflow-y: auto;
}

.result-info {
  margin-top: 12px;
  text-align: right;
}
</style>
