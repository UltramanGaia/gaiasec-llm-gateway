<script setup>
import { ref, onMounted, reactive, computed } from 'vue';
import { ElTable, ElTableColumn, ElButton, ElInput, ElSelect, ElOption, ElDatePicker, ElPagination, ElMessage, ElDialog, ElForm, ElFormItem } from 'element-plus';
import { logsAPI, modelMappingsAPI } from '../api';
import VueJsonPretty from 'vue-json-pretty';
import 'vue-json-pretty/lib/styles.css';

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

// 模拟日志数据，用于展示效果
const mockLogs = [
  {
    id: 1,
    modelName: 'gpt-3.5-turbo',
    userToken: 'user_123456',
    createdAt: new Date().toISOString(),
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

const fetchLogs = async () => {
  try {
    loading.value = true;
    // 过滤掉空值参数
    const params = { ...filters };
    Object.keys(params).forEach(key => {
      if (!params[key]) delete params[key];
    });

    logs.value = await logsAPI.getLogs(params);
    totalLogs.value = logs.value.length;

    // 如果没有实际数据，使用模拟数据展示效果
    if (logs.value.length === 0) {
      logs.value = mockLogs;
      totalLogs.value = mockLogs.length;
      // 只在没有实际数据时显示提示信息
      ElMessage.info('Showing sample logs for demonstration');
    }
  } catch (error) {
    console.error('Failed to fetch logs:', error);
    // API请求失败时使用模拟数据
    logs.value = mockLogs;
    totalLogs.value = mockLogs.length;
    ElMessage.warning('Using sample logs due to API error');
  } finally {
    loading.value = false;
  }
};

// 格式化JSON字符串，添加错误处理
const parseJson = (jsonString) => {
  if (!jsonString) return null;
  try {
    return JSON.parse(jsonString);
  } catch (error) {
    console.error('Failed to parse JSON:', error);
    return jsonString; // 如果解析失败，返回原始字符串
  }
};

// 复制内容到剪贴板
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

// 格式化时间，只显示日期、时、分、秒
const formatDateTime = (dateTime) => {
  if (!dateTime) return '';
  const date = new Date(dateTime);
  // 获取年、月、日
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, '0');
  const day = String(date.getDate()).padStart(2, '0');
  // 获取时、分、秒
  const hours = String(date.getHours()).padStart(2, '0');
  const minutes = String(date.getMinutes()).padStart(2, '0');
  const seconds = String(date.getSeconds()).padStart(2, '0');
  // 组合成日期时间字符串
  return `${year}-${month}-${day} ${hours}:${minutes}:${seconds}`;
};

// 从请求中提取最后一条消息内容
const getLastMessageContent = (requestString) => {
  if (!requestString) return 'N/A';
  try {
    const request = JSON.parse(requestString);
    
    // 检查是否有messages数组（OpenAI风格）
    if (request.messages && Array.isArray(request.messages) && request.messages.length > 0) {
      // 找到最后一条user或assistant的消息
      const lastUserMessage = request.messages
        .filter(msg => msg.role === 'user')
        .pop();
      const contents = lastUserMessage.content
      const content = contents[0]['text']
      if(content.length > 50) {
        return content.substring(0, 50) + '...';
      }
      return content;
    }
    
    // 检查是否有prompt字段（Claude风格）
    if (request.prompt) {
      return request.prompt;
    }
    
    // 默认返回请求的摘要
    return 'Request content not available';
  } catch (error) {
    console.error('Failed to parse request:', error);
    return 'Invalid request format';
  }
};

const viewLogDetails = (log) => {
  currentLog.value = log;
  dialogVisible.value = true;
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

onMounted(() => {
  fetchLogs();
  fetchModels();
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
        <el-form-item label="User Token">
          <el-input v-model="filters.userToken" placeholder="Input user token"></el-input>
        </el-form-item>
        <el-form-item label="Start Date">
          <el-date-picker v-model="filters.startDate" type="datetime" placeholder="Select start date"></el-date-picker>
        </el-form-item>
        <el-form-item label="End Date">
          <el-date-picker v-model="filters.endDate" type="datetime" placeholder="Select end date"></el-date-picker>
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
      <el-table-column prop="userToken" label="User Token" width="200"></el-table-column>
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
          <!-- <el-tooltip placement="top" :content="getLastMessageContent(row.request)" effect="light">
       
          </el-tooltip> -->
        </template>
      </el-table-column>
      <el-table-column label="Actions" width="100" fixed="right">
        <template #default="{ row }">
          <el-button size="small" @click="viewLogDetails(row)">View</el-button>
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

    <el-dialog title="Log Details" v-model="dialogVisible" width="95%" v-if="currentLog">
      <div class="log-details">
        <div class="log-header">
          <div class="log-item"><strong>ID:</strong> {{ currentLog.id }}</div>
          <div class="log-item"><strong>Model:</strong> {{ currentLog.modelName }}</div>
          <div class="log-item"><strong>User Token:</strong> {{ currentLog.userToken }}</div>
          <div class="log-item"><strong>Created At:</strong> {{ formatDateTime(currentLog.createdAt) }}</div>
        </div>
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
      </div>
      <template #footer>
        <el-button @click="dialogVisible = false">Close</el-button>
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
  grid-template-columns: repeat(2, 1fr);
  gap: 10px;
  margin-bottom: 20px;
  padding-bottom: 10px;
  border-bottom: 1px solid #eee;
}

.log-item {
  margin-bottom: 5px;
}

.log-content {
  display: grid;
  grid-template-columns: 1fr;
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

/* 调整vue-json-pretty的样式以适应我们的布局 */
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

/* 消息内容样式 */
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
</style>