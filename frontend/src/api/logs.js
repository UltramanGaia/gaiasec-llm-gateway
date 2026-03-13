import axios from './axios';

/**
 * 获取日志列表
 * @param {Object} params - 查询参数
 * @param {string} [params.model] - 模型名称
 * @param {string} [params.userToken] - 用户token
 * @param {string} [params.startDate] - 开始日期
 * @param {string} [params.endDate] - 结束日期
 * @param {number} [params.page=1] - 页码
 * @param {number} [params.pageSize=10] - 每页数量
 * @returns {Promise}
 */
export const getLogs = (params) => {
  return axios.get('/logs', { params });
};

/**
 * 获取单个日志详情
 * @param {string} id - 日志ID
 * @returns {Promise}
 */
export const getLogDetail = (id) => {
  return axios.get(`/logs/${id}`);
};

/**
 * 清空日志
 * @returns {Promise}
 */
export const clearLogs = () => {
  return axios.delete('/logs');
};

/**
 * 重放日志请求
 * @param {string} id - 日志ID
 * @param {Object} [override] - 覆盖参数
 * @returns {Promise}
 */
export const replayLog = (id, override = {}) => {
  return axios.post(`/logs/${id}/replay`, { override });
};