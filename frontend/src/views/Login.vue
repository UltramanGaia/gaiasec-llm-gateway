<template>
  <div class="login-container">
    <div class="login-form-wrapper">
      <h2 class="login-title">LLM Gateway 登录</h2>
      <el-form
        ref="loginFormRef"
        :model="loginForm"
        :rules="loginRules"
        class="login-form"
      >
        <el-form-item prop="username">
          <el-input
            v-model="loginForm.username"
            placeholder="请输入用户名"
            :prefix-icon="User"
            auto-complete="username"
          />
        </el-form-item>
        <el-form-item prop="password">
          <el-input
            v-model="loginForm.password"
            type="password"
            placeholder="请输入密码"
            :prefix-icon="Lock"
            auto-complete="current-password"
            show-password
          />
        </el-form-item>
        <el-form-item>
          <el-button
            type="primary"
            :loading="loading"
            @click="handleLogin"
            :disabled="loading"
            class="login-button"
          >
            登录
          </el-button>
        </el-form-item>
      </el-form>
      <p class="login-tips">默认账号: admin, 密码: admin123</p>
    </div>
  </div>
</template>

<script setup>
import { ref, reactive } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { User, Lock } from '@element-plus/icons-vue'
import { authAPI } from '../api'
import axios from 'axios';

const router = useRouter()
const loginFormRef = ref(null)
const loading = ref(false)

const loginForm = reactive({
  username: 'admin',
  password: 'admin123'
})

const loginRules = {
  username: [
    { required: true, message: '请输入用户名', trigger: 'blur' }
  ],
  password: [
    { required: true, message: '请输入密码', trigger: 'blur' }
  ]
}

const handleLogin = async () => {
  try {
    // 验证表单
    if(loginFormRef.value){
      await loginFormRef.value.validate()
    }
    loading.value = true

    // 发送登录请求
    const response = await authAPI.login({
      username: loginForm.username,
      password: loginForm.password
    })

    console.log(response)

    // 保存token到localStorage
    localStorage.setItem('token', response.token)

    // 设置axios的默认Authorization头
    axios.defaults.headers.common['Authorization'] = `Bearer ${response.token}`

    ElMessage.success('登录成功')
    // 跳转到首页
    router.push('/')
  } catch (error) {
    console.log(error)
    if (error.response) {
      // 服务器返回错误
      ElMessage.error(error.response.data || '登录失败')
    } else {
      ElMessage.error('网络错误，请稍后重试')
    }
  } finally {
    loading.value = false
  }
}

handleLogin()
</script>

<style scoped>
.login-container {
  display: flex;
  justify-content: center;
  align-items: center;
  min-height: 100vh;
  background-color: #f0f2f5;
}

.login-form-wrapper {
  background: white;
  padding: 40px;
  border-radius: 12px;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
  min-width: 400px;
}

.login-title {
  text-align: center;
  margin-bottom: 30px;
  color: #304156;
  font-size: 24px;
  font-weight: 600;
}

.login-form {
  width: 100%;
}

.login-button {
  width: 100%;
  font-size: 16px;
  padding: 10px;
}

.login-tips {
  text-align: center;
  margin-top: 20px;
  color: #909399;
  font-size: 14px;
}
</style>