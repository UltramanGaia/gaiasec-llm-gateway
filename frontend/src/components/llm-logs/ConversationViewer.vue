<script setup>
import { computed, ref } from 'vue';
import { ElMessage } from 'element-plus';
import { ChatDotRound, Check, Coin, CopyDocument, Cpu, Document, Monitor, User } from '@element-plus/icons-vue';

const props = defineProps({
  request: {
    type: [Object, String],
    default: () => ({}),
  },
  response: {
    type: [Object, String],
    default: () => ({}),
  },
});

const expandedItems = ref([]);

const parsePayload = (payload, fallback = {}) => {
  if (!payload) return fallback;
  if (typeof payload !== 'string') return payload;
  try {
    return JSON.parse(payload);
  } catch {
    return { rawText: payload };
  }
};

const requestData = computed(() => parsePayload(props.request));
const responseData = computed(() => parsePayload(props.response, null));

const tools = computed(() => Array.isArray(requestData.value?.tools) ? requestData.value.tools : []);
const usage = computed(() => responseData.value?.usage || null);

const getContentText = (content) => {
  if (!content) return '';
  if (typeof content === 'string') return content;
  if (Array.isArray(content)) {
    return content
      .filter(item => item.type === 'text')
      .map(item => item.text)
      .join('\n');
  }
  return formatJson(content);
};

const getContentParts = (content) => {
  if (!content) return [];
  if (typeof content === 'string') return [{ type: 'text', text: content }];
  if (Array.isArray(content)) return content;
  return [{ type: 'text', text: formatJson(content) }];
};

const requestMessages = computed(() => {
  if (Array.isArray(requestData.value?.messages)) return requestData.value.messages;
  if (requestData.value?.prompt) return [{ role: 'user', content: requestData.value.prompt }];
  return [];
});

const responseMessages = computed(() => {
  const data = responseData.value;
  if (!data) return [];
  if (Array.isArray(data.choices)) {
    return data.choices.map((choice, index) => ({
      role: choice.message?.role || 'assistant',
      content: choice.message?.content || choice.delta?.content || choice.text || '',
      reasoningContent: choice.message?.reasoning_content || choice.delta?.reasoning_content || '',
      tool_calls: choice.message?.tool_calls || choice.delta?.tool_calls || [],
      finishReason: choice.finish_reason || '',
      choiceIndex: choice.index ?? index,
    }));
  }
  if (data.completion || data.content) {
    return [{
      role: 'assistant',
      content: data.completion || data.content,
      finishReason: data.stop_reason || '',
    }];
  }
  if (data.rawText) {
    return [{ role: 'assistant', content: data.rawText }];
  }
  if (Object.keys(data).length) {
    return [{ role: 'assistant', content: formatJson(data) }];
  }
  return [];
});

const conversation = computed(() => [
  ...requestMessages.value.map((message, index) => ({ ...message, source: 'request', key: `request-${index}` })),
  ...responseMessages.value.map((message, index) => ({ ...message, source: 'response', key: `response-${index}` })),
]);

const getRoleInfo = (role) => {
  const roleMap = {
    system: { label: 'System', icon: Monitor, color: '#909399' },
    user: { label: 'User', icon: User, color: '#67c23a' },
    assistant: { label: 'Assistant', icon: ChatDotRound, color: '#e6a23c' },
    tool: { label: 'Tool', icon: Cpu, color: '#409eff' },
  };
  return roleMap[role] || { label: role || 'Message', icon: Document, color: '#409eff' };
};

const getFinishReasonInfo = (reason) => {
  const reasonMap = {
    stop: { label: 'Stop', type: 'success' },
    length: { label: 'Length', type: 'warning' },
    tool_calls: { label: 'Tool Calls', type: 'info' },
    content_filter: { label: 'Filtered', type: 'danger' },
  };
  return reasonMap[reason] || { label: reason, type: '' };
};

const copyContent = (text) => {
  navigator.clipboard.writeText(text || '').then(() => ElMessage.success('Copied to clipboard'));
};

