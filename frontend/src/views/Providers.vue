<script setup>
import { ref, onMounted } from 'vue';
import { ElDialog, ElForm, ElFormItem, ElInput, ElButton, ElTable, ElTableColumn, ElMessage } from 'element-plus';
import { providersAPI } from '../api';

const providers = ref([]);
const dialogVisible = ref(false);
const isEditing = ref(false);
const currentProvider = ref({ id: '', name: '', apiKey: '', baseURL: '' });
const formRef = ref(null);

const fetchProviders = async () => {
  try {
    const data = await providersAPI.getProviders();
    providers.value = data;
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
      <el-table-column prop="baseURL" label="Base URL"></el-table-column>
      <el-table-column prop="createdAt" label="Created At" width="180"></el-table-column>
      <el-table-column label="Actions" width="150" fixed="right">
        <template #default="{ row }">{{ row.id }}
          <el-button size="small" @click="openEditDialog(row)">Edit</el-button>
          <el-button size="small" type="danger" @click="deleteProvider(row.id)">Delete</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog :title="isEditing ? 'Edit Provider' : 'Add Provider'" :visible.sync="dialogVisible" width="500px">
      <el-form ref="formRef" :model="currentProvider" label-width="100px">
        <el-form-item label="Name" prop="name" :rules="[{ required: true, message: 'Please input provider name', trigger: 'blur' }]">
          <el-input v-model="currentProvider.name"></el-input>
        </el-form-item>
        <el-form-item label="API Key" prop="apiKey" :rules="[{ required: true, message: 'Please input API key', trigger: 'blur' }]">
          <el-input v-model="currentProvider.apiKey" type="password"></el-input>
        </el-form-item>
        <el-form-item label="Base URL" prop="baseURL" :rules="[{ required: true, message: 'Please input base URL', trigger: 'blur' }, { type: 'url', message: 'Please input valid URL', trigger: 'blur' }]">
          <el-input v-model="currentProvider.baseURL"></el-input>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">Cancel</el-button>
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