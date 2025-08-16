import axios from './axios';

/**
 * 获取所有提供者
 * @returns {Promise}
 */
export const getProviders = () => {
  return axios.get('/providers');
};

/**
 * 添加提供者
 * @param {Object} data - 提供者信息
 * @param {string} data.name - 提供者名称
 * @param {string} data.apiKey - API密钥
 * @param {string} data.baseURL - 基础URL
 * @returns {Promise}
 */
export const addProvider = (data) => {
  return axios.post('/providers', data);
};

/**
 * 更新提供者
 * @param {string} id - 提供者ID
 * @param {Object} data - 提供者信息
 * @param {string} data.name - 提供者名称
 * @param {string} data.apiKey - API密钥
 * @param {string} data.baseURL - 基础URL
 * @returns {Promise}
 */
export const updateProvider = (id, data) => {
  return axios.put(`/providers/${id}`, data);
};

/**
 * 删除提供者
 * @param {string} id - 提供者ID
 * @returns {Promise}
 */
export const deleteProvider = (id) => {
  return axios.delete(`/providers/${id}`);
};