const formatJson = (obj) => {
  try {
    return JSON.stringify(typeof obj === 'string' ? JSON.parse(obj) : obj, null, 2);
  } catch {
    return String(obj);
  }
};

const isLongContent = (text) => text && text.length > 500;

const toggleExpand = (key) => {
  const index = expandedItems.value.indexOf(key);
  if (index > -1) expandedItems.value.splice(index, 1);
  else expandedItems.value.push(key);
};

const isExpanded = (key) => expandedItems.value.includes(key);
</script>

<template>
  <div class="conversation-viewer">
    <div v-if="tools.length > 0" class="tools-section">
      <div class="section-title">
        <el-icon><Cpu /></el-icon>
        <span>Tools ({{ tools.length }})</span>
      </div>
      <el-collapse>
        <el-collapse-item v-for="(tool, index) in tools" :key="index" :name="index">
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
            <div v-if="tool.function?.parameters">
              <strong>Parameters:</strong>
              <pre class="code-block">{{ formatJson(tool.function.parameters) }}</pre>
            </div>
          </div>
        </el-collapse-item>
      </el-collapse>
    </div>

    <div v-if="usage" class="usage-section">
      <div class="section-title">
        <el-icon><Coin /></el-icon>
        <span>Token Usage</span>
      </div>
      <div class="usage-grid">
        <div class="usage-item">
          <div class="usage-label">Prompt</div>
          <div class="usage-value">{{ usage.prompt_tokens || 0 }}</div>
        </div>
        <div class="usage-item">
          <div class="usage-label">Completion</div>
          <div class="usage-value">{{ usage.completion_tokens || 0 }}</div>
        </div>
        <div class="usage-item total">
          <div class="usage-label">Total</div>
          <div class="usage-value">{{ usage.total_tokens || 0 }}</div>
        </div>
        <div v-if="usage.prompt_cache_hit_tokens" class="usage-item cached">
          <div class="usage-label">Cache Hit</div>
          <div class="usage-value">{{ usage.prompt_cache_hit_tokens }}</div>
        </div>
      </div>
    </div>

    <div v-if="conversation.length > 0" class="conversation-section">
      <div class="section-title">
        <el-icon><ChatDotRound /></el-icon>
        <span>Conversation ({{ conversation.length }})</span>
      </div>

      <div class="messages-list">
        <div
          v-for="(message, index) in conversation"
          :key="message.key"
          class="message-item"
          :class="[`message-${message.role}`, `source-${message.source}`]"
        >
          <div class="message-header">
            <div class="role-badge" :style="{ backgroundColor: getRoleInfo(message.role).color }">
              <el-icon><component :is="getRoleInfo(message.role).icon" /></el-icon>
              <span>{{ getRoleInfo(message.role).label }}</span>
            </div>
            <div class="message-actions">
              <el-tag v-if="message.source === 'response' && message.choiceIndex !== undefined" size="small" type="info">
                Choice {{ message.choiceIndex }}
              </el-tag>
              <el-tag v-if="message.finishReason" :type="getFinishReasonInfo(message.finishReason).type" size="small">
                {{ getFinishReasonInfo(message.finishReason).label }}
              </el-tag>
              <el-button v-if="getContentText(message.content)" size="small" text @click="copyContent(getContentText(message.content))">
                <el-icon><CopyDocument /></el-icon>
              </el-button>
            </div>
          </div>

          <div v-if="message.reasoningContent" class="reasoning-content">
            <div class="reasoning-header" @click="toggleExpand(`${message.key}-reasoning`)">
              <el-icon><Check /></el-icon>
              <span>Reasoning Content</span>
              <el-tag size="small" type="info">{{ isExpanded(`${message.key}-reasoning`) ? 'Hide' : 'Show' }}</el-tag>
            </div>
            <div v-if="isExpanded(`${message.key}-reasoning`)" class="reasoning-body">
              <pre>{{ message.reasoningContent }}</pre>
            </div>
          </div>

          <div class="message-content">
            <template v-for="(part, partIndex) in getContentParts(message.content)" :key="partIndex">
              <div v-if="part.type === 'text'" class="content-text">
                <div v-if="isLongContent(part.text)" class="collapsible-content">
                  <div v-if="!isExpanded(`${message.key}-${partIndex}`)" class="content-preview">
                    {{ part.text.substring(0, 300) }}...
                    <el-button size="small" text type="primary" @click="toggleExpand(`${message.key}-${partIndex}`)">
                      Show more
                    </el-button>
                  </div>
                  <div v-else class="content-full">
                    <pre>{{ part.text }}</pre>
                    <el-button size="small" text type="primary" @click="toggleExpand(`${message.key}-${partIndex}`)">
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

          <div v-if="message.tool_calls?.length" class="tool-calls">
            <div class="tool-calls-header">
              <el-icon><Cpu /></el-icon>
              <span>Tool Calls</span>
            </div>
            <div v-for="(toolCall, toolIndex) in message.tool_calls" :key="toolIndex" class="tool-call-item">
              <div class="tool-call-name">
                <el-tag size="small" type="warning">{{ toolCall.function?.name || 'Unknown' }}</el-tag>
                <span v-if="toolCall.id" class="tool-call-id">ID: {{ toolCall.id }}</span>
              </div>
              <div v-if="toolCall.function?.arguments" class="tool-call-args">
                <pre>{{ formatJson(toolCall.function.arguments) }}</pre>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <div v-else class="empty-state">
      <el-icon size="48"><Document /></el-icon>
      <p>No conversation found</p>
    </div>
  </div>
