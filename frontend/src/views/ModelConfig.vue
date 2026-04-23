<template>
  <Layout>
    <div class="page-container">
      <el-row :gutter="20" class="stats-overview">
        <el-col :span="6">
          <el-card shadow="hover" class="stat-card">
            <div class="stat-content">
              <div class="stat-icon" style="background: #409eff;"><el-icon><Setting /></el-icon></div>
              <div class="stat-info">
                <div class="stat-value">{{ modelConfigs.length }}</div>
                <div class="stat-label">总配置数</div>
              </div>
            </div>
          </el-card>
        </el-col>
        <el-col :span="6">
          <el-card shadow="hover" class="stat-card">
            <div class="stat-content">
              <div class="stat-icon" style="background: #67c23a;"><el-icon><CircleCheck /></el-icon></div>
              <div class="stat-info">
                <div class="stat-value">{{ enabledCount }}</div>
                <div class="stat-label">启用中</div>
              </div>
            </div>
          </el-card>
        </el-col>
        <el-col :span="6">
          <el-card shadow="hover" class="stat-card">
            <div class="stat-content">
              <div class="stat-icon" style="background: #e6a23c;"><el-icon><Connection /></el-icon></div>
              <div class="stat-info">
                <div class="stat-value">{{ totalActiveRequests }}</div>
                <div class="stat-label">总并发请求</div>
              </div>
            </div>
          </el-card>
        </el-col>
        <el-col :span="6">
          <el-card shadow="hover" class="stat-card">
            <div class="stat-content">
              <div class="stat-icon" style="background: #f56c6c;"><el-icon><TrendCharts /></el-icon></div>
              <div class="stat-info">
                <div class="stat-value">{{ avgSuccessRate }}%</div>
                <div class="stat-label">平均成功率</div>
              </div>
            </div>
          </el-card>
        </el-col>
      </el-row>

      <el-card class="toolbar-card">
        <div class="toolbar">
          <div class="toolbar-left">
            <el-input v-model="searchKeyword" placeholder="搜索配置名称或模型名称..." prefix-icon="Search" clearable style="width: 300px;" />
            <el-select v-model="filterStatus" placeholder="状态筛选" clearable style="width: 150px; margin-left: 12px;">
              <el-option label="全部" value="" />
              <el-option label="启用" value="enabled" />
              <el-option label="禁用" value="disabled" />
            </el-select>
          </div>
          <div class="toolbar-right">
            <el-button type="primary" @click="showAddDialog = true"><el-icon><Plus /></el-icon>添加模型配置</el-button>
            <el-button @click="refreshData"><el-icon><Refresh /></el-icon>刷新</el-button>
          </div>
        </div>
      </el-card>

      <div v-loading="loading" class="cards-grid">
        <el-empty v-if="!filteredConfigs.length && !loading" description="暂无模型配置" />
        <el-card
          v-for="config in filteredConfigs"
          :key="config.id"
          class="model-card"
          shadow="hover"
          :class="{ 'card-disabled': !config.enabled }"
        >
          <template #header>
            <div class="card-header">
              <div class="header-left">
                <div class="model-icon"><el-icon><Cpu /></el-icon></div>
                <div class="model-info">
                  <div class="model-name">{{ config.name }}</div>
                  <div class="model-type">{{ config.modelName }}</div>
                </div>
              </div>
              <el-tag :type="config.enabled ? 'success' : 'danger'" size="small">{{ config.enabled ? '启用' : '禁用' }}</el-tag>
            </div>
          </template>

          <div class="card-body">
            <div class="info-item">
              <el-icon class="info-icon"><Link /></el-icon>
              <el-tooltip :content="config.apiBaseUrl" placement="top">
                <span class="info-text">{{ truncate(config.apiBaseUrl, 40) }}</span>
              </el-tooltip>
            </div>
            <div v-if="config.description" class="info-item">
              <el-icon class="info-icon"><Document /></el-icon>
              <el-tooltip :content="config.description" placement="top">
                <span class="info-text">{{ truncate(config.description, 50) }}</span>
              </el-tooltip>
            </div>

            <div class="core-metrics">
              <div class="metrics-row">
                <div class="priority-item">
                  <div class="metric-label-small">优先级</div>
                  <div class="priority-value-small" :class="getPriorityClass(config.priority)">{{ config.priority }}</div>
                </div>
                <div class="concurrency-item">
                  <div class="metric-label-small">并发使用</div>
                  <div class="concurrency-ratio-small">
                    {{ getStat(config)?.activeRequests ?? 0 }} / {{ config.maxConcurrency === 0 ? '∞' : config.maxConcurrency }}
                  </div>
                  <el-progress :percentage="getConcurrencyPercent(config)" :color="getConcurrencyColor(config)" :stroke-width="8" :show-text="false" />
                  <div class="concurrency-status-small">
                    <el-tag size="small" :type="getConcurrencyTagType(config)">{{ getConcurrencyStatusText(config) }}</el-tag>
                  </div>
                </div>
              </div>
            </div>

            <div class="metrics-section">
              <div class="metric-item">
                <div class="metric-label">成功率</div>
                <div class="metric-value success">{{ formatPercent(getStat(config)?.successRate) }}</div>
              </div>
              <div class="metric-item">
                <div class="metric-label">首Token延迟</div>
                <div class="metric-value warning">{{ formatLatency(getStat(config)?.avgFirstTokenLatency) }}</div>
              </div>
              <div class="metric-item">
                <div class="metric-label">Token延迟</div>
                <div class="metric-value primary">{{ formatLatency(getStat(config)?.avgTokenLatency) }}</div>
              </div>
              <div class="metric-item">
                <div class="metric-label">调度分数</div>
                <div class="metric-value" :style="{ color: getScoreColor(getStat(config)?.adaptiveRoutingScore) }">
                  {{ formatScore(getStat(config)?.adaptiveRoutingScore) }}
                </div>
              </div>
            </div>
          </div>

          <template #footer>
            <div class="card-actions">
              <el-tooltip content="编辑" placement="top">
                <el-button size="small" @click="handleEdit(config)"><el-icon><Edit /></el-icon></el-button>
              </el-tooltip>
              <el-tooltip content="测试" placement="top">
                <el-button size="small" type="success" @click="testConnection(config)"><el-icon><CircleCheck /></el-icon></el-button>
              </el-tooltip>
              <el-tooltip content="重置调度" placement="top">
                <el-button size="small" type="warning" @click="resetRuntime(config)"><el-icon><RefreshLeft /></el-icon></el-button>
              </el-tooltip>
              <el-tooltip content="删除" placement="top">
                <el-button size="small" type="danger" @click="handleDelete(config)"><el-icon><Delete /></el-icon></el-button>
              </el-tooltip>
            </div>
          </template>
        </el-card>
      </div>

      <el-dialog :title="editingConfig ? '编辑模型配置' : '添加模型配置'" v-model="showAddDialog" width="600px" @close="handleDialogClose">
        <el-form :model="form" :rules="rules" ref="formRef" label-width="120px">
          <el-form-item label="配置名称" prop="name">
            <el-input v-model="form.name" placeholder="请输入配置名称" />
          </el-form-item>
          <el-form-item label="模型名称" prop="modelName">
            <el-input v-model="form.modelName" placeholder="请输入模型名称" />
          </el-form-item>
          <el-form-item label="API地址" prop="apiBaseUrl">
            <el-input v-model="form.apiBaseUrl" placeholder="请输入API地址" />
          </el-form-item>
          <el-form-item label="API密钥">
            <el-input v-model="form.apiKey" type="password" placeholder="请输入API密钥" show-password />
          </el-form-item>
          <el-form-item label="最大Token数">
            <el-input-number v-model="form.maxTokens" :min="1" :max="100000" />
          </el-form-item>
          <el-form-item label="优先级">
            <el-input-number v-model="form.priority" :min="0" :max="100" />
            <span style="margin-left:12px; font-size:12px; color:var(--el-text-color-secondary);">数字越小越优先</span>
          </el-form-item>
          <el-form-item label="最大并发">
            <el-input-number v-model="form.maxConcurrency" :min="0" :max="100000" />
            <span style="margin-left:12px; font-size:12px; color:var(--el-text-color-secondary);">0 表示不限制</span>
          </el-form-item>
          <el-form-item label="温度">
            <el-slider v-model="form.temperature" :min="0" :max="2" :step="0.1" show-input style="width:100%;" />
          </el-form-item>
          <el-form-item label="描述">
            <el-input v-model="form.description" type="textarea" :rows="3" placeholder="请输入描述信息" />
          </el-form-item>
          <el-form-item label="启用状态">
            <el-switch v-model="form.enabled" />
          </el-form-item>
        </el-form>
        <template #footer>
          <el-button @click="showAddDialog = false">取消</el-button>
          <el-button type="primary" @click="handleSubmit" :loading="submitting">{{ editingConfig ? '更新' : '添加' }}</el-button>
        </template>
      </el-dialog>
    </div>
  </Layout>
