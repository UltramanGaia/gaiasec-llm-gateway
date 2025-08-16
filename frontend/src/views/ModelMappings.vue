<script setup>
import { ref, onMounted } from 'vue';
import { ElDialog, ElForm, ElFormItem, ElInput, ElSelect, ElOption, ElButton, ElTable, ElTableColumn, ElMessage } from 'element-plus';
import { modelMappingsAPI, providersAPI } from '../api';

const modelMappings = ref([]);
const providers = ref([]);
const dialogVisible = ref(false);
const isEditing = ref(false);
const currentMapping = ref({ id: '', alias: '', providerID: '', modelName: '' });
const formRef = ref(null);

const fetchModelMappings = async () => {
  try {
    const data = await modelMappingsAPI.getModelMappings();
    modelMappings.value = data;
  } catch (error) {
    console.error('Failed to fetch model mappings:', error);
    ElMessage.error('Failed to load model mappings');
  }
};

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
  currentMapping.value = { id: '', alias: '', providerID: '', modelName: '' };
  dialogVisible.value = true;
};

const openEditDialog = (mapping) => {
  isEditing.value = true;
  currentMapping.value = { ...mapping };
  dialogVisible.value = true;
};

const saveMapping = async () => {
  try {
    await formRef.value.validate();
    if (isEditing.value) {
      await modelMappingsAPI.updateModelMapping(currentMapping.value.id, currentMapping.value);
      ElMessage.success('Model mapping updated successfully');
    } else {
      await modelMappingsAPI.addModelMapping(currentMapping.value);
      ElMessage.success('Model mapping added successfully');
    }
    dialogVisible.value = false;
    fetchModelMappings();
  } catch (error) {
    console.error('Failed to save model mapping:', error);
    ElMessage.error('Failed to save model mapping');
  }
};

const deleteMapping = async (id) => {
  try {
    if (confirm('Are you sure you want to delete this model mapping?')) {
      await modelMappingsAPI.deleteModelMapping(id);
      ElMessage.success('Model mapping deleted successfully');
      fetchModelMappings();
    }
  } catch (error) {
    console.error('Failed to delete model mapping:', error);
    ElMessage.error('Failed to delete model mapping');
  }
};

onMounted(() => {
  fetchModelMappings();
  fetchProviders();
});
</script>

<template>
  <div class="model-mappings">
    <div class="action-bar">
      <el-button type="primary" @click="openAddDialog">Add Model Mapping</el-button>
    </div>
    <el-table :data="modelMappings" style="width: 100%">
      <el-table-column prop="id" label="ID" width="80"></el-table-column>
      <el-table-column prop="alias" label="Alias" width="180"></el-table-column>
      <el-table-column prop="Provider.name" label="Provider" width="180"></el-table-column>
      <el-table-column prop="modelName" label="Model Name"></el-table-column>
      <el-table-column label="Actions" width="150" fixed="right">
        <template #default="{ row }">{{ row.id }}
          <el-button size="small" @click="openEditDialog(row)">Edit</el-button>
          <el-button size="small" type="danger" @click="deleteMapping(row.id)">Delete</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog :title="isEditing ? 'Edit Model Mapping' : 'Add Model Mapping'" :visible.sync="dialogVisible" width="500px">
      <el-form ref="formRef" :model="currentMapping" label-width="100px">
        <el-form-item label="Alias" prop="alias" :rules="[{ required: true, message: 'Please input model alias', trigger: 'blur' }]">
          <el-input v-model="currentMapping.alias"></el-input>
        </el-form-item>
        <el-form-item label="Provider" prop="providerID" :rules="[{ required: true, message: 'Please select provider', trigger: 'change' }]">
          <el-select v-model="currentMapping.providerID" placeholder="Select provider">
            <el-option v-for="provider in providers" :key="provider.id" :label="provider.name" :value="provider.id"></el-option>
          </el-select>
        </el-form-item>
        <el-form-item label="Model Name" prop="modelName" :rules="[{ required: true, message: 'Please input model name', trigger: 'blur' }]">
          <el-input v-model="currentMapping.modelName"></el-input>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">Cancel</el-button>
        <el-button type="primary" @click="saveMapping">Save</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.model-mappings {
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