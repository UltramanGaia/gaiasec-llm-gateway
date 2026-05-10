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
                <div class="stat-value">{{ totalWaitingRequests }}</div>
                <div class="stat-label">排队请求</div>
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

      <div v-loading="loading" class="alias-groups">
        <el-empty v-if="!aliasGroups.length && !loading" description="暂无模型配置" />
        <section
          v-for="group in aliasGroups"
          :key="group.alias"
          class="alias-group"
          :class="{ 'alias-group-drop-target': dragTargetAlias === group.alias && canDropToAlias(group.alias) }"
          @dragenter.prevent="handleGroupDragEnter(group)"
          @dragover.prevent="handleGroupDragOver(group, $event)"
          @drop.prevent="handleGroupDrop(group, $event)"
        >
          <div class="alias-pill">
            <span class="alias-pill-value">{{ group.alias }}</span>
          </div>
          <div class="cards-row">
            <el-card
              v-for="config in group.configs"
              :key="config.id"
              class="model-card"
              shadow="hover"
              :class="{
                'card-disabled': !config.enabled,
                'model-card-dragging': draggingConfig?.id === config.id,
              }"
              draggable="true"
              @dragstart="handleDragStart(config, $event)"
              @dragend="handleDragEnd"
            >
          <template #header>
            <div class="card-header">
              <div class="header-left">
                <div class="model-icon"><el-icon><Cpu /></el-icon></div>
                <div class="model-info">
                  <div class="model-name">{{ config.modelName }}</div>
                </div>
              </div>
              <el-switch
                :model-value="config.enabled"
                inline-prompt
                :active-text="'启用'"
                :inactive-text="'禁用'"
                @change="toggleConfigEnabled(config, $event)"
              />
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
              <div class="metrics-grid">
                <div class="metric-priority">
                  <div class="metric-label-small">优先级</div>
                  <div class="priority-value-small" :class="getPriorityClass(config.priority)">{{ config.priority }}</div>
                </div>
                <div class="metric-concurrency">
                  <div class="metric-label-small">并发</div>
                  <div class="concurrency-info">
                    <span class="metric-value-compact">{{ getStat(config)?.activeRequests ?? 0 }}/{{ config.maxConcurrency === 0 ? '∞' : config.maxConcurrency }}</span>
                    <span v-if="getWaitingRequests(config) > 0" class="queue-text">({{ getWaitingRequests(config) }})</span>
                  </div>
                  <el-progress :percentage="getConcurrencyPercent(config)" :color="getConcurrencyColor(config)" :stroke-width="5" :show-text="false" />
                </div>
                <div class="metric-small metric-col-4">
                  <div class="metric-label-small">首Token</div>
                  <div class="metric-value-compact warning">{{ formatLatency(getStat(config)?.avgFirstTokenLatency) }}</div>
                </div>
                <div class="metric-small metric-col-5">
                  <div class="metric-label-small">Token延迟</div>
                  <div class="metric-value-compact primary">{{ formatLatency(getStat(config)?.avgTokenLatency) }}</div>
                </div>
                <div class="metric-small metric-col-4 metric-row-2">
                  <div class="metric-label-small">成功率</div>
                  <div class="metric-value-compact success">{{ formatPercent(getStat(config)?.successRate) }}</div>
                </div>
                <div class="metric-small metric-col-5 metric-row-2">
                  <div class="metric-label-small">调度分数</div>
                  <div class="metric-value-compact" :style="{ color: getScoreColor(getStat(config)?.adaptiveRoutingScore) }">
                    {{ formatScore(getStat(config)?.adaptiveRoutingScore) }}
                  </div>
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
        </section>
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
            <el-input
              v-model="form.apiKey"
              type="password"
              :placeholder="editingConfig ? '留空则保持现有API密钥' : '请输入API密钥'"
              show-password
            />
            <span v-if="editingConfig?.apiKeySet" style="margin-left:12px; font-size:12px; color:var(--el-text-color-secondary);">当前已配置 API 密钥，留空则保持不变</span>
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
const draggingConfig = ref(null);
const dragTargetAlias = ref('');
let statsTimer = null;

