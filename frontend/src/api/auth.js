import axios from './axios';

/**
 * 用户登录
 * @param {Object} data - 登录信息
 * @param {string} data.username - 用户名
 * @param {string} data.password - 密码
 * @returns {Promise}
 */
export const login = (data) => {
  return axios.post('/login', data);
};

/**
 * 用户登出
 * @returns {Promise}
 */
export const logout = () => {
  return axios.post('/logout');
};

/**
 * 获取用户信息
 * @returns {Promise}
 */
export const getUserInfo = () => {
  return axios.get('/user/info');
};