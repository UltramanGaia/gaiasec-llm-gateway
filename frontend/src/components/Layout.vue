<template>
  <el-container class="layout-container" direction="vertical">
    <el-header class="header">
      <div class="header-inner">
        <div class="brand">
          <el-icon style="font-size: 20px; color: var(--el-color-primary);"><Connection /></el-icon>
          <span class="brand-text">LLM Gateway</span>
        </div>
        <el-menu
          :default-active="activeRoute"
          mode="horizontal"
          class="nav-menu"
          :ellipsis="false"
        >
          <el-menu-item index="/model-config" @click="router.push('/model-config')">
            <el-icon><Cpu /></el-icon>模型配置
          </el-menu-item>
          <el-menu-item index="/logs" @click="router.push('/logs')">
            <el-icon><DocumentChecked /></el-icon>请求日志
          </el-menu-item>
        </el-menu>
      </div>
    </el-header>
    <el-main class="main-content">
      <slot />
    </el-main>
  </el-container>
</template>

<script setup>
import { computed } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { Connection, Cpu, DocumentChecked } from '@element-plus/icons-vue';

const route = useRoute();
const router = useRouter();
const activeRoute = computed(() => route.path);
</script>

<style scoped>
.layout-container {
  height: 100vh;
  background: var(--el-bg-color-page);
}

.header {
  height: 60px;
  padding: 0 24px;
  background: var(--el-bg-color);
  border-bottom: 1px solid var(--el-border-color);
  display: flex;
  align-items: center;
}

.header-inner {
  width: 100%;
  display: flex;
  align-items: center;
  gap: 32px;
}

.brand {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
}

.brand-text {
  font-size: 18px;
  font-weight: 700;
  color: var(--el-text-color-primary);
  letter-spacing: 0.02em;
}

.nav-menu {
  border-bottom: none;
  background: transparent;
  flex: 1;
}

.nav-menu :deep(.el-menu-item) {
  height: 60px;
  line-height: 60px;
  border-bottom-width: 3px;
  font-size: 14px;
}

.main-content {
  height: calc(100vh - 60px);
  overflow: auto;
  padding: 20px;
}
</style>
