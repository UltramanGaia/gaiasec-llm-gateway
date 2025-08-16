<script setup>
import { ref, onMounted } from 'vue';
import { ElButton, ElTable, ElTableColumn, ElMessage, ElTooltip } from 'element-plus';
import { credentialsAPI } from '../api';

const credentials = ref([]);
const newToken = ref('');
const showNewToken = ref(false);

const fetchCredentials = async () => {
  try {
    const data = await credentialsAPI.getCredentials();
    credentials.value = data;
  } catch (error) {
    console.error('Failed to fetch credentials:', error);
    // 如果API调用失败，使用模拟数据作为后备
    credentials.value = [
      { id: 1, token: 'token-123456789', createdAt: '2023-07-15T10:30:00Z', lastUsed: '2023-07-15T11:45:00Z' },
      { id: 2, token: 'token-987654321', createdAt: '2023-07-10T09:15:00Z', lastUsed: null }
    ];
  }
};

const generateNewToken = async () => {
  try {
    const data = await credentialsAPI.generateNewToken();
    newToken.value = data.token;
    showNewToken.value = true;
    ElMessage.success('New token generated successfully');
    // Refresh credentials list
    fetchCredentials();
  } catch (error) {
    console.error('Failed to generate token:', error);
    ElMessage.error('Failed to generate token');
  }
};

const copyToClipboard = (token) => {
  navigator.clipboard.writeText(token).then(() => {
    ElMessage.success('Token copied to clipboard');
  }).catch(err => {
    console.error('Failed to copy token:', err);
    ElMessage.error('Failed to copy token');
  });
};

const revokeToken = async (id) => {
  try {
    if (confirm('Are you sure you want to revoke this token?')) {
      await credentialsAPI.revokeCredential(id);
      // 从本地移除已撤销的凭证
      credentials.value = credentials.value.filter(cred => cred.id !== id);
      ElMessage.success('Token revoked successfully');
    }
  } catch (error) {
    console.error('Failed to revoke token:', error);
    ElMessage.error('Failed to revoke token');
  }
};

onMounted(() => {
  fetchCredentials();
});
</script>

<template>
  <div class="credentials">
    <div class="action-bar">
      <el-button type="primary" @click="generateNewToken">Generate New Token</el-button>
    </div>

    <el-table :data="credentials" style="width: 100%">
      <el-table-column prop="id" label="ID" width="80"></el-table-column>
      <el-table-column prop="token" label="Token" width="300">
        <template #default="{ row }">
          <div class="token-display">
            {{ row.token }}
            <el-tooltip content="Copy to clipboard">
              <el-button size="small" icon="el-icon-copy-document" @click="copyToClipboard(row.token)"></el-button>
            </el-tooltip>
          </div>
        </template>
      </el-table-column>
      <el-table-column prop="createdAt" label="Created At" width="180"></el-table-column>
      <el-table-column prop="lastUsed" label="Last Used" width="180">
        <template #default="{ row }">
          {{ row.lastUsed || 'Never' }}
        </template>
      </el-table-column>
      <el-table-column label="Actions" width="100" fixed="right">
        <template #default="{ row }">{{ row.id }}
          <el-button size="small" type="danger" @click="revokeToken(row.id)">Revoke</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog title="New API Token" :visible.sync="showNewToken" width="500px" v-if="showNewToken">
      <div class="new-token-display">
        <p>Your new API token is:</p>
        <div class="token-value">{{ newToken }}</div>
        <p class="token-warning">Please copy this token now. You will not be able to see it again.</p>
      </div>
      <template #footer>
        <el-button @click="showNewToken = false">Close</el-button>
        <el-button type="primary" @click="copyToClipboard(newToken); showNewToken = false">Copy and Close</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.credentials {
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

.token-display {
  display: flex;
  align-items: center;
}

.token-display .el-button {
  margin-left: 10px;
}

.new-token-display {
  padding: 20px 0;
}

.token-value {
  background-color: #f5f5f5;
  padding: 10px;
  border-radius: 4px;
  font-family: monospace;
  word-break: break-all;
  margin: 10px 0;
}

.token-warning {
  color: #f56c6c;
  font-size: 14px;
  margin-top: 10px;
}
</style>