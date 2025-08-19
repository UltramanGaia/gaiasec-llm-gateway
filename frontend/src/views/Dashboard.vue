<script setup>
import { ref, onMounted } from 'vue';
import { ElRow, ElCol, ElCard, ElTable, ElTableColumn, ElTag } from 'element-plus';
import { statsAPI } from '../api';

const stats = ref({
  totalRequests: 0,
  activeProviders: 0,
  modelMappings: 0,
  avgResponseTime: 0
});

const requests = ref([
  {
    model: 'gpt-3.5-turbo',
    time: '2 min ago',
    status: 'success'
  },
  {
    model: 'claude-2',
    time: '5 min ago',
    status: 'success'
  },
  {
    model: 'gemini-pro',
    time: '10 min ago',
    status: 'error'
  }
]);

const statCards = ref([
  { title: 'Total Requests', icon: 'Collection', value: stats.value.totalRequests },
  { title: 'Active Providers', icon: 'Connection', value: stats.value.activeProviders },
  { title: 'Model Mappings', icon: 'Sort', value: stats.value.modelMappings },
  { title: 'Avg Response Time', icon: 'Timer', value: stats.value.avgResponseTime + ' ms' }
]);

const fetchStats = async () => {
  try {
    const data = await statsAPI.getStats();
    stats.value = data;
    
    // Update stat cards with new values
    statCards.value[0].value = stats.value.totalRequests;
    statCards.value[1].value = stats.value.activeProviders;
    statCards.value[2].value = stats.value.modelMappings;
    statCards.value[3].value = stats.value.avgResponseTime + ' ms';
  } catch (error) {
    console.error('Failed to fetch stats:', error);
  }
};

onMounted(() => {
  fetchStats();
  // Refresh stats every 30 seconds
  setInterval(fetchStats, 30000);
});
</script>

<template>
  <div class="dashboard">
    <el-row :gutter="20" class="stats-grid">
      <el-col :span="6" v-for="(card, index) in statCards" :key="index">
        <el-card class="stat-card">
          <div class="stat-content">
            <el-icon :size="20">
              <component :is="card.icon" />
            </el-icon>
            <h3>{{ card.title }}</h3>
            <p>{{ card.value }}</p>
          </div>
        </el-card>
      </el-col>
    </el-row>
    
    <el-card class="recent-requests">
      <template #header>
        <div class="card-header">
          <span>Recent Requests</span>
          <router-link to="/logs" class="view-all">View All</router-link>
        </div>
      </template>
      
      <el-table :data="requests" style="width: 100%" :show-header="false">
        <el-table-column prop="model" label="Model" />
        <el-table-column prop="time" label="Time" />
        <el-table-column label="Status">
          <template #default="scope">
            <el-tag
              :type="scope.row.status === 'success' ? 'success' : 'danger'"
              disable-transitions
            >
              {{ scope.row.status === 'success' ? 'Success' : 'Failed' }}
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
}



.stats-grid {
  margin-bottom: 30px;
}

.stat-card {
  border-radius: 8px;
}

.stat-content {
  display: flex;
  flex-direction: column;
  align-items: center;
  text-align: center;
}

.stat-content .el-icon {
  margin-bottom: 10px;
  color: #42b883;
}

.stat-content h3 {
  margin: 0 0 10px 0;
  font-size: 16px;
  color: #666;
}

.stat-content p {
  font-size: 24px;
  font-weight: bold;
  margin-bottom: 0;
}

.recent-requests {
  margin-top: 20px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.view-all {
  color: #42b883;
  text-decoration: none;
}
</style>