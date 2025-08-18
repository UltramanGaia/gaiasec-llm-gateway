import * as authAPI from './auth';
import * as providersAPI from './providers';
import * as modelMappingsAPI from './modelMappings';
import * as logsAPI from './logs';
import * as statsAPI from './stats';

// 导出所有API模块
export {
  authAPI,
  providersAPI,
  modelMappingsAPI,
  logsAPI,
  statsAPI,
};

// 导出默认对象，方便一次性导入所有API
export default {
  auth: authAPI,
  providers: providersAPI,
  modelMappings: modelMappingsAPI,
  logs: logsAPI,
  stats: statsAPI,
};