</template>

<script setup>
import { ref, computed, onMounted, onBeforeUnmount } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import {
  Plus, Refresh, Setting, CircleCheck, Connection, TrendCharts,
  Cpu, Link, Document, Edit, Delete, RefreshLeft,
} from '@element-plus/icons-vue';
import Layout from '@/components/Layout.vue';
import { api } from '@/api/client.js';

const loading = ref(false);
const submitting = ref(false);
const showAddDialog = ref(false);
const editingConfig = ref(null);
const modelConfigs = ref([]);
const providerStats = ref([]);
const formRef = ref();
const searchKeyword = ref('');
const filterStatus = ref('');
let statsTimer = null;

const defaultForm = () => ({
  name: '', modelName: '', apiBaseUrl: '', apiKey: '',
  maxTokens: 8192, priority: 0, maxConcurrency: 0, temperature: 0.7,
  description: '', enabled: true,
});
const form = ref(defaultForm());

const rules = {
  name: [{ required: true, message: '请输入配置名称', trigger: 'blur' }],
  modelName: [{ required: true, message: '请输入模型名称', trigger: 'blur' }],
  apiBaseUrl: [{ required: true, message: '请输入API地址', trigger: 'blur' }],
};

const enabledCount = computed(() => modelConfigs.value.filter(c => c.enabled).length);
const totalActiveRequests = computed(() =>
  modelConfigs.value.reduce((s, c) => s + (getStat(c)?.activeRequests || 0), 0)
);
const avgSuccessRate = computed(() => {
  const rates = modelConfigs.value
    .map(c => getStat(c)?.successRate)
    .filter(r => r !== undefined && r !== null);
  if (!rates.length) return 0;
  return ((rates.reduce((s, r) => s + r, 0) / rates.length) * 100).toFixed(1);
});
const filteredConfigs = computed(() => {
  let result = modelConfigs.value;
  if (searchKeyword.value) {
    const kw = searchKeyword.value.toLowerCase();
    result = result.filter(c =>
      c.name.toLowerCase().includes(kw) ||
      c.modelName.toLowerCase().includes(kw)
    );
  }
  if (filterStatus.value === 'enabled') result = result.filter(c => c.enabled);
  else if (filterStatus.value === 'disabled') result = result.filter(c => !c.enabled);
  return result.slice().sort((a, b) => a.priority - b.priority);
});

