<script setup>
import { ref, onMounted, reactive, computed } from 'vue';
import { ElTable, ElTableColumn, ElButton, ElInput, ElSelect, ElOption, ElDatePicker, ElPagination, ElMessage, ElDialog, ElForm, ElFormItem } from 'element-plus';
import { logsAPI, modelMappingsAPI } from '../api';

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

    const data = await logsAPI.getLogs(params);
    logs.value = data.items || data;
    totalLogs.value = data.total || logs.value.length;
  } catch (error) {
    console.error('Failed to fetch logs:', error);
    ElMessage.error('Failed to load logs');
  } finally {
    loading.value = false;
  }
};

const fetchModels = async () => {
  try {
    const data = await modelMappingsAPI.getModelMappings();
    models.value = data.map(m => m.alias);
  } catch (error) {
    console.error('Failed to fetch models:', error);
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
      <el-table-column prop="createdAt" label="Created At" width="180"></el-table-column>
      <el-table-column label="Actions" width="100" fixed="right">
        <template #default="{ row }">{{ row.id }}
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

    <el-dialog title="Log Details" :visible.sync="dialogVisible" width="800px" v-if="currentLog">
      <div class="log-details">
        <div class="log-header">
          <div class="log-item"><strong>ID:</strong> {{ currentLog.id }}</div>
          <div class="log-item"><strong>Model:</strong> {{ currentLog.modelName }}</div>
          <div class="log-item"><strong>User Token:</strong> {{ currentLog.userToken }}</div>
          <div class="log-item"><strong>Created At:</strong> {{ currentLog.createdAt }}</div>
        </div>
        <div class="log-content">
          <div class="log-section">
            <h4>Request</h4>
            <pre>{{ currentLog.request ? JSON.stringify(JSON.parse(currentLog.request), null, 2) : 'N/A' }}</pre>
          </div>
          <div class="log-section">
            <h4>Response</h4>
            <pre>{{ currentLog.response ? JSON.stringify(JSON.parse(currentLog.response), null, 2) : 'N/A' }}</pre>
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
  max-width: 1200px;
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
  max-height: 600px;
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

.log-section h4 {
  margin-top: 0;
  margin-bottom: 10px;
  padding-bottom: 5px;
  border-bottom: 1px solid #eee;
}

pre {
  background-color: #f5f5f5;
  padding: 15px;
  border-radius: 4px;
  overflow-x: auto;
  max-height: 300px;
}
</style>