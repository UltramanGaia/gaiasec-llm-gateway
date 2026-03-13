<script setup>
import { computed, ref } from 'vue';
import { ElTag, ElButton, ElCollapse, ElCollapseItem, ElProgress, ElMessage } from 'element-plus';
import { CopyDocument, ChatDotRound, Cpu, Timer, Coin, Document, Check } from '@element-plus/icons-vue';

const props = defineProps({
  data: {
    type: [Object, String],
    default: () => ({})
  },
  responseTime: {
    type: Number,
    default: 0
  }
});

const parsedData = computed(() => {
  if (!props.data) return null;
  if (typeof props.data === 'string') {
    try {
      return JSON.parse(props.data);
    } catch {
      return { rawText: props.data };
    }
  }
  return props.data;
});

const isOpenAIFormat = computed(() => {
  return parsedData.value?.choices && Array.isArray(parsedData.value.choices);
});

const isClaudeFormat = computed(() => {
  return parsedData.value?.completion || parsedData.value?.content;
});

const choices = computed(() => {
  if (!isOpenAIFormat.value) return [];
  return parsedData.value.choices || [];
});

const usage = computed(() => {
  if (!parsedData.value) return null;
  return parsedData.value.usage || null;
});

const model = computed(() => {
  return parsedData.value?.model || '';
});

const id = computed(() => {
  return parsedData.value?.id || '';
});

const rawCompletion = computed(() => {
  if (isClaudeFormat.value) {
    return parsedData.value.completion || parsedData.value.content || '';
  }
  return '';
});

const getMessageContent = (choice) => {
  if (choice.message?.content) {
    return choice.message.content;
  }
  if (choice.text) {
    return choice.text;
  }
  return '';
};

const getReasoningContent = (choice) => {
  return choice.message?.reasoning_content || choice.delta?.reasoning_content || '';
};

const getToolCalls = (choice) => {
  return choice.message?.tool_calls || [];
};

const getFinishReason = (choice) => {
  return choice.finish_reason || '';
};

