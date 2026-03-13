<script setup>
import { computed, ref } from 'vue';
import { ElTag, ElCollapse, ElCollapseItem, ElButton, ElTooltip, ElMessage } from 'element-plus';
import { CopyDocument, ChatDotRound, User, Monitor, Cpu, Document } from '@element-plus/icons-vue';

const props = defineProps({
  data: {
    type: Object,
    default: () => ({})
  },
  expandAll: {
    type: Boolean,
    default: false
  }
});

const activeCollapse = ref([]);

const messages = computed(() => {
  if (!props.data) return [];
  
  if (props.data.messages && Array.isArray(props.data.messages)) {
    return props.data.messages;
  }
  
  if (props.data.prompt) {
    return [{ role: 'user', content: props.data.prompt }];
  }
  
  return [];
});

const tools = computed(() => {
  if (!props.data || !props.data.tools) return [];
  return props.data.tools;
});

const getRoleInfo = (role) => {
  const roleMap = {
    system: { label: 'System', type: 'info', icon: Monitor, color: '#909399' },
    user: { label: 'User', type: 'success', icon: User, color: '#67c23a' },
    assistant: { label: 'Assistant', type: 'warning', icon: ChatDotRound, color: '#e6a23c' }
  };
  return roleMap[role] || { label: role, type: '', icon: Document, color: '#409eff' };
};

const getContentText = (content) => {
  if (!content) return '';
  if (typeof content === 'string') return content;
  if (Array.isArray(content)) {
    return content
      .filter(item => item.type === 'text')
      .map(item => item.text)
      .join('\n');
  }
  return JSON.stringify(content, null, 2);
};

const getContentParts = (content) => {
  if (!content) return [];
  if (typeof content === 'string') {
    return [{ type: 'text', text: content }];
  }
  if (Array.isArray(content)) {
    return content;
  }
  return [{ type: 'text', text: JSON.stringify(content, null, 2) }];
};

const hasToolCalls = (message) => {
  return message.tool_calls && message.tool_calls.length > 0;
};

const copyContent = (text) => {
  navigator.clipboard.writeText(text).then(() => {
    ElMessage.success('Copied to clipboard');
  }).catch(err => {
    console.error('Failed to copy:', err);
  });
};

const formatJson = (obj) => {
  try {
    return JSON.stringify(obj, null, 2);
  } catch {
    return String(obj);
  }
};

const isLongContent = (text) => {
  return text && text.length > 500;
};

const toggleExpand = (index) => {
  const idx = activeCollapse.value.indexOf(index);
  if (idx > -1) {
    activeCollapse.value.splice(idx, 1);
  } else {
    activeCollapse.value.push(index);
  }
};

const isExpanded = (index) => {
  return activeCollapse.value.includes(index) || props.expandAll;
};
</script>

<template>
  <div class="message-viewer">
    <div v-if="tools.length > 0" class="tools-section">
      <div class="section-title">
        <el-icon><Cpu /></el-icon>
        <span>Tools ({{ tools.length }})</span>
      </div>
      <el-collapse>
        <el-collapse-item 
          v-for="(tool, index) in tools" 
          :key="index"
          :name="index"
        >
          <template #title>
            <div class="tool-title">
              <el-tag size="small" type="primary">{{ tool.type || 'function' }}</el-tag>
              <span class="tool-name">{{ tool.function?.name || 'Unknown' }}</span>
            </div>
          </template>
          <div class="tool-detail">
            <div v-if="tool.function?.description" class="tool-description">
              <strong>Description:</strong>
              <p>{{ tool.function.description }}</p>
            </div>
            <div v-if="tool.function?.parameters" class="tool-parameters">
              <strong>Parameters:</strong>
              <pre class="code-block">{{ formatJson(tool.function.parameters) }}</pre>
            </div>
          </div>
        </el-collapse-item>
      </el-collapse>
    </div>

    <div v-if="messages.length > 0" class="messages-section">
      <div class="section-title">
        <el-icon><ChatDotRound /></el-icon>
        <span>Messages ({{ messages.length }})</span>
      </div>
      <div class="messages-list">
        <div 
          v-for="(message, index) in messages" 
          :key="index"
          class="message-item"
          :class="`message-${message.role}`"
        >
          <div class="message-header">
            <div class="role-badge" :style="{ backgroundColor: getRoleInfo(message.role).color }">
              <el-icon><component :is="getRoleInfo(message.role).icon" /></el-icon>
              <span>{{ getRoleInfo(message.role).label }}</span>
            </div>
            <el-button 
              v-if="getContentText(message.content)"
              size="small" 
              text 
              @click="copyContent(getContentText(message.content))"
            >
              <el-icon><CopyDocument /></el-icon>
            </el-button>
          </div>
          
          <div class="message-content">
            <template v-for="(part, pIndex) in getContentParts(message.content)" :key="pIndex">
              <div v-if="part.type === 'text'" class="content-text">
                <div 
                  v-if="isLongContent(part.text)" 
                  class="collapsible-content"
                >
                  <div v-if="!isExpanded(`${index}-${pIndex}`)" class="content-preview">
                    {{ part.text.substring(0, 300) }}...
                    <el-button size="small" text type="primary" @click="toggleExpand(`${index}-${pIndex}`)">
                      Show more
                    </el-button>
                  </div>
                  <div v-else class="content-full">
                    <pre>{{ part.text }}</pre>
                    <el-button size="small" text type="primary" @click="toggleExpand(`${index}-${pIndex}`)">
                      Show less
                    </el-button>
                  </div>
                </div>
                <pre v-else>{{ part.text }}</pre>
              </div>
              <div v-else-if="part.type === 'image_url'" class="content-image">
                <el-tag size="small" type="info">Image</el-tag>
                <span class="image-url">{{ part.image_url?.url || 'Image data' }}</span>
              </div>
              <div v-else class="content-other">
                <el-tag size="small">{{ part.type }}</el-tag>
              </div>
            </template>
          </div>

          <div v-if="hasToolCalls(message)" class="tool-calls">
            <div class="tool-calls-header">
              <el-icon><Cpu /></el-icon>
              <span>Tool Calls</span>
            </div>
            <div 
              v-for="(toolCall, tcIndex) in message.tool_calls" 
              :key="tcIndex"
              class="tool-call-item"
            >
              <div class="tool-call-name">
                <el-tag size="small" type="warning">{{ toolCall.function?.name || 'Unknown' }}</el-tag>
                <span v-if="toolCall.id" class="tool-call-id">ID: {{ toolCall.id }}</span>
              </div>
              <div v-if="toolCall.function?.arguments" class="tool-call-args">
                <pre>{{ formatJson(JSON.parse(toolCall.function.arguments)) }}</pre>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <div v-if="messages.length === 0 && tools.length === 0" class="empty-state">
      <el-icon size="48"><Document /></el-icon>
      <p>No messages or tools found</p>
    </div>
  </div>