const normalizeModelConfig = (config = {}) => ({
  id: config.id,
  name: config.name ?? '',
  modelName: config.modelName ?? config.model_name ?? '',
  apiBaseUrl: config.apiBaseUrl ?? config.api_base_url ?? '',
  apiKey: config.apiKey ?? config.api_key ?? '',
  maxTokens: config.maxTokens ?? config.max_tokens ?? 8192,
  priority: config.priority ?? 0,
  maxConcurrency: config.maxConcurrency ?? config.max_concurrency ?? 0,
  temperature: config.temperature ?? 0.7,
  description: config.description ?? '',
  enabled: config.enabled ?? true,
});

const normalizeProviderStat = (stat = {}) => ({
  modelName: stat.modelName ?? stat.model_name ?? '',
  requestCount: stat.requestCount ?? stat.request_count ?? 0,
  avgResponseTime: stat.avgResponseTime ?? stat.avg_response_time ?? 0,
  avgFirstTokenLatency: stat.avgFirstTokenLatency ?? stat.avg_first_token_latency ?? 0,
  avgTokenLatency: stat.avgTokenLatency ?? stat.avg_token_latency ?? 0,
  activeRequests: stat.activeRequests ?? stat.active_requests ?? 0,
  successRate: stat.successRate ?? stat.success_rate ?? null,
  backendConfigId: stat.backendConfigId ?? stat.backend_config_id ?? 0,
  backendModelName: stat.backendModelName ?? stat.backend_model_name ?? '',
  backendApiBaseUrl: stat.backendApiBaseURL ?? stat.backend_api_base_url ?? '',
  adaptiveRoutingScore: stat.adaptiveRoutingScore ?? stat.adaptive_routing_score ?? null,
});

const normalizeProviderStats = (stats) => Array.isArray(stats)
  ? stats.map(stat => normalizeProviderStat(stat))
  : [];

const getStat = (config) => {
  if (!config) return undefined;
  return providerStats.value.find((stat) => (
    (config.id && stat.backendConfigId === config.id) ||
    (stat.modelName && stat.modelName === config.name) ||
    (stat.backendModelName && stat.backendModelName === config.modelName)
  ));
};