const getFinishReasonInfo = (reason) => {
  const reasonMap = {
    stop: { label: 'Stop', type: 'success', desc: 'Natural end' },
    length: { label: 'Length', type: 'warning', desc: 'Max tokens reached' },
    tool_calls: { label: 'Tool Calls', type: 'info', desc: 'Function calling' },
    content_filter: { label: 'Filtered', type: 'danger', desc: 'Content filtered' }
  };
  return reasonMap[reason] || { label: reason, type: '', desc: '' };
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

const formatTime = (ms) => {
  if (!ms) return 'N/A';
  if (ms < 1000) return `${ms}ms`;
  return `${(ms / 1000).toFixed(2)}s`;
};

const expandedChoices = ref([]);

const toggleChoice = (index) => {
  const idx = expandedChoices.value.indexOf(index);
  if (idx > -1) {
    expandedChoices.value.splice(idx, 1);
  } else {
    expandedChoices.value.push(index);
  }
};

const isChoiceExpanded = (index) => {
  return expandedChoices.value.includes(index);
};
</script>

<template>
  <div class="response-viewer">
    <div v-if="!parsedData" class="empty-state">
      <el-icon size="48"><Document /></el-icon>
      <p>No response data</p>
    </div>

    <template v-else>
      <div class="response-meta">
        <div v-if="model" class="meta-item">
          <span class="meta-label">Model:</span>
          <el-tag size="small">{{ model }}</el-tag>
        </div>
        <div v-if="id" class="meta-item">
          <span class="meta-label">ID:</span>
          <span class="meta-value mono">{{ id }}</span>
        </div>
        <div v-if="responseTime" class="meta-item">
          <span class="meta-label">Response Time:</span>
          <el-tag size="small" type="info">
            <el-icon><Timer /></el-icon>
            {{ formatTime(responseTime) }}
          </el-tag>
        </div>
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
        <div v-if="usage.total_tokens" class="usage-bar">
          <div class="bar-label">Token Distribution</div>
          <el-progress 
            :percentage="Math.round((usage.completion_tokens / usage.total_tokens) * 100)" 
            :format="() => `Completion ${usage.completion_tokens}`"
            :stroke-width="12"
          />
        </div>
      </div>

      <div v-if="isOpenAIFormat" class="choices-section">
        <div class="section-title">
          <el-icon><ChatDotRound /></el-icon>
          <span>Response ({{ choices.length }} choice{{ choices.length > 1 ? 's' : '' }})</span>
        </div>
        <div class="choices-list">
          <div 
            v-for="(choice, index) in choices" 
            :key="index"
            class="choice-item"
          >
            <div class="choice-header">
              <div class="choice-index">
                <span>Choice {{ choice.index !== undefined ? choice.index : index }}</span>
                <el-tag 
                  v-if="getFinishReason(choice)" 
                  :type="getFinishReasonInfo(getFinishReason(choice)).type"
                  size="small"
                >
                  {{ getFinishReasonInfo(getFinishReason(choice)).label }}
                </el-tag>
              </div>
              <el-button 
                v-if="getMessageContent(choice)"
                size="small" 
                text 
                @click="copyContent(getMessageContent(choice))"
              >
                <el-icon><CopyDocument /></el-icon>
              </el-button>
            </div>

            <div v-if="getMessageContent(choice)" class="choice-content">
              <pre>{{ getMessageContent(choice) }}</pre>
            </div>

            <div v-if="getReasoningContent(choice)" class="reasoning-content">
              <div class="reasoning-header" @click="toggleChoice(index)">
                <el-icon><Check /></el-icon>
                <span>Reasoning Content</span>
                <el-tag size="small" type="info">{{ isChoiceExpanded(index) ? 'Hide' : 'Show' }}</el-tag>
              </div>
              <div v-if="isChoiceExpanded(index)" class="reasoning-body">
                <pre>{{ getReasoningContent(choice) }}</pre>
              </div>
            </div>

            <div v-if="getToolCalls(choice).length > 0" class="tool-calls-section">
              <div class="tool-calls-header">
                <el-icon><Cpu /></el-icon>
                <span>Tool Calls ({{ getToolCalls(choice).length }})</span>
              </div>
              <div 
                v-for="(toolCall, tcIndex) in getToolCalls(choice)" 
                :key="tcIndex"
                class="tool-call-item"
              >
                <div class="tool-call-header">
                  <el-tag type="warning" size="small">{{ toolCall.function?.name || 'Unknown' }}</el-tag>
                  <span v-if="toolCall.id" class="tool-call-id">{{ toolCall.id }}</span>
                </div>
                <div v-if="toolCall.function?.arguments" class="tool-call-args">
                  <pre>{{ formatJson(JSON.parse(toolCall.function.arguments)) }}</pre>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>

      <div v-else-if="isClaudeFormat" class="claude-section">
        <div class="section-title">
          <el-icon><ChatDotRound /></el-icon>
          <span>Completion</span>
        </div>
        <div class="claude-content">
          <pre>{{ rawCompletion }}</pre>
        </div>
        <div v-if="parsedData.stop_reason" class="stop-reason">
          <span class="label">Stop Reason:</span>
          <el-tag size="small">{{ parsedData.stop_reason }}</el-tag>
        </div>
      </div>

      <div v-else-if="parsedData.rawText" class="raw-section">
        <div class="section-title">
          <el-icon><Document /></el-icon>
          <span>Raw Response</span>
        </div>
        <div class="raw-content">
          <pre>{{ parsedData.rawText }}</pre>
        </div>
      </div>

      <div v-else class="fallback-section">
        <div class="section-title">
          <el-icon><Document /></el-icon>
          <span>Response Data</span>
        </div>
        <pre class="fallback-json">{{ formatJson(parsedData) }}</pre>
      </div>
    </template>
  </div>
</template>

<style scoped>
.response-viewer {
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

.response-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 16px;
  padding: 12px;
  background: #f5f7fa;
  border-radius: 8px;
  margin-bottom: 16px;
}

.meta-item {
  display: flex;
  align-items: center;
  gap: 6px;
}

.meta-label {
  font-size: 12px;
  color: #909399;
}

.meta-value {
  font-size: 13px;
  color: #303133;
}

.meta-value.mono {
  font-family: 'SF Mono', Monaco, monospace;
  font-size: 12px;
}

.usage-section {
  margin-bottom: 16px;
  padding: 12px;
  background: linear-gradient(135deg, #f0f9eb 0%, #e1f3d8 100%);
  border-radius: 8px;
  border: 1px solid #e1f3d8;
}

.usage-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(100px, 1fr));
  gap: 12px;
  margin-bottom: 12px;
}

