import axios from './axios';

/**
 * 获取所有凭证
 * @returns {Promise}
 */
export const getCredentials = () => {
  return axios.get('/credentials');
};

/**
 * 生成新的API令牌
 * @returns {Promise}
 */
export const generateNewToken = () => {
  return axios.post('/credentials/generate');
};

/**
 * 撤销凭证
 * @param {string|number} id - 凭证ID
 * @returns {Promise}
 */
export const revokeCredential = (id) => {
  return axios.delete(`/credentials/${id}`);
};