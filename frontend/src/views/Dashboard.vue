<script setup>
import { ref, onMounted, computed } from 'vue';
import { ElRow, ElCol, ElCard, ElTable, ElTableColumn, ElTag, ElMessage, ElProgress, ElTooltip } from 'element-plus';
import { Collection, Connection, Sort, Timer, PieChart, Clock } from '@element-plus/icons-vue';
import { statsAPI, logsAPI } from '../api';

// 系统统计数据
const stats = ref({
  totalRequests: 0,
  activeProviders: 0,
  modelMappings: 0,
  avgResponseTime: 0
});

// 最近请求数据
const requests = ref([
  {
    id: '1',
    model: 'gpt-3.5-turbo',
    time: '2 min ago',
    status: 'success',
    provider: 'OpenAI',
    content: 'Hello, how can I assist you today?'
  },
  {
    id: '2',
    model: 'claude-2',
    time: '5 min ago',
    status: 'success',
    provider: 'Anthropic',
    content: 'I need help with a programming problem...'
  },
  {
    id: '3',
    model: 'gemini-pro',
    time: '10 min ago',
    status: 'error',
    provider: 'Google AI',
    content: 'Error: API rate limit exceeded'
  }
]);

// 提供者统计数据
const providerStats = ref([]);

// 模型统计数据
const modelStats = ref([]);

// 格式化时间
const formatTime = (dateString) => {
  const date = new Date(dateString);
  const now = new Date();
  const diffMs = now - date;
  const diffMins = Math.floor(diffMs / 60000);
  const diffHours = Math.floor(diffMs / 3600000);
  const diffDays = Math.floor(diffMs / 86400000);

  if (diffMins < 60) {
    return `${diffMins} min ago`;
  } else if (diffHours < 24) {
    return `${diffHours} hour${diffHours > 1 ? 's' : ''} ago`;
  } else {
    return `${diffDays} day${diffDays > 1 ? 's' : ''} ago`;
  }
};

// 格式化请求内容
const formatContent = (content) => {
  if (!content) return 'No content';
  if (content.length > 50) {
    return content.substring(0, 50) + '...';
  }
  return content;
};

// 获取系统统计信息
const fetchStats = async () => {
  try {
    const data = await statsAPI.getStats();
    stats.value = data;
  } catch (error) {
    console.error('Failed to fetch stats:', error);
    ElMessage.error('Failed to fetch statistics');
  }
};

// 获取最近请求
const fetchRecentRequests = async () => {
  try {
    const response = await logsAPI.getLogs({ page: 1, pageSize: 5 });
    if (response.logs && response.logs.length > 0) {
      const formattedRequests = response.logs.map(log => {
        // 解析请求内容以获取消息
        let content = 'No content';
        try {
          const requestData = JSON.parse(log.Request);
          if (requestData.messages && requestData.messages.length > 0) {
            const lastUserMessage = requestData.messages
              .filter(msg => msg.role === 'user')
              .pop();
            if (lastUserMessage && lastUserMessage.content) {
              content = lastUserMessage.content;
            }
          } else if (requestData.prompt) {
            content = requestData.prompt;
          }
        } catch (e) {
          console.error('Failed to parse request:', e);
        }

        return {
          id: log.ID.toString(),
          model: log.ModelName,
          time: formatTime(log.CreatedAt),
          status: 'success', // 假设所有日志都是成功的，实际应用中应该有状态字段
          provider: 'Unknown', // 这里应该从数据库中获取提供者信息
          content: formatContent(content)
        };
      });
      requests.value = formattedRequests;
    }
  } catch (error) {
    console.error('Failed to fetch recent requests:', error);
    // 保持使用模拟数据
  }
};

// 获取提供者统计
const fetchProviderStats = async () => {
  try {
    const data = await statsAPI.getProviderStats();
    providerStats.value = data;
  } catch (error) {
    console.error('Failed to fetch provider stats:', error);
    // 使用模拟数据
    providerStats.value = [
      { providerName: 'OpenAI', requestCount: 153, avgResponseTime: 185 },
      { providerName: 'Anthropic', requestCount: 87, avgResponseTime: 210 },
      { providerName: 'Google AI', requestCount: 64, avgResponseTime: 176 }
    ];
  }
};

