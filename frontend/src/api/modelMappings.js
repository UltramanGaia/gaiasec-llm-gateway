import axios from './axios';

/**
 * 获取所有模型映射
 * @returns {Promise}
 */
export const getModelMappings = () => {
  return axios.get('/model-mappings');
};

/**
 * 添加模型映射
 * @param {Object} data - 模型映射信息
 * @param {string} data.alias - 别名
 * @param {string} data.providerID - 提供者ID
 * @param {string} data.modelName - 模型名称
 * @returns {Promise}
 */
export const addModelMapping = (data) => {
  return axios.post('/model-mappings', data);
};

/**
 * 更新模型映射
 * @param {string} id - 模型映射ID
 * @param {Object} data - 模型映射信息
 * @param {string} data.alias - 别名
 * @param {string} data.providerID - 提供者ID
 * @param {string} data.modelName - 模型名称
 * @returns {Promise}
 */
export const updateModelMapping = (id, data) => {
  return axios.post(`/model-mappings/${id}`, data);
};

/**
 * 删除模型映射
 * @param {string} id - 模型映射ID
 * @returns {Promise}
 */
export const deleteModelMapping = (id) => {
  return axios.delete(`/model-mappings/${id}`);
};