</template>

<style scoped>
.conversation-viewer { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, sans-serif; }
.section-title { display: flex; align-items: center; gap: 8px; font-size: 14px; font-weight: 600; color: var(--el-text-color-primary, #abb2bf); margin-bottom: 12px; padding-bottom: 8px; border-bottom: 1px solid var(--el-border-color, #3e4451); }
.section-title .el-icon { font-size: 16px; }
.tools-section, .usage-section { margin-bottom: 20px; }
.tool-title { display: flex; align-items: center; gap: 8px; }
.tool-name { font-weight: 500; color: var(--el-text-color-primary, #abb2bf); }
.tool-detail { padding: 12px; background: var(--el-fill-color-light, #21252b); border-radius: 4px; border: 1px solid var(--el-border-color, #3e4451); }
.tool-description { margin-bottom: 12px; }
.tool-detail strong { display: block; margin-bottom: 8px; color: var(--el-text-color-primary, #abb2bf); opacity: 0.8; }
.tool-description p { margin: 0; color: var(--el-text-color-primary, #abb2bf); line-height: 1.6; }
.usage-section { padding: 12px; background: linear-gradient(135deg, #1a2e1a 0%, #162616 100%); border-radius: 8px; border: 1px solid #2d4a2d; }
.usage-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(110px, 1fr)); gap: 12px; }
.usage-item { text-align: center; padding: 8px; background: rgba(0, 0, 0, 0.3); border-radius: 6px; }
.usage-item.total { background: rgba(152, 195, 121, 0.2); }
.usage-item.cached { background: rgba(97, 175, 239, 0.2); }
.usage-label { font-size: 11px; color: var(--el-text-color-primary, #abb2bf); opacity: 0.7; margin-bottom: 4px; }
.usage-value { font-size: 20px; font-weight: 600; color: var(--el-text-color-primary, #abb2bf); }
.messages-list { display: flex; flex-direction: column; gap: 12px; }
.message-item { border-radius: 8px; border: 1px solid var(--el-border-color, #3e4451); overflow: hidden; transition: all 0.3s; }
.message-item:hover { box-shadow: 0 2px 8px rgba(0, 0, 0, 0.3); }
.message-system { background: linear-gradient(135deg, var(--el-fill-color-light, #21252b) 0%, var(--el-fill-color, #282c34) 100%); border-left: 3px solid #5c6370; }
.message-user { background: linear-gradient(135deg, #1e3a1e 0%, #1a2e1a 100%); border-left: 3px solid #98c379; }
.message-assistant { background: linear-gradient(135deg, #3d2f1a 0%, #2d2215 100%); border-left: 3px solid #e5c07b; }
.message-tool { background: linear-gradient(135deg, #1c3040 0%, #162636 100%); border-left: 3px solid #409eff; }
.source-response { margin-top: 4px; }
.message-header { display: flex; align-items: center; justify-content: space-between; gap: 12px; padding: 8px 12px; background: rgba(0, 0, 0, 0.2); border-bottom: 1px solid rgba(255, 255, 255, 0.05); }
.role-badge, .message-actions { display: flex; align-items: center; gap: 6px; }
.role-badge { padding: 4px 10px; border-radius: 12px; color: white; font-size: 12px; font-weight: 500; }
.role-badge .el-icon { font-size: 14px; }
.message-content { padding: 12px; }
.content-text pre, .reasoning-body pre { margin: 0; white-space: pre-wrap; word-wrap: break-word; font-family: 'SF Mono', 'Monaco', 'Inconsolata', 'Fira Code', monospace; font-size: 13px; line-height: 1.6; color: var(--el-text-color-primary, #abb2bf); }
.content-full pre { max-height: 400px; overflow-y: auto; }
.content-image { display: flex; align-items: center; gap: 8px; padding: 8px; background: rgba(0, 0, 0, 0.2); border-radius: 4px; }
.image-url { font-size: 12px; color: var(--el-text-color-primary, #abb2bf); opacity: 0.7; word-break: break-all; }
.content-other { padding: 8px; }
.reasoning-content { border-bottom: 1px solid var(--el-border-color, #3e4451); }
.reasoning-header { display: flex; align-items: center; gap: 6px; padding: 10px 12px; background: #3d2f1a; cursor: pointer; font-size: 12px; color: var(--el-text-color-primary, #abb2bf); }
.reasoning-header:hover { background: #4d3a22; }
.reasoning-body { padding: 12px; background: #2d2215; }
.reasoning-body pre { font-size: 12px; line-height: 1.5; opacity: 0.9; }
.tool-calls { margin-top: 12px; padding: 12px; background: rgba(0, 0, 0, 0.2); border-top: 1px solid rgba(255, 255, 255, 0.05); }
.tool-calls-header { display: flex; align-items: center; gap: 6px; font-size: 12px; font-weight: 600; color: var(--el-text-color-primary, #abb2bf); opacity: 0.8; margin-bottom: 8px; }
.tool-call-item { padding: 10px; background: var(--el-bg-color, #181818); border-radius: 6px; margin-bottom: 8px; border: 1px solid var(--el-border-color, #3e4451); }
.tool-call-item:last-child { margin-bottom: 0; }
.tool-call-name { display: flex; align-items: center; gap: 8px; margin-bottom: 8px; }
.tool-call-id { font-size: 11px; color: var(--el-text-color-primary, #abb2bf); opacity: 0.5; font-family: monospace; }
.tool-call-args pre, .code-block { margin: 0; padding: 8px; background: var(--el-fill-color-light, #21252b); border-radius: 4px; font-size: 12px; white-space: pre-wrap; word-wrap: break-word; max-height: 300px; overflow-y: auto; color: var(--el-text-color-primary, #abb2bf); }
.code-block { margin-top: 8px; background: #282c34; color: #abb2bf; border-radius: 6px; }
.empty-state { display: flex; flex-direction: column; align-items: center; justify-content: center; padding: 40px; color: var(--el-text-color-secondary, #5c6370); }
.empty-state .el-icon { margin-bottom: 12px; opacity: 0.5; }
.empty-state p { margin: 0; font-size: 14px; }
@media (max-width: 720px) {
  .message-header { align-items: flex-start; flex-direction: column; }
  .message-actions { flex-wrap: wrap; }
}
</style>