.usage-item {
  text-align: center;
  padding: 8px;
  background: rgba(255, 255, 255, 0.6);
  border-radius: 6px;
}

.usage-item.total {
  background: rgba(103, 194, 58, 0.2);
}

.usage-item.cached {
  background: rgba(64, 158, 255, 0.2);
}

.usage-label {
  font-size: 11px;
  color: #909399;
  margin-bottom: 4px;
}

.usage-value {
  font-size: 20px;
  font-weight: 600;
  color: #303133;
}

.usage-bar {
  margin-top: 12px;
}

.bar-label {
  font-size: 12px;
  color: #606266;
  margin-bottom: 8px;
}

.choices-section {
  margin-bottom: 16px;
}

.choices-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.choice-item {
  border-radius: 8px;
  border: 1px solid #e4e7ed;
  overflow: hidden;
  background: #fff;
}

.choice-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 12px;
  background: #f5f7fa;
  border-bottom: 1px solid #ebeef5;
}

.choice-index {
  display: flex;
  align-items: center;
  gap: 8px;
  font-weight: 500;
  color: #303133;
}

.choice-content {
  padding: 12px;
}

.choice-content pre {
  margin: 0;
  white-space: pre-wrap;
  word-wrap: break-word;
  font-family: 'SF Mono', Monaco, Inconsolata, monospace;
  font-size: 13px;
  line-height: 1.6;
  color: #303133;
}

.reasoning-content {
  border-top: 1px solid #ebeef5;
}

.reasoning-header {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 10px 12px;
  background: #fdf6ec;
  cursor: pointer;
  font-size: 12px;
  color: #606266;
}

.reasoning-header:hover {
  background: #faecd8;
}

.reasoning-body {
  padding: 12px;
  background: #fefcf3;
}

.reasoning-body pre {
  margin: 0;
  white-space: pre-wrap;
  word-wrap: break-word;
  font-family: 'SF Mono', Monaco, monospace;
  font-size: 12px;
  line-height: 1.5;
  color: #606266;
}

.tool-calls-section {
  padding: 12px;
  background: #f5f7fa;
  border-top: 1px solid #ebeef5;
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

.tool-call-header {
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

.claude-section,
.raw-section,
.fallback-section {
  margin-bottom: 16px;
}

.claude-content,
.raw-content {
  padding: 12px;
  background: #f5f7fa;
  border-radius: 8px;
}

.claude-content pre,
.raw-content pre {
  margin: 0;
  white-space: pre-wrap;
  word-wrap: break-word;
  font-family: 'SF Mono', Monaco, monospace;
  font-size: 13px;
  line-height: 1.6;
  color: #303133;
}

.stop-reason {
  margin-top: 12px;
  display: flex;
  align-items: center;
  gap: 8px;
}

.stop-reason .label {
  font-size: 12px;
  color: #909399;
}

.fallback-json {
  margin: 0;
  padding: 12px;
  background: #282c34;
  color: #abb2bf;
  border-radius: 8px;
  font-size: 12px;
  white-space: pre-wrap;
  word-wrap: break-word;
  max-height: 400px;
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