const loadData = async () => {
  loading.value = true;
  try {
    const [cfgRes, statsRes] = await Promise.all([
      api.get('/model-configs'),
      api.get('/stats/providers'),
    ]);
    modelConfigs.value = Array.isArray(cfgRes.data)
      ? cfgRes.data.map((config) => normalizeModelConfig(config))
      : [];
    providerStats.value = normalizeProviderStats(statsRes.data);
  } catch {
    ElMessage.error('加载数据失败');
  } finally {
    loading.value = false;
  }
};

const loadStats = async () => {
  try {
    const res = await api.get('/stats/providers');
    providerStats.value = normalizeProviderStats(res.data);
  } catch {}
};

const refreshData = () => { loadData(); ElMessage.success('数据已刷新'); };

const handleEdit = (config) => {
  editingConfig.value = config;
  form.value = normalizeModelConfig(config);
  showAddDialog.value = true;
};

const handleDelete = async (config) => {
  try {
    await ElMessageBox.confirm(`确定要删除配置 "${config.name}" 吗？`, '确认删除', {
      confirmButtonText: '删除', cancelButtonText: '取消', type: 'warning',
    });
    await api.delete(`/model-configs/${config.id}`);
    ElMessage.success('删除成功');
    loadData();
  } catch (e) {
    if (e !== 'cancel') ElMessage.error('删除失败');
  }
};

const testConnection = async (config) => {
  try {
    const res = await api.post(`/model-configs/${config.id}/test`);
    if (res.data?.success) ElMessage.success('连接测试成功');
    else ElMessage.error('连接测试失败');
  } catch {
    ElMessage.error('测试连接失败');
  }
};

const resetRuntime = async (config) => {
  try {
    const res = await api.post(`/model-configs/${config.id}/reset-runtime`);
    ElMessage.success(res.data?.message || '调度状态已重置');
    loadData();
  } catch {
    ElMessage.error('重置失败');
  }
};

const handleSubmit = async () => {
  try {
    await formRef.value.validate();
    submitting.value = true;
    const payload = {
      name: form.value.name,
      model_name: form.value.modelName,
      api_base_url: form.value.apiBaseUrl,
      api_key: form.value.apiKey,
      max_tokens: form.value.maxTokens,
      priority: form.value.priority,
      max_concurrency: form.value.maxConcurrency,
      temperature: form.value.temperature,
      description: form.value.description,
      enabled: form.value.enabled,
    };
    if (editingConfig.value) {
      await api.put(`/model-configs/${editingConfig.value.id}`, payload);
      ElMessage.success('更新成功');
    } else {
      await api.post('/model-configs', payload);
      ElMessage.success('添加成功');
    }
    showAddDialog.value = false;
    loadData();
  } catch (e) {
    if (!e?.errors) ElMessage.error('操作失败');
  } finally {
    submitting.value = false;
  }
};

const handleDialogClose = () => {
  editingConfig.value = null;
  formRef.value?.resetFields();
  form.value = defaultForm();
};

const truncate = (text, max) => text?.length > max ? text.substring(0, max) + '...' : (text || '');
const formatLatency = (v) => v ? `${Math.round(v)}ms` : '-';
const formatPercent = (v) => v !== undefined && v !== null ? `${(v * 100).toFixed(1)}%` : '-';
const formatScore = (v) => v !== undefined && v !== null ? v.toFixed(0) : '-';
const getScoreColor = (v) => {
  if (v === undefined || v === null) return '#909399';
  if (v < 500) return '#67c23a';
  if (v < 1000) return '#e6a23c';
  return '#f56c6c';
};
const getPriorityClass = (p) => p <= 10 ? 'priority-high' : p <= 50 ? 'priority-medium' : 'priority-low';
const getConcurrencyPercent = (config) => {
  const active = getStat(config)?.activeRequests || 0;
  if (config.maxConcurrency === 0) return Math.min(active, 100);
  return Math.min((active / config.maxConcurrency) * 100, 100);
};
const getConcurrencyColor = (config) => {
  const p = getConcurrencyPercent(config);
  return p < 60 ? '#67c23a' : p < 80 ? '#e6a23c' : '#f56c6c';
};
const getConcurrencyTagType = (config) => {
  const p = getConcurrencyPercent(config);
  return p < 60 ? 'success' : p < 80 ? 'warning' : 'danger';
};
const getConcurrencyStatusText = (config) => {
  if (config.maxConcurrency === 0) return '无限制';
  const p = getConcurrencyPercent(config);
  if (p >= 90) return '接近满载';
  if (p >= 70) return '高负载';
  if (p >= 40) return '中等负载';
  return '低负载';
};

