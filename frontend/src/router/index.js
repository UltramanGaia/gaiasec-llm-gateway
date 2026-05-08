import { createRouter, createWebHashHistory } from 'vue-router';
import ModelConfig from '@/views/ModelConfig.vue';
import Logs from '@/views/Logs.vue';
import Traces from '@/views/Traces.vue';

const routes = [
  { path: '/', redirect: '/model-config' },
  { path: '/model-config', component: ModelConfig },
  { path: '/logs', component: Logs },
  { path: '/traces', component: Traces },
];

export default createRouter({
  history: createWebHashHistory(),
  routes,
});