const defaultForm = () => ({
  name: '', modelName: '', apiBaseUrl: '', apiKey: '',
  maxTokens: 32000, priority: 0, maxConcurrency: 0, temperature: 0.7,
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
const totalWaitingRequests = computed(() =>
  modelConfigs.value.reduce((s, c) => s + getWaitingRequests(c), 0)
);
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

const aliasGroups = computed(() => {
  const groups = new Map();
  filteredConfigs.value.forEach((config) => {
    const key = config.name || '未命名别名';
    if (!groups.has(key)) {
      groups.set(key, {
        alias: key,
        configs: [],
        enabledCount: 0,
        uniqueModelCount: 0,
      });
    }
    const group = groups.get(key);
    group.configs.push(config);
    if (config.enabled) group.enabledCount += 1;
  });
  const aliasOrder = ['auto', 'max', 'pro', 'mini'];
  const getAliasPriority = (alias) => {
    const index = aliasOrder.indexOf(alias.toLowerCase());
    return index === -1 ? aliasOrder.length : index;
  };
  return Array.from(groups.values())
    .map((group) => ({
      ...group,
      configs: group.configs.slice().sort((a, b) => {
        if (a.enabled !== b.enabled) return b.enabled ? 1 : -1;
        return a.priority - b.priority || a.id - b.id;
      }),
      uniqueModelCount: new Set(group.configs.map(config => config.modelName)).size,
    }))
    .sort((a, b) => {
      const pa = getAliasPriority(a.alias);
      const pb = getAliasPriority(b.alias);
      if (pa !== pb) return pa - pb;
      return a.alias.localeCompare(b.alias);
    });
});

const normalizeModelConfig = (config = {}) => ({
  id: config.id,
  name: config.name ?? '',
  modelName: config.modelName ?? config.model_name ?? '',
  apiBaseUrl: config.apiBaseUrl ?? config.api_base_url ?? '',
  apiKey: '',
  apiKeySet: config.apiKeySet ?? config.api_key_set ?? false,
  maxTokens: config.maxTokens ?? config.max_tokens ?? 32000,
  priority: config.priority ?? 0,
  maxConcurrency: config.maxConcurrency ?? config.max_concurrency ?? 0,
  temperature: config.temperature ?? 0.7,
  description: config.description ?? '',
  enabled: config.enabled ?? true,
});

const buildModelConfigPayload = (config, overrides = {}) => ({
  name: overrides.name ?? config.name,
  model_name: overrides.modelName ?? config.modelName,
  api_base_url: overrides.apiBaseUrl ?? config.apiBaseUrl,
  max_tokens: overrides.maxTokens ?? config.maxTokens,
  priority: overrides.priority ?? config.priority,
  max_concurrency: overrides.maxConcurrency ?? config.maxConcurrency,
  temperature: overrides.temperature ?? config.temperature,
  description: overrides.description ?? config.description,
  enabled: overrides.enabled ?? config.enabled,
  ...(overrides.apiKey ? { api_key: overrides.apiKey } : {}),
});

const normalizeProviderStat = (stat = {}) => ({
  modelName: stat.modelName ?? stat.model_name ?? '',
  requestCount: stat.requestCount ?? stat.request_count ?? 0,
  avgResponseTime: stat.avgResponseTime ?? stat.avg_response_time ?? 0,
  avgFirstTokenLatency: stat.avgFirstTokenLatency ?? stat.avg_first_token_latency ?? 0,
  avgTokenLatency: stat.avgTokenLatency ?? stat.avg_token_latency ?? 0,
  activeRequests: stat.activeRequests ?? stat.active_requests ?? 0,
  waitingRequests: stat.waitingRequests ?? stat.waiting_requests ?? 0,
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
  form.value = { ...normalizeModelConfig(config), apiKey: '' };
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

const toggleConfigEnabled = async (config, enabled) => {
  const previous = config.enabled;
  config.enabled = enabled;
  try {
    const payload = buildModelConfigPayload(config, { enabled });
    await api.put(`/model-configs/${config.id}`, payload);
    ElMessage.success(`配置已${enabled ? '启用' : '禁用'}`);
  } catch {
    config.enabled = previous;
    ElMessage.error('状态切换失败');
  }
};

const canDropToAlias = (targetAlias) => {
  if (!draggingConfig.value) return false;
  return draggingConfig.value.name !== targetAlias;
};

const handleDragStart = (config, event) => {
  draggingConfig.value = config;
  dragTargetAlias.value = '';
  if (event.dataTransfer) {
    event.dataTransfer.effectAllowed = 'copyMove';
    event.dataTransfer.setData('text/plain', String(config.id));
  }
};

const handleDragEnd = () => {
  draggingConfig.value = null;
  dragTargetAlias.value = '';
};

const handleGroupDragEnter = (group) => {
  if (canDropToAlias(group.alias)) {
    dragTargetAlias.value = group.alias;
  }
};

const handleGroupDragOver = (group, event) => {
  if (!canDropToAlias(group.alias)) return;
  dragTargetAlias.value = group.alias;
  if (event.dataTransfer) {
    event.dataTransfer.dropEffect = event.ctrlKey || event.metaKey ? 'copy' : 'move';
  }
};

const moveConfigToAlias = async (config, targetAlias) => {
  const payload = buildModelConfigPayload(config, { name: targetAlias });
  await api.put(`/model-configs/${config.id}`, payload);
  ElMessage.success(`已移动到分组 ${targetAlias}`);
  await loadData();
};

const cloneConfigToAlias = async (config, targetAlias) => {
  await api.post(`/model-configs/${config.id}/clone`, { name: targetAlias });
  ElMessage.success(`已复制到分组 ${targetAlias}`);
  await loadData();
};

const handleGroupDrop = async (group, event) => {
  const source = draggingConfig.value;
  dragTargetAlias.value = '';
  if (!source || !canDropToAlias(group.alias)) return;

  try {
    if (event.ctrlKey || event.metaKey) {
      await cloneConfigToAlias(source, group.alias);
      return;
    }

    await ElMessageBox.confirm(
      `将配置 "${source.modelName}" 放入分组 "${group.alias}"。`,
      '选择拖拽操作',
      {
        confirmButtonText: '移动',
        cancelButtonText: '复制',
        distinguishCancelAndClose: true,
        closeOnClickModal: false,
        closeOnPressEscape: true,
        type: 'info',
      }
    );
    await moveConfigToAlias(source, group.alias);
  } catch (error) {
    if (error === 'cancel') {
      try {
        await cloneConfigToAlias(source, group.alias);
      } catch {
        ElMessage.error('复制失败');
      }
    } else if (error !== 'close') {
      ElMessage.error('拖拽操作失败');
    }
  } finally {
    handleDragEnd();
  }
};

const handleSubmit = async () => {
  try {
    await formRef.value.validate();
    submitting.value = true;
    const payload = buildModelConfigPayload(form.value);
    if (!editingConfig.value || form.value.apiKey) {
      payload.api_key = form.value.apiKey;
    }
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
const getWaitingRequests = (config) => getStat(config)?.waitingRequests || 0;
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
.drag-hint {
  margin-top: 12px;
  font-size: 13px;
  color: var(--el-text-color-secondary);
}
.alias-groups { display: flex; flex-direction: column; gap: 24px; }
.alias-group {
  display: flex;
  align-items: flex-start;
  gap: 16px;
  padding: 16px;
  border-radius: 22px;
  background:
    radial-gradient(circle at top left, rgba(102, 126, 234, 0.15), transparent 32%),
    linear-gradient(180deg, rgba(30, 41, 59, 0.8) 0%, rgba(15, 23, 42, 0.9) 100%);
  border: 1px solid rgba(102, 126, 234, 0.2);
  box-shadow: 0 18px 40px rgba(15, 23, 42, 0.28);
}
.alias-group-drop-target {
  border-color: rgba(96, 165, 250, 0.85);
  box-shadow: 0 0 0 2px rgba(96, 165, 250, 0.32), 0 18px 40px rgba(15, 23, 42, 0.28);
}
.alias-pill {
  display: flex;
  align-items: center;
  justify-content: center;
  min-width: 100px;
  padding: 12px 16px;
  border-radius: 14px;
  background: linear-gradient(135deg, rgba(59, 130, 246, 0.25) 0%, rgba(139, 92, 246, 0.25) 100%);
  color: #e8eefc;
  border: 1px solid rgba(96, 165, 250, 0.4);
  backdrop-filter: blur(6px);
  align-self: stretch;
}
.alias-pill-value { font-size: 18px; font-weight: 700; color: #fff; text-shadow: 0 0 12px rgba(96, 165, 250, 0.5); }
.cards-row { display: flex; gap: 12px; flex-wrap: wrap; flex: 1; }
.group-stat {
  min-width: 90px;
  padding: 10px 14px;
  border-radius: 16px;
  background: rgba(255, 255, 255, 0.08);
  border: 1px solid rgba(148, 163, 184, 0.22);
  text-align: center;
}
.group-stat-value {
  display: block;
  font-size: 20px;
  font-weight: 700;
  color: #f8fbff;
}
.group-stat-label {
  display: block;
  margin-top: 4px;
  font-size: 12px;
  color: #9fb0ca;
}
.group-target-strip {
  display: flex;
  gap: 14px;
  align-items: flex-start;
  padding: 14px 16px;
  border-radius: 18px;
  background: rgba(255, 255, 255, 0.06);
  border: 1px solid rgba(148, 163, 184, 0.18);
  margin-bottom: 18px;
}
.strip-label {
  flex-shrink: 0;
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: #9fb0ca;
  padding-top: 6px;
}
.target-chip-list { display: flex; flex-wrap: wrap; gap: 10px; }
.target-chip {
  display: inline-flex;
  flex-direction: column;
  gap: 4px;
  padding: 10px 12px;
  border-radius: 14px;
  background: linear-gradient(180deg, rgba(255, 255, 255, 0.12) 0%, rgba(255, 255, 255, 0.07) 100%);
  border: 1px solid rgba(148, 163, 184, 0.22);
  box-shadow: 0 8px 18px rgba(15, 23, 42, 0.18);
  backdrop-filter: blur(4px);
}
.target-chip.disabled { opacity: 0.55; }
.target-chip-model { font-size: 13px; font-weight: 700; color: #eef4ff; }
.target-chip-meta { font-size: 11px; color: #9fb0ca; }
.model-card { width: 420px; border-radius: 8px; transition: all 0.3s ease; }
.model-card-dragging { opacity: 0.45; transform: scale(0.98); }
.model-card :deep(.el-card__header) { padding: 8px 12px; }
.model-card :deep(.el-card__body) { padding: 8px 12px; }
.model-card :deep(.el-card__footer) { padding: 6px 12px; }
.model-card:hover { transform: translateY(-2px); box-shadow: 0 4px 16px rgba(0,0,0,0.1); }
.card-disabled { opacity: 0.7; }
.card-header { display: flex; justify-content: space-between; align-items: center; }
.header-left { display: flex; align-items: center; gap: 10px; }
.model-icon { width: 36px; height: 36px; border-radius: 8px; background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); display: flex; align-items: center; justify-content: center; color: white; font-size: 18px; }
.model-name { font-size: 15px; font-weight: 600; color: var(--el-text-color-primary); }
.info-item { display: flex; align-items: center; gap: 6px; margin-bottom: 4px; font-size: 12px; }
.info-icon { color: var(--el-text-color-secondary); font-size: 13px; flex-shrink: 0; }
.info-text { color: var(--el-text-color-regular); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.core-metrics { margin: 6px 0; }
.metrics-grid { display: grid; grid-template-columns: 70px 1fr 1fr 70px 70px; grid-template-rows: auto auto; gap: 6px 8px; }
.metric-priority { grid-row: span 2; grid-column: 1; padding: 8px 6px; background: var(--el-fill-color-light); border-radius: 6px; text-align: center; display: flex; flex-direction: column; justify-content: center; }
.metric-concurrency { grid-row: span 2; grid-column: 2 / span 2; padding: 8px; background: var(--el-fill-color-light); border-radius: 6px; display: flex; flex-direction: column; justify-content: center; align-items: stretch; }
.metric-concurrency .metric-label-small { text-align: center; }
.metric-concurrency :deep(.el-progress) { width: 100%; }
.concurrency-info { text-align: center; margin-bottom: 6px; }
.queue-text { font-size: 12px; font-weight: 600; color: #f56c6c; }
.metric-small { padding: 6px 4px; background: var(--el-fill-color-light); border-radius: 6px; text-align: center; }
.metric-col-4 { grid-column: 4; }
.metric-col-5 { grid-column: 5; }
.metric-row-2 { grid-row: 2; }
.metric-label-small { font-size: 10px; color: var(--el-text-color-secondary); margin-bottom: 2px; }
.priority-value-small { font-size: 16px; font-weight: bold; padding: 2px 6px; border-radius: 4px; display: inline-block; }
.priority-value-small.priority-high { background: linear-gradient(135deg, #67c23a 0%, #85ce61 100%); color: white; }
.priority-value-small.priority-medium { background: linear-gradient(135deg, #e6a23c 0%, #ebb563 100%); color: white; }
.priority-value-small.priority-low { background: linear-gradient(135deg, #909399 0%, #a6a9ad 100%); color: white; }
.metric-value-compact { font-size: 12px; font-weight: 600; color: var(--el-text-color-primary); }
.metric-value-compact.success { color: #67c23a; }
.metric-value-compact.warning { color: #e6a23c; }
.metric-value-compact.primary { color: #409eff; }
.card-actions { display: flex; gap: 8px; justify-content: flex-end; }
.card-actions .el-button { width: 36px; height: 36px; padding: 0; display: flex; align-items: center; justify-content: center; font-size: 16px; }

@media (max-width: 1200px) {
  .alias-group { flex-direction: column; }
  .alias-pill { min-width: auto; width: 100%; min-height: auto; padding: 8px 12px; }
  .cards-row { flex-direction: column; }
  .model-card { width: 100%; }
}

@media (max-width: 768px) {
  .page-container { padding: 12px; }
  .toolbar { flex-direction: column; align-items: stretch; gap: 12px; }
  .toolbar-left, .toolbar-right { flex-wrap: wrap; }
  .group-target-strip { flex-direction: column; }
  .metrics-grid { grid-template-columns: 60px 1fr 60px 60px; }
}
</style>
