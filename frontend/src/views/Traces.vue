<template>
  <Layout>
    <div class="sessions-page">
      <el-card class="sessions-card">
        <template #header>
          <div class="card-header">
            <div>
              <span class="title">推断会话</span>
              <div class="subtitle">基于 request 上下文前缀，把连续 LLM 请求还原成会话流</div>
            </div>
            <div class="header-actions">
              <el-input-number v-model="limit" :min="100" :max="3000" :step="100" size="small" controls-position="right" />
              <el-button :icon="Refresh" @click="fetchSessions">刷新</el-button>
            </div>
          </div>
        </template>

        <el-table :data="sessions" v-loading="loading" @row-click="openSession">
          <el-table-column prop="rootLogId" label="Session" width="110">
            <template #default="{ row }">
              <span class="mono">#{{ row.rootLogId }}</span>
            </template>
          </el-table-column>
          <el-table-column label="本次任务" min-width="360" show-overflow-tooltip>
            <template #default="{ row }">
              <div class="session-title">{{ row.preview || 'N/A' }}</div>
            </template>
          </el-table-column>
          <el-table-column label="消息数" width="100" align="center">
            <template #default="{ row }">
              <el-tag type="info">{{ row.stepCount }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="模型" min-width="160" show-overflow-tooltip>
            <template #default="{ row }">
              <span>{{ row.modelNames.join(', ') || '-' }}</span>
            </template>
          </el-table-column>
          <el-table-column label="Backend" min-width="180" show-overflow-tooltip>
            <template #default="{ row }">
              <span>{{ row.backendNames.join(', ') || '-' }}</span>
            </template>
          </el-table-column>
          <el-table-column label="耗时" width="110">
            <template #default="{ row }">{{ formatDuration(row.durationMs) }}</template>
          </el-table-column>
          <el-table-column label="置信度" width="100" align="center">
            <template #default="{ row }">
              <el-tag :type="confidenceType(row.confidence)">{{ percent(row.confidence) }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="更新时间" width="180">
            <template #default="{ row }">{{ formatDateTime(row.endAt) }}</template>
          </el-table-column>
          <el-table-column label="操作" width="100" fixed="right">
            <template #default="{ row }">
              <el-button link type="primary" :icon="View" @click.stop="openSession(row)">详情</el-button>
            </template>
          </el-table-column>
        </el-table>
      </el-card>

      <el-dialog v-model="sessionVisible" width="88%" class="session-dialog" destroy-on-close>
        <template #header>
          <div class="dialog-title">
            <span>Session #{{ activeSession?.rootLogId }}</span>
            <el-tag size="small" :type="confidenceType(activeSession?.confidence || 0)">
              {{ percent(activeSession?.confidence || 0) }}
            </el-tag>
          </div>
        </template>

        <div v-if="activeSession" class="session-detail">
          <div class="session-header-bar">
            <div class="session-header-info">
              <code class="session-id">{{ activeSession.traceKey }}</code>
              <span class="session-badges">
                <el-tag size="small" effect="plain">{{ activeSession.stepCount }} 条消息</el-tag>
                <el-tag v-for="model in activeSession.modelNames" :key="model" size="small" effect="plain" type="success">{{ model }}</el-tag>
                <el-tag v-for="backend in activeSession.backendNames" :key="backend" size="small" effect="plain" type="warning">{{ backend }}</el-tag>
              </span>
            </div>
          </div>

          <div class="summary-stack">
            <section class="summary-panel">
              <div class="summary-heading">本次任务</div>
              <div class="summary-text">{{ activeSession.preview || 'N/A' }}</div>
            </section>
            <section class="summary-panel">
              <div class="summary-heading">会话统计</div>
              <div class="summary-metrics">
                <span>开始 {{ formatDateTime(activeSession.startAt) }}</span>
                <span>结束 {{ formatDateTime(activeSession.endAt) }}</span>
                <span>耗时 {{ formatDuration(activeSession.durationMs) }}</span>
              </div>
            </section>
          </div>

          <div class="messages-block">
            <div class="section-heading">
              <div>
                <div class="events-title">消息时间线</div>
                <div class="section-subtitle">每条消息对应一次网关请求，按上下文前缀推断父子关系</div>
              </div>
              <el-tag size="small" effect="plain" type="info">{{ activeSession.steps.length }} 条消息</el-tag>
            </div>

            <div class="message-list">
              <article v-for="(step, index) in activeSession.steps" :key="step.id" class="message-card">
                <div class="message-card-header">
                  <div class="message-card-meta">
                    <el-tag effect="plain" type="success">user</el-tag>
                    <span class="message-title">{{ step.preview || '用户输入' }}</span>
                    <span class="message-index">#{{ index + 1 }}</span>
                  </div>
                  <span class="message-time">{{ formatDateTime(step.createdAt) }}</span>
                </div>

                <div class="parts-list">
                  <section class="part-card part-card--text">
                    <div class="part-header">
                      <div class="part-title">
                        <span class="part-section-title">输入摘要</span>
                      </div>
                    </div>
                    <div class="part-text">{{ step.preview || 'N/A' }}</div>
                  </section>

                  <section class="part-card part-card--assistant">
                    <div class="part-header">
                      <div class="part-title">
                        <el-tag size="small" effect="plain" type="warning">assistant</el-tag>
                        <span class="part-section-title">模型响应</span>
                      </div>
                      <div class="part-meta">
                        <span>{{ formatLatency(step.responseTime) }}</span>
                      </div>
                    </div>
                    <div class="response-line">
                      <span>{{ step.modelName }} / {{ step.backendModelName || '-' }}</span>
                      <span>{{ bytes(step.responseBytes) }}</span>
                    </div>
                  </section>
                </div>

                <details class="message-disclosure">
                  <summary>查看请求元数据</summary>
                  <div class="detail-grid message-descriptions">
                    <div class="detail-item">
                      <span class="detail-label">Log ID</span>
                      <button class="link-button detail-value" @click="viewLog(step)">#{{ step.id }}</button>
                    </div>
                    <div class="detail-item">
                      <span class="detail-label">Parent</span>
                      <span class="detail-value">{{ step.parentId ? `#${step.parentId}` : '-' }}</span>
                    </div>
                    <div class="detail-item">
                      <span class="detail-label">关联方式</span>
                      <span class="detail-value">{{ step.matchReason || 'root' }}</span>
                    </div>
                    <div class="detail-item">
                      <span class="detail-label">置信度</span>
                      <span class="detail-value">{{ percent(step.confidence) }}</span>
                    </div>
                    <div class="detail-item">
                      <span class="detail-label">上下文消息数</span>
                      <span class="detail-value">{{ step.messageCount }}</span>
                    </div>
                    <div class="detail-item">
                      <span class="detail-label">Request / Response</span>
                      <span class="detail-value">{{ bytes(step.requestBytes) }} / {{ bytes(step.responseBytes) }}</span>
                    </div>
                  </div>
                </details>
              </article>
            </div>
          </div>
        </div>
      </el-dialog>

      <LogDetailsDialog v-model="detailVisible" :log="currentLog" />
    </div>
  </Layout>
</template>

<script setup>
import { onMounted, ref } from 'vue';
import { ElMessage } from 'element-plus';
import { Refresh, View } from '@element-plus/icons-vue';
import Layout from '@/components/Layout.vue';
import LogDetailsDialog from '@/components/llm-logs/LogDetailsDialog.vue';
import { api } from '@/api/client.js';
import { formatDateTime, formatLatency } from '@/utils/format.js';

const sessions = ref([]);
const activeSession = ref(null);
const loading = ref(false);
const limit = ref(500);
const sessionVisible = ref(false);
const currentLog = ref(null);
const detailVisible = ref(false);

const normalizeStep = (step = {}) => ({
  ...step,
  id: step.id ?? 0,
  parentId: step.parentId ?? step.parent_id ?? 0,
  createdAt: step.createdAt ?? step.created_at ?? '',
  modelName: step.modelName ?? step.model_name ?? '',
  backendModelName: step.backendModelName ?? step.backend_model_name ?? '',
  responseTime: step.responseTime ?? step.response_time ?? 0,
  messageCount: step.messageCount ?? step.message_count ?? 0,
  requestBytes: step.requestBytes ?? step.request_bytes ?? 0,
  responseBytes: step.responseBytes ?? step.response_bytes ?? 0,
  matchReason: step.matchReason ?? step.match_reason ?? '',
  confidence: step.confidence ?? 1,
  preview: step.preview ?? '',
});

const normalizeSession = (trace = {}) => ({
  ...trace,
  traceKey: trace.traceKey ?? trace.trace_key ?? '',
  rootLogId: trace.rootLogId ?? trace.root_log_id ?? 0,
  stepCount: trace.stepCount ?? trace.step_count ?? 0,
  confidence: trace.confidence ?? 1,
  startAt: trace.startAt ?? trace.start_at ?? '',
  endAt: trace.endAt ?? trace.end_at ?? '',
  durationMs: trace.durationMs ?? trace.duration_ms ?? 0,
  modelNames: trace.modelNames ?? trace.model_names ?? [],
  backendNames: trace.backendNames ?? trace.backend_names ?? [],
  preview: trace.preview ?? '',
  steps: (trace.steps || []).map(normalizeStep),
});

const fetchSessions = async () => {
  loading.value = true;
  try {
    const res = await api.get('/request-logs/inferred-traces', {
      params: { include_steps: true, limit: limit.value, min_steps: 2 },
    });
    sessions.value = (res.data?.traces || []).map(normalizeSession);
  } catch {
    ElMessage.error('获取推断会话失败');
  } finally {
    loading.value = false;
  }
};

const openSession = (session) => {
  activeSession.value = session;
  sessionVisible.value = true;
};

const confidenceType = (value) => {
  if (value >= 0.95) return 'success';
  if (value >= 0.8) return 'warning';
  return 'info';
};

const percent = (value) => `${Math.round((value || 0) * 100)}%`;

const bytes = (value) => {
  if (!value) return '-';
  if (value < 1024) return `${value} B`;
  if (value < 1024 * 1024) return `${Math.round(value / 1024)} KB`;
  return `${(value / 1024 / 1024).toFixed(1)} MB`;
};

const formatDuration = (value) => {
  if (!value) return '-';
  if (value < 1000) return `${Math.round(value)}ms`;
  const seconds = value / 1000;
  if (seconds < 60) return `${seconds.toFixed(1)}s`;
  const minutes = Math.floor(seconds / 60);
  const rest = Math.round(seconds % 60);
  return `${minutes}m ${rest}s`;
};

const viewLog = async (step) => {
  try {
    const res = await api.get(`/request-logs/${step.id}`);
    currentLog.value = res.data || {};
    detailVisible.value = true;
  } catch {
    ElMessage.error('获取日志详情失败');
  }
};

onMounted(fetchSessions);
</script>

<style scoped>
.sessions-page { padding: 20px; }
.sessions-card { background: var(--el-bg-color); }
.card-header, .header-actions, .dialog-title, .session-header-info, .session-badges, .message-card-header, .message-card-meta, .part-header, .part-title, .part-meta, .summary-metrics {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}
.card-header { justify-content: space-between; }
.title { font-size: 16px; font-weight: 700; color: var(--el-text-color-primary); }
.subtitle { margin-top: 4px; font-size: 12px; color: var(--el-text-color-secondary); }
.session-title { font-weight: 600; color: var(--el-text-color-primary); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.mono, .session-id { font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace; }
.dialog-title { font-size: 18px; font-weight: 700; }
.session-detail { min-height: 200px; color: var(--el-text-color-primary); }
.session-header-bar {
  justify-content: space-between;
  gap: 16px;
  padding: 12px 16px;
  border: 1px solid var(--el-border-color);
  border-radius: 8px;
  background: var(--el-fill-color-light);
  margin-bottom: 16px;
}
.session-id {
  font-size: 13px;
  color: var(--el-text-color-primary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.summary-stack {
  display: grid;
  gap: 12px;
  margin-bottom: 16px;
}
.summary-panel, .message-card, .part-card, .detail-item {
  border: 1px solid var(--el-border-color);
  border-radius: 8px;
  background: var(--el-fill-color-light);
}
.summary-panel { padding: 16px; }
.summary-heading, .events-title, .part-section-title { font-weight: 700; }
.summary-text {
  margin-top: 8px;
  max-height: 160px;
  overflow: auto;
  font-size: 13px;
  line-height: 1.6;
  white-space: pre-wrap;
}
.summary-metrics { margin-top: 8px; color: var(--el-text-color-secondary); font-size: 13px; }
.section-heading {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 14px;
}
.section-subtitle { margin-top: 4px; color: var(--el-text-color-secondary); font-size: 13px; }
.message-list, .parts-list { display: flex; flex-direction: column; gap: 12px; }
.message-list { gap: 14px; }
.message-card { padding: 16px 18px; }
.message-card-header, .part-header { justify-content: space-between; align-items: flex-start; }
.message-title { font-weight: 600; max-width: min(760px, 80vw); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.message-index, .message-time, .part-meta, .response-line {
  color: var(--el-text-color-secondary);
  font-size: 13px;
}
.parts-list { margin-top: 12px; gap: 10px; }
.part-card {
  padding: 12px;
  background: var(--el-bg-color);
}
.part-text {
  margin-top: 8px;
  line-height: 1.6;
  white-space: pre-wrap;
  word-break: break-word;
}
.response-line {
  display: flex;
  gap: 12px;
  flex-wrap: wrap;
  margin-top: 8px;
}
.message-disclosure {
  margin-top: 12px;
  border-top: 1px dashed var(--el-border-color);
  padding-top: 12px;
}
.message-disclosure summary {
  cursor: pointer;
  color: var(--el-color-primary);
  font-size: 13px;
  user-select: none;
}
.message-disclosure[open] summary { margin-bottom: 12px; }
.detail-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px;
}
.detail-item { padding: 12px 14px; }
.detail-label {
  display: block;
  margin-bottom: 6px;
  color: var(--el-text-color-secondary);
  font-size: 12px;
}
.detail-value { display: block; font-size: 13px; line-height: 1.5; }
.link-button {
  border: 0;
  background: transparent;
  padding: 0;
  color: var(--el-color-primary);
  cursor: pointer;
}
:deep(.session-dialog .el-dialog__body) { padding-top: 12px; }

@media (max-width: 960px) {
  .sessions-page { padding: 12px; }
  .card-header, .section-heading { align-items: flex-start; flex-direction: column; }
  .detail-grid { grid-template-columns: 1fr; }
  .message-title { max-width: 100%; }
}
</style>
