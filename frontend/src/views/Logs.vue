<template>
  <Layout>
    <div class="logs">
      <el-card class="logs-card">
        <template #header>
          <div class="card-header">
            <span class="title">LLM 请求日志</span>
            <div class="header-actions">
              <el-switch v-model="autoRefresh" @change="toggleAutoRefresh" />
              <span class="auto-refresh-label">自动刷新</span>
            </div>
          </div>
        </template>

        <div class="filter-form">
          <el-form :model="filters" inline>
            <el-form-item label="Model">
              <el-select v-model="filters.model" placeholder="Select model" clearable>
                <el-option v-for="m in models" :key="m" :label="m" :value="m" />
              </el-select>
            </el-form-item>
            <el-form-item label="Backend">
              <el-select v-model="filters.backendModel" placeholder="Select backend" clearable>
                <el-option v-for="m in backendModels" :key="m" :label="m" :value="m" />
              </el-select>
            </el-form-item>
            <el-form-item label="Start Date">
              <el-date-picker v-model="filters.startDate" type="datetime" placeholder="Select start date" />
            </el-form-item>
            <el-form-item label="End Date">
              <el-date-picker v-model="filters.endDate" type="datetime" placeholder="Select end date" />
            </el-form-item>
            <el-form-item>
              <el-button type="primary" @click="fetchLogs">Search</el-button>
              <el-button @click="resetFilters">Reset</el-button>
            </el-form-item>
          </el-form>
        </div>

        <div class="summary-grid" v-if="summary">
          <div class="summary-card"><div class="summary-label">总请求数</div><div class="summary-value">{{ summary.totalRequests || 0 }}</div></div>
          <div class="summary-card"><div class="summary-label">平均响应延迟</div><div class="summary-value">{{ formatLatency(summary.avgResponseTime) }}</div></div>
          <div class="summary-card"><div class="summary-label">平均首Token延迟</div><div class="summary-value">{{ formatLatency(summary.avgFirstTokenLatency) }}</div></div>
          <div class="summary-card"><div class="summary-label">平均Token间延迟</div><div class="summary-value">{{ formatLatency(summary.avgTokenLatency) }}</div></div>
          <div class="summary-card"><div class="summary-label">当前活跃连接</div><div class="summary-value">{{ summary.activeRequests || 0 }}</div></div>
        </div>

        <el-table :data="logs" style="width: 100%" v-loading="loading">
          <el-table-column prop="id" label="ID" width="80" />
          <el-table-column prop="modelName" label="Model" width="150" />
          <el-table-column prop="backendModelName" label="Backend" width="170" show-overflow-tooltip />
          <el-table-column prop="createdAt" label="Created At" width="180">
            <template #default="{ row }">{{ formatDateTime(row.createdAt) }}</template>
          </el-table-column>
          <el-table-column label="Message Content" min-width="200">
            <template #default="{ row }">
              <div class="message-content">{{ getLastMessage(row) }}</div>
            </template>
          </el-table-column>
          <el-table-column label="Response Time" width="120" align="center">
            <template #default="{ row }">
              <el-tag size="small" type="info" v-if="row.responseTime">{{ row.responseTime }}ms</el-tag>
              <span v-else>-</span>
            </template>
          </el-table-column>
          <el-table-column label="首Token" width="100" align="center">
            <template #default="{ row }">
              <el-tag size="small" type="warning" v-if="row.firstTokenLatency">{{ row.firstTokenLatency }}ms</el-tag>
              <span v-else>-</span>
            </template>
          </el-table-column>
          <el-table-column label="平均Token" width="110" align="center">
            <template #default="{ row }">
              <el-tag size="small" type="primary" v-if="row.avgTokenLatency">{{ Math.round(row.avgTokenLatency) }}ms</el-tag>
              <span v-else>-</span>
            </template>
          </el-table-column>
          <el-table-column label="并发快照" width="100" align="center">
            <template #default="{ row }">
              <el-tag size="small" type="success">{{ row.activeRequests || 0 }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="Actions" width="140" fixed="right">
            <template #default="{ row }">
              <el-button size="small" @click="viewLog(row)">View</el-button>
              <el-button size="small" type="warning" @click="openReplay(row)">
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
            @size-change="(s) => { filters.pageSize = s; filters.page = 1; fetchLogs(); }"
            @current-change="(p) => { filters.page = p; fetchLogs(); }"
          />
        </div>
      </el-card>

      <LogDetailsDialog v-model="detailVisible" :log="currentLog" />
      <ReplayDialog v-model="replayVisible" :log="replayLog" />
    </div>
  </Layout>
</template>

<script setup>
import { ref, reactive, onMounted, onUnmounted } from 'vue';
import { ElMessage } from 'element-plus';
import { Refresh } from '@element-plus/icons-vue';
import Layout from '@/components/Layout.vue';
import LogDetailsDialog from '@/components/llm-logs/LogDetailsDialog.vue';
import ReplayDialog from '@/components/llm-logs/ReplayDialog.vue';
import { api } from '@/api/client.js';
import { formatDateTime, formatLatency } from '@/utils/format.js';

const logs = ref([]);
const totalLogs = ref(0);
const loading = ref(false);
const summary = ref(null);
const models = ref([]);
const backendModels = ref([]);
const autoRefresh = ref(true);
let refreshTimer = null;
let logsInFlight = false;
let summaryInFlight = false;

const filters = reactive({ model: '', backendModel: '', startDate: '', endDate: '', page: 1, pageSize: 20 });

const detailVisible = ref(false);
const currentLog = ref(null);
const replayVisible = ref(false);
const replayLog = ref(null);

const normalizeLog = (log = {}) => ({
  ...log,
  modelName: log.model_name ?? log.modelName ?? '',
  backendModelName: log.backend_model_name ?? log.backendModelName ?? '',
  createdAt: log.created_at ?? log.createdAt ?? '',
  responseTime: log.response_time ?? log.responseTime ?? 0,
  firstTokenLatency: log.first_token_latency ?? log.firstTokenLatency ?? 0,
  avgTokenLatency: log.avg_token_latency ?? log.avgTokenLatency ?? 0,
  activeRequests: log.active_requests ?? log.activeRequests ?? 0,
  requestPreview: log.request_preview ?? log.requestPreview ?? '',
  requestBytes: log.request_bytes ?? log.requestBytes ?? 0,
  responseBytes: log.response_bytes ?? log.responseBytes ?? 0,
  streamBytes: log.stream_bytes ?? log.streamBytes ?? 0,
});

const normalizeSummary = (data = {}) => ({
  totalRequests: data.total_requests ?? data.totalRequests ?? 0,
  avgResponseTime: data.avg_response_time ?? data.avgResponseTime ?? 0,
  avgFirstTokenLatency: data.avg_first_token_latency ?? data.avgFirstTokenLatency ?? 0,
  avgTokenLatency: data.avg_token_latency ?? data.avgTokenLatency ?? 0,
  activeRequests: data.active_requests ?? data.activeRequests ?? 0,
});

const normalizeModelConfig = (config = {}) => ({
  ...config,
  modelName: config.model_name ?? config.modelName ?? '',
});

const fetchLogs = async () => {
  if (logsInFlight) return;
  logsInFlight = true;
  loading.value = true;
  try {
    const params = {};
    if (filters.page) params.page = filters.page;
    if (filters.pageSize) params.page_size = filters.pageSize;
    if (filters.model) params.model = filters.model;
    if (filters.backendModel) params.backend_model = filters.backendModel;
    if (filters.startDate) params.start_date = filters.startDate;
    if (filters.endDate) params.end_date = filters.endDate;
    const res = await api.get('/request-logs', { params });
    const data = res.data || {};
    logs.value = (data.records || data.logs || []).map(log => normalizeLog(log));
    totalLogs.value = data.total || 0;
  } catch {
    ElMessage.error('获取日志列表失败');
  } finally {
    loading.value = false;
    logsInFlight = false;
  }
};

const fetchSummary = async () => {
  if (summaryInFlight) return;
  summaryInFlight = true;
  try {
    const res = await api.get('/stats');
    summary.value = res.data ? normalizeSummary(res.data) : null;
  } catch {
  } finally {
    summaryInFlight = false;
  }
};

const fetchModels = async () => {
  try {
    const res = await api.get('/model-configs');
    const data = Array.isArray(res.data) ? res.data.map(config => normalizeModelConfig(config)) : [];
    const backendSet = new Set();
    models.value = data
      .filter(c => c.enabled !== false && c.name)
      .map(c => { if (c.modelName) backendSet.add(c.modelName); return c.name; })
      .sort((a, b) => a.localeCompare(b));
    backendModels.value = Array.from(backendSet).sort((a, b) => a.localeCompare(b));
  } catch {}
};

const resetFilters = () => {
  filters.model = ''; filters.backendModel = ''; filters.startDate = ''; filters.endDate = '';
  filters.page = 1;
  fetchLogs();
};

const loadLogDetail = async (log) => {
  if (log.request || log.response || log.streamResponse) return log;
  const res = await api.get(`/request-logs/${log.id}`);
  return normalizeLog(res.data || {});
};

const viewLog = async (log) => {
  try {
    currentLog.value = await loadLogDetail(log);
    detailVisible.value = true;
  } catch {
    ElMessage.error('获取日志详情失败');
  }
};

const openReplay = async (log) => {
  try {
    replayLog.value = await loadLogDetail(log);
    replayVisible.value = true;
  } catch {
    ElMessage.error('获取日志详情失败');
  }
};

const getLastMessage = (row) => {
  if (row.requestPreview) return row.requestPreview;
  const requestStr = row.request;
  if (!requestStr) return 'N/A';
  try {
    const req = JSON.parse(requestStr);
    if (req.messages?.length) {
      const last = req.messages.filter(m => m.role === 'user').pop();
      if (!last) return 'N/A';
      const content = Array.isArray(last.content)
        ? last.content.filter(i => i.type === 'text').map(i => i.text).join('')
        : (last.content || '');
      const flat = content.replace(/\n/g, '');
      return flat.length > 50 ? flat.substring(0, 50) + '...' : flat || 'N/A';
    }
    return req.prompt || 'N/A';
  } catch { return 'N/A'; }
};

const toggleAutoRefresh = (val) => {
  if (refreshTimer) {
    clearInterval(refreshTimer);
    refreshTimer = null;
  }
  if (val) {
    refreshTimer = setInterval(() => {
      if (document.visibilityState === 'hidden') return;
      fetchLogs();
      fetchSummary();
    }, 5000);
    ElMessage.success('自动刷新已开启');
  } else {
    ElMessage.info('自动刷新已关闭');
  }
};

onMounted(() => {
  fetchLogs(); fetchModels(); fetchSummary();
  refreshTimer = setInterval(() => {
    if (document.visibilityState === 'hidden') return;
    fetchLogs();
    fetchSummary();
  }, 5000);
});
onUnmounted(() => { if (refreshTimer) clearInterval(refreshTimer); });
</script>

<style scoped>
.logs { padding: 20px; }
.logs-card { background: var(--el-bg-color); }
.card-header { display: flex; justify-content: space-between; align-items: center; }
.title { font-size: 16px; font-weight: 600; }
.header-actions { display: flex; align-items: center; gap: 10px; }
.auto-refresh-label { font-size: 14px; }
.filter-form { margin-bottom: 20px; padding: 15px; background: var(--el-fill-color-light); border-radius: 8px; border: 1px solid var(--el-border-color); }
.summary-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(180px, 1fr)); gap: 12px; margin-bottom: 20px; }
.summary-card { padding: 14px 16px; border-radius: 8px; background: var(--el-fill-color-light); border: 1px solid var(--el-border-color); }
.summary-label { font-size: 12px; color: var(--el-text-color-secondary); margin-bottom: 6px; }
.summary-value { font-size: 22px; font-weight: 700; color: var(--el-text-color-primary); }
.pagination-container { margin-top: 20px; display: flex; justify-content: flex-end; }
.message-content { font-size: 14px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; cursor: pointer; }
</style>