// 获取模型统计
const fetchModelStats = async () => {
  try {
    const data = await statsAPI.getModelStats();
    modelStats.value = data;
  } catch (error) {
    console.error('Failed to fetch model stats:', error);
    // 使用模拟数据
    modelStats.value = [
      { modelName: 'gpt-3.5-turbo', requestCount: 98, avgResponseTime: 150 },
      { modelName: 'claude-2', requestCount: 76, avgResponseTime: 195 },
      { modelName: 'gemini-pro', requestCount: 45, avgResponseTime: 165 },
      { modelName: 'gpt-4', requestCount: 35, avgResponseTime: 280 }
    ];
  }
};

// 计算总请求数（用于百分比计算）
const totalRequests = computed(() => {
  return Math.max(stats.value.totalRequests, 1); // 避免除以0
});

// 统计卡片数据
const statCards = computed(() => [
  { title: 'Total Requests', icon: 'Collection', value: stats.value.totalRequests, color: '#42b883' },
  { title: 'Active Providers', icon: 'Connection', value: stats.value.activeProviders, color: '#3b82f6' },
  { title: 'Model Mappings', icon: 'Sort', value: stats.value.modelMappings, color: '#8b5cf6' },
  { title: 'Avg Response Time', icon: 'Timer', value: `${stats.value.avgResponseTime} ms`, color: '#f59e0b' }
]);

// 获取提供者颜色
const getProviderColor = (index) => {
  const colors = ['#42b883', '#3b82f6', '#8b5cf6', '#f59e0b', '#ef4444'];
  return colors[index % colors.length];
};

// 获取模型颜色
const getModelColor = (index) => {
  const colors = ['#8b5cf6', '#3b82f6', '#42b883', '#f59e0b', '#ef4444'];
  return colors[index % colors.length];
};

// 获取所有数据
const fetchAllData = async () => {
  await Promise.all([
    fetchStats(),
    fetchRecentRequests(),
    fetchProviderStats(),
    fetchModelStats()
  ]);
};

onMounted(() => {
  fetchAllData();
  // 每30秒刷新一次数据
  setInterval(fetchAllData, 30000);
});
</script>

<template>
  <div class="dashboard">
    <!-- 统计卡片区域 -->
    <el-row :gutter="20" class="stats-grid">
      <el-col :span="6" v-for="(card, index) in statCards" :key="index">
        <el-card class="stat-card" shadow="hover">
          <div class="stat-content">
            <el-icon :size="24" :style="{ color: card.color }">
              <component :is="card.icon" />
            </el-icon>
            <h3>{{ card.title }}</h3>
            <p class="stat-value">{{ card.value }}</p>
          </div>
        </el-card>
      </el-col>
    </el-row>

    <!-- 图表和数据统计区域 -->
    <el-row :gutter="20" class="charts-row">
      <!-- 提供者统计 -->
      <el-col :span="12">
        <el-card class="stats-chart-card">
          <template #header>
            <div class="card-header">
              <el-icon><PieChart /></el-icon>
              <span>Provider Usage</span>
            </div>
          </template>
          <div class="chart-content">
            <div v-if="providerStats.length > 0" class="chart-data">
              <div v-for="(stat, index) in providerStats" :key="index" class="chart-item">
                <div class="chart-item-header">
                  <span>{{ stat.providerName }}</span>
                  <span class="chart-item-value">{{ stat.requestCount }}</span>
                </div>
                <el-progress
                  :percentage="(stat.requestCount / totalRequests) * 100"
                  :stroke-color="getProviderColor(index)"
                  :show-text="false"
                />
                <div class="chart-item-footer">
                  <el-tooltip content="Average Response Time">
                    <div class="chart-item-time">
                      <Clock :size="14" />
                      <span>{{ stat.avgResponseTime }} ms</span>
                    </div>
                  </el-tooltip>
                </div>
              </div>
            </div>
            <div v-else class="no-data">No provider data available</div>
          </div>
        </el-card>
      </el-col>

      <!-- 模型统计 -->
      <el-col :span="12">
        <el-card class="stats-chart-card">
          <template #header>
            <div class="card-header">
              <el-icon><PieChart /></el-icon>
              <span>Model Usage</span>
            </div>
          </template>
          <div class="chart-content">
            <div v-if="modelStats.length > 0" class="chart-data">
              <div v-for="(stat, index) in modelStats.slice(0, 5)" :key="index" class="chart-item">
                <div class="chart-item-header">
                  <span>{{ stat.modelName }}</span>
                  <span class="chart-item-value">{{ stat.requestCount }}</span>
                </div>
                <el-progress
                  :percentage="(stat.requestCount / totalRequests) * 100"
                  :stroke-color="getModelColor(index)"
                  :show-text="false"
                />
                <div class="chart-item-footer">
                  <el-tooltip content="Average Response Time">
                    <div class="chart-item-time">
                      <Clock :size="14" />
                      <span>{{ stat.avgResponseTime }} ms</span>
                    </div>
                  </el-tooltip>
                </div>
              </div>
            </div>
            <div v-else class="no-data">No model data available</div>
          </div>
        </el-card>
      </el-col>
    </el-row>

    <!-- 最近请求 -->
    <el-card class="recent-requests">
      <template #header>
        <div class="card-header">
          <el-icon><PieChart /></el-icon>
          <span>Recent Requests</span>
          <router-link to="/logs" class="view-all">View All</router-link>
        </div>
      </template>
      
      <el-table :data="requests" style="width: 100%">
        <el-table-column prop="model" label="Model" width="150" />
        <el-table-column prop="provider" label="Provider" width="120" />
        <el-table-column label="Content" min-width="300">
          <template #default="scope">
            <el-tooltip :content="scope.row.content" placement="top">
              <div class="request-content">{{ scope.row.content }}</div>
            </el-tooltip>
          </template>
        </el-table-column>
        <el-table-column prop="time" label="Time" width="120" />
        <el-table-column label="Status" width="80">
          <template #default="scope">
            <el-tag
              :type="scope.row.status === 'success' ? 'success' : 'danger'"
              disable-transitions
              size="small"
            >
              {{ scope.row.status === 'success' ? 'S' : 'F' }}
            </el-tag>
          </template>
        </el-table-column>
      </el-table>
    </el-card>
  </div>