</template>

<style scoped>
.message-viewer {
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, sans-serif;
}

.section-title {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 14px;
  font-weight: 600;
  color: #303133;
  margin-bottom: 12px;
  padding-bottom: 8px;
  border-bottom: 1px solid #ebeef5;
}

.section-title .el-icon {
  font-size: 16px;
}

.tools-section {
  margin-bottom: 20px;
}

.tool-title {
  display: flex;
  align-items: center;
  gap: 8px;
}

.tool-name {
  font-weight: 500;
  color: #303133;
}

.tool-detail {
  padding: 12px;
  background: #f5f7fa;
  border-radius: 4px;
}

.tool-description {
  margin-bottom: 12px;
}

.tool-description strong {
  display: block;
  margin-bottom: 4px;
  color: #606266;
}

.tool-description p {
  margin: 0;
  color: #303133;
  line-height: 1.6;
}

.tool-parameters strong {
  display: block;
  margin-bottom: 8px;
  color: #606266;
}

.messages-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.message-item {
  border-radius: 8px;
  border: 1px solid #e4e7ed;
  overflow: hidden;
  transition: all 0.3s;
}

.message-item:hover {
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.08);
}

.message-system {
  background: linear-gradient(135deg, #f5f7fa 0%, #e4e7ed 100%);
  border-left: 3px solid #909399;
}

.message-user {
  background: linear-gradient(135deg, #f0f9eb 0%, #e1f3d8 100%);
  border-left: 3px solid #67c23a;
}

.message-assistant {
  background: linear-gradient(135deg, #fdf6ec 0%, #faecd8 100%);
  border-left: 3px solid #e6a23c;
}

.message-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 12px;
  background: rgba(255, 255, 255, 0.6);
  border-bottom: 1px solid rgba(0, 0, 0, 0.05);
}

.role-badge {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 4px 10px;
  border-radius: 12px;
  color: white;
  font-size: 12px;
  font-weight: 500;
}

.role-badge .el-icon {
  font-size: 14px;
}

.message-content {
  padding: 12px;
}

.content-text pre {
  margin: 0;
  white-space: pre-wrap;
  word-wrap: break-word;
  font-family: 'SF Mono', 'Monaco', 'Inconsolata', 'Fira Code', monospace;
  font-size: 13px;
  line-height: 1.6;
  color: #303133;
}

.collapsible-content {
  position: relative;
}

.content-preview,
.content-full {
  position: relative;
}

.content-full pre {
  max-height: 400px;
  overflow-y: auto;
}

.content-image {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px;
  background: rgba(255, 255, 255, 0.5);
  border-radius: 4px;
}

.image-url {
  font-size: 12px;
  color: #606266;
  word-break: break-all;
}

.content-other {
  padding: 8px;
}

.tool-calls {
  margin-top: 12px;
  padding: 12px;
  background: rgba(255, 255, 255, 0.6);
  border-top: 1px solid rgba(0, 0, 0, 0.05);
}

.tool-calls-header {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  font-weight: 600;
  color: #606266;
  margin-bottom: 8px;
}

.tool-call-item {
  padding: 10px;
  background: #fff;
  border-radius: 6px;
  margin-bottom: 8px;
  border: 1px solid #ebeef5;
}

.tool-call-item:last-child {
  margin-bottom: 0;
}

.tool-call-name {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 8px;
}

.tool-call-id {
  font-size: 11px;
  color: #909399;
  font-family: monospace;
}

.tool-call-args pre {
  margin: 0;
  padding: 8px;
  background: #f5f7fa;
  border-radius: 4px;
  font-size: 12px;
  white-space: pre-wrap;
  word-wrap: break-word;
  max-height: 200px;
  overflow-y: auto;
}

.code-block {
  margin: 8px 0 0 0;
  padding: 12px;
  background: #282c34;
  color: #abb2bf;
  border-radius: 6px;
  font-size: 12px;
  white-space: pre-wrap;
  word-wrap: break-word;
  max-height: 300px;
  overflow-y: auto;
}

.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 40px;
  color: #909399;
}

.empty-state .el-icon {
  margin-bottom: 12px;
  opacity: 0.5;
}

.empty-state p {
  margin: 0;
  font-size: 14px;
}
</style>
