import axios from 'axios';
import { ElMessage } from 'element-plus';

const ERROR_MESSAGES = {
  400: '请求参数错误',
  401: '认证失败',
  403: '没有权限',
  404: '资源不存在',
  500: '服务器内部错误',
  502: '网关错误',
  503: '服务暂时不可用',
};

function createApiInstance(baseURL) {
  const instance = axios.create({
    baseURL,
    timeout: 30000,
    headers: { 'Content-Type': 'application/json', Accept: 'application/json' },
  });

  instance.interceptors.response.use(
    (response) => {
      const data = response.data;
      if (data && typeof data.success === 'boolean') {
        if (data.success) {
          return { ...response, data: toCamelCaseDeep(data.data) };
        }
        const err = new Error(data.message || 'Request failed');
        err.response = response;
        throw err;
      }
      return { ...response, data: toCamelCaseDeep(response.data) };
    },
    (error) => {
      const status = error.response?.status;
      const message =
        error.response?.data?.message ||
        ERROR_MESSAGES[status] ||
        '请求失败，请稍后重试';
      ElMessage.error(message);
      return Promise.reject(error);
    }
  );

  return instance;
}

function toCamelKey(key) {
  return key.replace(/_([a-z0-9])/g, (_, c) => c.toUpperCase());
}

function toCamelCaseDeep(value) {
  if (Array.isArray(value)) return value.map(toCamelCaseDeep);
  if (value !== null && typeof value === 'object') {
    const result = {};
    for (const [k, v] of Object.entries(value)) {
      result[toCamelKey(k)] = toCamelCaseDeep(v);
    }
    return result;
  }
  return value;
}

export const api = createApiInstance('/api');
