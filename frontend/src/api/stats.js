import axios from './axios';

/**
 * 获取系统统计信息
 * @returns {Promise}
 */
export const getStats = () => {
  return axios.get('/stats');
};

/**
 * 获取提供者使用统计
 * @returns {Promise}
 */
export const getProviderStats = () => {
  return axios.get('/stats/providers');
};

/**
 * 获取模型使用统计
 * @returns {Promise}
 */
export const getModelStats = () => {
  return axios.get('/stats/models');
};