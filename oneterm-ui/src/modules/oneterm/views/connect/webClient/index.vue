<template>
  <div class="oneterm-web-container">
    <!-- 加载状态 -->
    <div v-if="loading" class="oneterm-web-loading">
      <a-spin size="large" tip="正在连接Web站点...">
        <div class="loading-placeholder">
          <a-icon type="global" style="font-size: 48px; color: #1890ff;" />
          <p>正在建立安全连接...</p>
        </div>
      </a-spin>
    </div>

    <!-- 错误状态 -->
    <div v-else-if="error" class="oneterm-web-error">
      <a-result
        status="error"
        :title="error"
        sub-title="连接Web站点时发生错误"
      >
        <template #extra>
          <a-button type="primary" @click="retry">重试连接</a-button>
          <a-button @click="goBack">返回资产列表</a-button>
        </template>
      </a-result>
    </div>

    <!-- 准备跳转提示 -->
    <div v-else-if="readyToRedirect" class="oneterm-web-redirect">
      <a-result
        status="success"
        title="连接建立成功"
        sub-title="即将跳转到Web站点..."
      >
        <template #extra>
          <a-button type="primary" @click="redirectToWebProxy">
            立即访问
          </a-button>
          <a-button @click="openInNewTab">
            新标签页打开
          </a-button>
        </template>
      </a-result>
    </div>
  </div>
</template>

<script>
import { startWebSession } from '@/modules/oneterm/api/connect.js'
import { SOCKET_STATUS } from '../../workStation/constants.js'

export default {
  name: 'WebPanel',
  props: {
    assetId: {
      type: [String, Number],
      required: true
    },
    accountId: {
      type: [String, Number],
      default: null
    },
    protocol: {
      type: String,
      required: true
    },
    assetPermissions: {
      type: Object,
      default: () => ({})
    }
  },
  
  data() {
    return {
      loading: true,
      error: null,
      readyToRedirect: false,
      sessionId: '',
      proxyUrl: '',
      retryCount: 0,
      maxRetries: 3
    }
  },

  mounted() {
    this.initWebConnection()
  },

  methods: {
    async initWebConnection() {
      this.loading = true
      this.error = null
      this.readyToRedirect = false
      
      try {
        // 启动Web会话
        const response = await startWebSession({
          asset_id: this.assetId,
          auth_mode: 'smart' // 后端智能选择账号
        })

        if (response.data.success) {
          this.sessionId = response.data.session_id
          this.proxyUrl = response.data.proxy_url
          
          // 显示准备跳转状态
          this.loading = false
          this.readyToRedirect = true
          
          // 发送连接成功事件
          this.$emit('webSocketStatus', SOCKET_STATUS.SUCCESS)
          
          // 自动跳转（可配置延迟）
          setTimeout(() => {
            this.redirectToWebProxy()
          }, 1500)
          
        } else {
          throw new Error(response.data.message || '启动Web会话失败')
        }
        
      } catch (err) {
        this.error = err.response?.data?.message || err.message || '连接Web站点失败'
        this.loading = false
        this.$emit('webSocketStatus', SOCKET_STATUS.ERROR)
      }
    },

    // 跳转到Web代理页面（当前标签页）
    redirectToWebProxy() {
      const fullProxyUrl = `/oneterm/v1${this.proxyUrl}?session_id=${this.sessionId}`
      
      // 保存当前路由信息，方便返回
      sessionStorage.setItem('oneterm_return_route', this.$route.fullPath)
      sessionStorage.setItem('oneterm_web_session', JSON.stringify({
        assetId: this.assetId,
        sessionId: this.sessionId,
        protocol: this.protocol
      }))
      
      // 直接跳转到代理URL
      window.location.href = fullProxyUrl
    },

    // 在新标签页打开
    openInNewTab() {
      const fullProxyUrl = `/oneterm/v1${this.proxyUrl}?session_id=${this.sessionId}`
      window.open(fullProxyUrl, '_blank')
      
      // 跳转后返回工作台
      this.goBack()
    },

    // 重试连接
    async retry() {
      if (this.retryCount < this.maxRetries) {
        this.retryCount++
        await this.initWebConnection()
      } else {
        this.$message.error('重试次数过多，请检查网络连接或联系管理员')
      }
    },

    // 返回资产列表
    goBack() {
      this.$emit('close')
      // 或者使用路由返回
      // this.$router.go(-1)
    }
  }
}
</script>

<style lang="less" scoped>
.oneterm-web-container {
  width: 100%;
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(135deg, #f5f7fa 0%, #c3cfe2 100%);
}

.oneterm-web-loading {
  text-align: center;
  
  .loading-placeholder {
    padding: 40px;
    
    p {
      margin-top: 16px;
      color: #666;
      font-size: 16px;
    }
  }
}

.oneterm-web-error {
  max-width: 500px;
}

.oneterm-web-redirect {
  max-width: 500px;
  
  .ant-result {
    padding: 48px 32px;
  }
}
</style> 