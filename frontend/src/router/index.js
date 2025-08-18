import { createRouter, createWebHashHistory } from 'vue-router';
import Layout from '../components/Layout.vue';
import Dashboard from '../views/Dashboard.vue';
import Providers from '../views/Providers.vue';
import ModelMappings from '../views/ModelMappings.vue';
import Logs from '../views/Logs.vue';
import Login from '../views/Login.vue';
import axios from 'axios';

const routes = [
  {
    path: '/login',
    name: 'Login',
    component: Login,
    meta: {
      title: '登录',
      requiresAuth: false
    }
  },
  {
    path: '/',
    component: Layout,
    meta: {
      requiresAuth: true
    },
    children: [
      {
        path: '',
        name: 'Dashboard',
        component: Dashboard,
        meta: {
          title: 'Dashboard',
          requiresAuth: true
        }
      },
      {
        path: 'providers',
        name: 'Providers',
        component: Providers,
        meta: {
          title: 'Providers',
          requiresAuth: true
        }
      },
      {
        path: 'model-mappings',
        name: 'ModelMappings',
        component: ModelMappings,
        meta: {
          title: 'Model Mappings',
          requiresAuth: true
        }
      },
      {
        path: 'logs',
        name: 'Logs',
        component: Logs,
        meta: {
          title: 'Request Logs',
          requiresAuth: true
        }
      }
    ]
  }
];

const router = createRouter({
  history: createWebHashHistory(import.meta.env.BASE_URL),
  routes
});

// 全局路由守卫
router.beforeEach((to, from, next) => {
  // 设置页面标题
  if (to.meta.title) {
    document.title = to.meta.title + ' - LLM Gateway';
  }

  // 检查是否需要认证
  if (to.meta.requiresAuth) {
    const token = localStorage.getItem('token');
    
    if (!token) {
      // 没有token，跳转到登录页
      next({ name: 'Login' });
    } else {
      // 设置axios的Authorization头
      axios.defaults.headers.common['Authorization'] = `Bearer ${token}`;
      next();
    }
  } else {
    // 不需要认证的页面直接通过
    next();
  }
});

export default router;