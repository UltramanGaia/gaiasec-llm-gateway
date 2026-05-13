import { createRouter, createWebHashHistory } from 'vue-router';
import ModelConfig from '@/views/ModelConfig.vue';
import Logs from '@/views/Logs.vue';

const routes = [
  { path: '/', redirect: '/model-config' },
  { path: '/model-config', component: ModelConfig },
  { path: '/logs', component: Logs },
];

export default createRouter({
  history: createWebHashHistory(),
  routes,
});
