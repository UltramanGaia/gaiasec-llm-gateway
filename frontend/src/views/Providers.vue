<script setup>
import {onMounted, ref} from 'vue';
import {
  ElButton,
  ElDialog,
  ElForm,
  ElFormItem,
  ElInput,
  ElMessage,
  ElOption,
  ElSelect,
  ElTable,
  ElTableColumn,
  ElTooltip
} from 'element-plus';
import {providersAPI} from '../api';
import {CopyDocument} from '@element-plus/icons-vue';

const providers = ref([]);
const dialogVisible = ref(false);
const isEditing = ref(false);
const currentProvider = ref({ id: '', name: '', apiKey: '', baseURL: '' });
const formRef = ref(null);

// 常见提供商预设
const defaultProviders = ref([
  { name: 'DeepSeek', baseURL: 'https://api.deepseek.com/' },
  { name: '阿里云', baseURL: 'https://dashscope.aliyuncs.com/compatible-mode/v1' },
  { name: 'OpenAI', baseURL: 'https://api.openai.com/v1' },
  { name: 'Anthropic', baseURL: 'https://api.anthropic.com/v1' },
  { name: 'Google Gemini', baseURL: 'https://generativelanguage.googleapis.com/v1beta' }
]);

// 选择预设提供商时自动填充信息
const selectDefaultProvider = (providerName) => {
  if (!providerName) {
    currentProvider.value.name = '';
    currentProvider.value.baseURL = '';
    return;
  }
  
  const provider = defaultProviders.value.find(p => p.name === providerName);
  if (provider) {
    currentProvider.value.name = provider.name;
    currentProvider.value.baseURL = provider.baseURL;
  }
};

const fetchProviders = async () => {
  try {
    // 转换数据字段命名：将大驼峰转换为小驼峰
    providers.value = await providersAPI.getProviders();
  } catch (error) {
    console.error('Failed to fetch providers:', error);
    ElMessage.error('Failed to load providers');
  }
};

const openAddDialog = () => {
  isEditing.value = false;
  currentProvider.value = { id: '', name: '', apiKey: '', baseURL: '' };
  dialogVisible.value = true;
};

const openEditDialog = (provider) => {
  isEditing.value = true;
  currentProvider.value = { ...provider };
  dialogVisible.value = true;
};

const saveProvider = async () => {
  try {
    await formRef.value.validate();
    if (isEditing.value) {
      await providersAPI.updateProvider(currentProvider.value.id, currentProvider.value);
      ElMessage.success('Provider updated successfully');
    } else {
      await providersAPI.addProvider(currentProvider.value);
      ElMessage.success('Provider added successfully');
    }
    dialogVisible.value = false;
    fetchProviders();
  } catch (error) {
    console.error('Failed to save provider:', error);
    ElMessage.error('Failed to save provider');
  }
};

const deleteProvider = async (id) => {
  try {
    if (confirm('Are you sure you want to delete this provider?')) {
      await providersAPI.deleteProvider(id);
      ElMessage.success('Provider deleted successfully');
      fetchProviders();
    }
  } catch (error) {
    console.error('Failed to delete provider:', error);
    ElMessage.error('Failed to delete provider');
  }
};

// 复制API Key到剪贴板
const copyApiKey = async (apiKey) => {
  try {
    await navigator.clipboard.writeText(apiKey);
    ElMessage.success('API Key copied to clipboard');
  } catch (error) {
    console.error('Failed to copy API Key:', error);
    ElMessage.error('Failed to copy API Key');
  }
};

// 格式化API Key为星号显示
const formatApiKey = (apiKey) => {
  if (!apiKey) return '';
  // 保留前6个字符和后4个字符，中间用星号代替
  const prefix = apiKey.slice(0, 6);
  const suffix = apiKey.slice(-4);
  const masked = '*'.repeat(apiKey.length - 10);
  return `${prefix}${masked}${suffix}`;
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

onMounted(() => {
  fetchProviders();
});
</script>

<template>
  <div class="providers">
    <div class="action-bar">
      <el-button type="primary" @click="openAddDialog">Add Provider</el-button>
    </div>
    <el-table :data="providers" style="width: 100%">
      <el-table-column prop="id" label="ID" width="80"></el-table-column>
      <el-table-column prop="name" label="Name" width="180"></el-table-column>
      <el-table-column prop="baseUrl" label="Base URL"></el-table-column>
      <el-table-column prop="apiKey" label="API Key" width="280">
        <template #default="{ row }">
          <div style="display: flex; align-items: center;">
            <span>{{ formatApiKey(row.apiKey) }}</span>
            <el-tooltip content="Copy API Key" placement="top">
              <el-button 
                type="text" 
                size="small" 
                icon="CopyDocument"
                @click.stop="copyApiKey(row.apiKey)"
                style="margin-left: 8px; padding: 0;"
              >
                <CopyDocument />
              </el-button>
            </el-tooltip>
          </div>
        </template>
      </el-table-column>
      <el-table-column prop="createdAt" label="Created At" width="180">
        <template #default="{ row }">
          <span>{{ formatDateTime(row.createdAt) }}</span>
        </template>
      </el-table-column>
      <el-table-column label="Actions" width="150" fixed="right">
        <template #default="{ row }">
          <el-button size="small" @click="openEditDialog(row)">Edit</el-button>
          <el-button size="small" type="danger" @click="deleteProvider(row.id)">Delete</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog :title="isEditing ? 'Edit Provider' : 'Add Provider'" v-model="dialogVisible" width="500px" append-to-body>
      <el-form ref="formRef" :model="currentProvider" label-width="100px">
        <el-form-item label="常见提供商" prop="defaultProvider" v-if="!isEditing">
          <el-select v-model="currentProvider.defaultProvider" placeholder="选择常见提供商" @change="selectDefaultProvider">
            <el-option label="自定义" value=""></el-option>
            <el-option v-for="provider in defaultProviders" :key="provider.name" :label="provider.name" :value="provider.name"></el-option>
          </el-select>
        </el-form-item>
        <el-form-item label="名称" prop="name" :rules="[{ required: true, message: '请输入提供商名称', trigger: 'blur' }]">
          <el-input v-model="currentProvider.name"></el-input>
        </el-form-item>
        <el-form-item label="API Key" prop="apiKey" :rules="[{ required: true, message: '请输入API密钥', trigger: 'blur' }]">
          <el-input v-model="currentProvider.apiKey" type="password"></el-input>
        </el-form-item>
        <el-form-item label="Base URL" prop="baseURL" :rules="[{ required: true, message: '请输入基础URL', trigger: 'blur' }, { type: 'url', message: '请输入有效的URL', trigger: 'blur' }]">
          <el-input v-model="currentProvider.baseURL"></el-input>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible=false">Cancel</el-button>
        <el-button type="primary" @click="saveProvider">Save</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.providers {
  max-width: 1200px;
  margin: 0;
  padding: 20px;
  width: 100%;
}

.action-bar {
  margin-bottom: 20px;
  display: flex;
  justify-content: flex-end;
}
</style>