</template>

<style scoped>
.dashboard {
  margin: 0;
  padding: 20px;
  width: 100%;
  min-height: 100vh;
  background-color: #f5f7fa;
}

.stats-grid {
  margin-bottom: 24px;
}

.charts-row {
  margin-bottom: 24px;
}

.stat-card {
  border-radius: 12px;
  border: none;
  transition: all 0.3s ease;
}

.stat-card:hover {
  transform: translateY(-2px);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
}

.stat-content {
  display: flex;
  flex-direction: column;
  align-items: center;
  text-align: center;
  padding: 20px 16px;
}

.stat-content .el-icon {
  margin-bottom: 12px;
  opacity: 0.8;
}

.stat-content h3 {
  margin: 0 0 8px 0;
  font-size: 14px;
  font-weight: 400;
  color: #606266;
}

.stat-value {
  font-size: 28px;
  font-weight: 600;
  margin-bottom: 0;
  color: #303133;
}

.stats-chart-card {
  border-radius: 12px;
  border: none;
  background-color: #ffffff;
}

.recent-requests {
  border-radius: 12px;
  border: none;
  background-color: #ffffff;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 16px 20px;
  font-size: 16px;
  font-weight: 500;
  color: #303133;
}

.card-header .el-icon {
  margin-right: 8px;
}

.card-header span:first-of-type {
  display: flex;
  align-items: center;
}

.view-all {
  color: #42b883;
  text-decoration: none;
  font-size: 14px;
  font-weight: 400;
}

.view-all:hover {
  color: #52c41a;
}

.chart-content {
  padding: 0 20px 20px;
}

.chart-data {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.chart-item {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.chart-item-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 14px;
}

.chart-item-value {
  font-weight: 500;
  color: #606266;
}

.chart-item-footer {
  display: flex;
  justify-content: flex-end;
}

.chart-item-time {
  display: flex;
  align-items: center;
  font-size: 12px;
  color: #909399;
}

.chart-item-time .el-icon {
  margin-right: 4px;
}

.no-data {
  display: flex;
  justify-content: center;
  align-items: center;
  height: 200px;
  color: #909399;
  font-size: 14px;
}

.request-content {
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  font-size: 14px;
  color: #606266;
}

/* 自定义进度条样式 */
.el-progress-bar__outer {
  height: 6px;
  border-radius: 3px;
  background-color: #f0f2f5;
}

.el-progress-bar__inner {
  border-radius: 3px;
}

/* 响应式设计 */
@media (max-width: 768px) {
  .dashboard {
    padding: 12px;
  }
  
  .stats-grid .el-col {
    margin-bottom: 12px;
  }
  
  .charts-row .el-col {
    margin-bottom: 12px;
  }
}
</style>