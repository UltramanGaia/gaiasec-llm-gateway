<script setup>
import { ref, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Expand, Fold, Monitor, Folder, Connection, Warning, Document, User, ArrowDown } from '@element-plus/icons-vue'
import axios from 'axios'

const route = useRoute()
const router = useRouter()

// Sidebar collapse state - default to expanded
const isSidebarCollapsed = ref(false)

// Toggle sidebar collapse state
const toggleSidebar = () => {
  isSidebarCollapsed.value = !isSidebarCollapsed.value
}

const breadcrumbs = computed(() => {
  const routeMap = {
    '/': { name: '仪表板', path: '/' },
    '/providers': { name: '提供商', path: '/providers' },
    '/model-mappings': { name: '模型映射', path: '/model-mappings' },
    '/credentials': { name: '凭证管理', path: '/credentials' },
    '/logs': { name: '请求日志', path: '/logs' }
  }
  
  const items = [{ name: '首页', path: '/' }]
  
  if (route.path !== '/' && routeMap[route.path]) {
    items.push(routeMap[route.path])
  }
  
  return items
})

const handleCommand = async (command) => {
  if (command === 'logout') {
    try {
      await ElMessageBox.confirm('确定要退出登录吗？', '提示', {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      })
      
      // 清除token
      localStorage.removeItem('token')
      // 移除axios的Authorization头
      delete axios.defaults.headers.common['Authorization']
      
      ElMessage.success('已退出登录')
      router.push('/login')
    } catch {
      // 用户取消
    }
  }
}
</script>

<template>
  <el-container class="layout-container">
    <el-aside :width="isSidebarCollapsed ? '64px' : '150px'" class="sidebar" :class="{ collapsed: isSidebarCollapsed }">
      <div class="logo">
        <h2 v-if="!isSidebarCollapsed" class="logo-text">LLM Gateway</h2>
        <el-icon v-else class="collapse-icon" :title="'LLM Gateway'" @click="toggleSidebar"><Expand /></el-icon>
        <el-icon v-if="!isSidebarCollapsed" class="collapse-icon" @click="toggleSidebar"><Fold /></el-icon>
      </div>
      
      <el-menu
        :default-active="$route.path"
        class="sidebar-menu"
        router
        background-color="#304156"
        text-color="#bfcbd9"
        active-text-color="#409eff"
      >
        <el-menu-item index="/">
          <el-tooltip v-if="isSidebarCollapsed" effect="dark" content="仪表板" placement="right">
            <el-icon><Monitor /></el-icon>
          </el-tooltip>
          <template v-else>
            <el-icon><Monitor /></el-icon>
            <span>仪表板</span>
          </template>
        </el-menu-item>
        
        <el-menu-item index="/providers">
          <el-tooltip v-if="isSidebarCollapsed" effect="dark" content="提供商" placement="right">
            <el-icon><Folder /></el-icon>
          </el-tooltip>
          <template v-else>
            <el-icon><Folder /></el-icon>
            <span>提供商</span>
          </template>
        </el-menu-item>
        
        <el-menu-item index="/model-mappings">
          <el-tooltip v-if="isSidebarCollapsed" effect="dark" content="模型映射" placement="right">
            <el-icon><Connection /></el-icon>
          </el-tooltip>
          <template v-else>
            <el-icon><Connection /></el-icon>
            <span>模型映射</span>
          </template>
        </el-menu-item>

        <el-menu-item index="/credentials">
          <el-tooltip v-if="isSidebarCollapsed" effect="dark" content="凭证管理" placement="right">
            <el-icon><Warning /></el-icon>
          </el-tooltip>
          <template v-else>
            <el-icon><Warning /></el-icon>
            <span>凭证管理</span>
          </template>
        </el-menu-item>
        <el-menu-item index="/logs">
          <el-tooltip v-if="isSidebarCollapsed" effect="dark" content="请求日志" placement="right">
            <el-icon><Document /></el-icon>
          </el-tooltip>
          <template v-else>
            <el-icon><Document /></el-icon>
            <span>请求日志</span>
          </template>
        </el-menu-item>
      </el-menu>
    </el-aside>
    
    <el-container>
      <el-header class="header">
        <div class="header-left">
          <el-breadcrumb separator="/">
            <el-breadcrumb-item v-for="item in breadcrumbs" :key="item.path" :to="item.path">
              {{ item.name }}
            </el-breadcrumb-item>
          </el-breadcrumb>
        </div>
        
        <div class="header-right">
          <el-dropdown @command="handleCommand">
            <span class="user-dropdown">
              <el-icon><User /></el-icon>
              User
              <el-icon class="el-icon--right"><ArrowDown /></el-icon>
            </span>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item command="logout">退出登录</el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </div>
      </el-header>
      
      <el-main class="main-content">
        <router-view />
      </el-main>
    </el-container>
  </el-container>
</template>

<style scoped>
.layout-container {
  height: 100vh;
}

.sidebar {
  background-color: #304156;
  overflow: hidden;
  transition: width 0.3s ease;
}

.logo {
  height: 60px;
  display: flex;
  align-items: center;
  justify-content: center;
  background-color: #2b3a4b;
  color: white;
  margin-bottom: 0;
  padding: 0 10px;
  cursor: pointer;
}

.sidebar:not(.collapsed) .logo {
  justify-content: space-between;
}

.logo h2 {
  margin: 0;
  font-size: 16px;
  font-weight: bold;
}

.collapse-icon {
  font-size: 20px;
}

.collapse-button {
  height: 40px;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  background-color: #2b3a4b;
  color: white;
  transition: all 0.3s;
}

.collapse-button:hover {
  background-color: #3a4b5c;
}

.sidebar-menu {
  border: none;
  height: calc(100vh - 100px);
}

.sidebar-menu .el-menu-item {
  height: 50px;
  line-height: 50px;
  display: flex;
  align-items: center;
  padding-left: 0 !important;
  padding-right: 0 !important;
  justify-content: center;
}

.sidebar:not(.collapsed) .sidebar-menu .el-menu-item {
  justify-content: flex-start;
  padding-left: 20px !important;
}

.sidebar.collapsed .sidebar-menu .el-menu-item .el-icon {
  margin-right: 0;
}

.sidebar.collapsed .sidebar-menu .el-menu-item {
  padding: 0 10px !important;
}

.sidebar-menu .el-menu-item span {
  display: inline-block;
  transition: all 0.3s;
  opacity: 1;
  width: auto;
  height: auto;
  overflow: visible;
}

.sidebar.collapsed .sidebar-menu .el-menu-item span {
  opacity: 0;
  width: 0;
  height: 0;
  overflow: hidden;
  display: block;
}

.sidebar.collapsed .logo h2 {
  opacity: 0;
  width: 0;
  height: 0;
  overflow: hidden;
}

.sidebar:not(.collapsed) .collapse-icon {
  opacity: 1;
  width: auto;
  height: auto;
  overflow: visible;
  cursor: pointer;
}

.header {
  background-color: white;
  border-bottom: 1px solid #e4e7ed;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 20px;
}

.header-left {
  flex: 1;
}

.header-right {
  display: flex;
  align-items: center;
}

.user-dropdown {
  display: flex;
  align-items: center;
  cursor: pointer;
  padding: 8px 12px;
  border-radius: 4px;
  transition: background-color 0.3s;
}

.user-dropdown:hover {
  background-color: #f5f7fa;
}

.user-dropdown .el-icon {
  margin-right: 8px;
}

.user-dropdown .el-icon--right {
  margin-left: 8px;
  margin-right: 0;
}

.main-content {
  background-color: #f0f2f5;
  padding: 20px;
}
</style>