onMounted(() => {
  loadData();
  statsTimer = setInterval(loadStats, 5000);
});
onBeforeUnmount(() => { if (statsTimer) clearInterval(statsTimer); });
</script>

<style scoped>
.page-container { padding: 20px; }
.stats-overview { margin-bottom: 20px; }
.stat-card { border-radius: 8px; }
.stat-content { display: flex; align-items: center; gap: 16px; }
.stat-icon { width: 60px; height: 60px; border-radius: 12px; display: flex; align-items: center; justify-content: center; color: white; font-size: 28px; }
.stat-value { font-size: 28px; font-weight: bold; color: var(--el-text-color-primary); line-height: 1; margin-bottom: 4px; }
.stat-label { font-size: 14px; color: var(--el-text-color-secondary); }
.toolbar-card { margin-bottom: 20px; border-radius: 8px; }
.toolbar { display: flex; justify-content: space-between; align-items: center; }
.toolbar-left { display: flex; align-items: center; }
.toolbar-right { display: flex; gap: 12px; }
.cards-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(380px, 1fr)); gap: 20px; }
.model-card { border-radius: 8px; transition: all 0.3s ease; }
.model-card :deep(.el-card__header) { padding: 12px 16px; }
.model-card :deep(.el-card__body) { padding: 12px 16px; }
.model-card :deep(.el-card__footer) { padding: 10px 16px; }
.model-card:hover { transform: translateY(-2px); box-shadow: 0 4px 16px rgba(0,0,0,0.1); }
.card-disabled { opacity: 0.7; }
.card-header { display: flex; justify-content: space-between; align-items: center; }
.header-left { display: flex; align-items: center; gap: 10px; }
.model-icon { width: 36px; height: 36px; border-radius: 8px; background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); display: flex; align-items: center; justify-content: center; color: white; font-size: 18px; }
.model-name { font-size: 15px; font-weight: 600; color: var(--el-text-color-primary); margin-bottom: 2px; }
.model-type { font-size: 12px; color: var(--el-text-color-secondary); }
.info-item { display: flex; align-items: center; gap: 6px; margin-bottom: 8px; font-size: 12px; }
.info-icon { color: var(--el-text-color-secondary); font-size: 13px; flex-shrink: 0; }
.info-text { color: var(--el-text-color-regular); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.core-metrics { margin: 10px 0; }
.metrics-row { display: grid; grid-template-columns: 1fr 2fr; gap: 10px; }
.priority-item, .concurrency-item { padding: 8px 6px; background: var(--el-fill-color-light); border-radius: 6px; text-align: center; }
.metric-label-small { font-size: 11px; color: var(--el-text-color-secondary); margin-bottom: 4px; }
.priority-value-small { font-size: 16px; font-weight: bold; padding: 2px 6px; border-radius: 4px; display: inline-block; }
.priority-value-small.priority-high { background: linear-gradient(135deg, #67c23a 0%, #85ce61 100%); color: white; }
.priority-value-small.priority-medium { background: linear-gradient(135deg, #e6a23c 0%, #ebb563 100%); color: white; }
.priority-value-small.priority-low { background: linear-gradient(135deg, #909399 0%, #a6a9ad 100%); color: white; }
.concurrency-ratio-small { font-size: 12px; font-weight: bold; color: var(--el-text-color-primary); margin-bottom: 6px; }
.concurrency-status-small { display: flex; justify-content: center; margin-top: 4px; }
.metrics-section { display: grid; grid-template-columns: repeat(4, 1fr); gap: 8px; margin: 8px 0; padding: 8px; background: var(--el-fill-color-lighter); border-radius: 6px; }
.metric-item { text-align: center; }
.metric-label { font-size: 10px; color: var(--el-text-color-secondary); margin-bottom: 2px; }
.metric-value { font-size: 13px; font-weight: 600; color: var(--el-text-color-primary); }
.metric-value.success { color: #67c23a; }
.metric-value.warning { color: #e6a23c; }
.metric-value.primary { color: #409eff; }
.card-actions { display: flex; gap: 8px; justify-content: flex-end; }
.card-actions .el-button { width: 36px; height: 36px; padding: 0; display: flex; align-items: center; justify-content: center; font-size: 16px; }
</style>
