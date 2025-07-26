<template>
  <div class="oneterm-web-container">
    <!-- 加载状态 -->
    <div v-if="loading" class="oneterm-web-loading">
      <a-spin size="large" tip="正在连接Web站点...">
        <div class="loading-placeholder"></div>
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
        </template>
      </a-result>
    </div>

    <!-- Web内容 -->
    <div v-else class="oneterm-web-content">
      <!-- 工具栏 -->
      <div class="oneterm-web-toolbar">
        <div class="toolbar-left">
          <a-button size="small" @click="reload">
            <a-icon type="reload" />
            刷新
          </a-button>
          <a-button size="small" @click="openInNewWindow">
            <a-icon type="export" />
            新窗口打开
          </a-button>
        </div>
        <div class="toolbar-center">
          <span class="web-url">{{ currentUrl }}</span>
        </div>
        <div class="toolbar-right">
          <a-button size="small" @click="toggleFullscreen">
            <a-icon :type="isFullscreen ? 'fullscreen-exit' : 'fullscreen'" />
            {{ isFullscreen ? '退出全屏' : '全屏' }}
          </a-button>
        </div>
      </div>

      <!-- iframe容器 -->
      <div class="oneterm-web-iframe-container" :class="{ 'fullscreen': isFullscreen }">
        <iframe
          ref="webIframe"
          :src="proxyUrl"
          frameborder="0"
          allowfullscreen
          @load="onIframeLoad"
          @error="onIframeError"
        />
      </div>
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
    },
    isFullScreen: {
      type: Boolean,
      default: false
    }
  },
  data() {
    return {
      loading: true,
      error: null,
      sessionId: '',
      proxyUrl: '',
      currentUrl: '',
      isFullscreen: false,
      retryCount: 0,
      maxRetries: 3
    }
  },
  mounted() {
    this.initWebConnection()
  },
  beforeDestroy() {
    this.cleanup()
  },
  methods: {
    async initWebConnection() {
      this.loading = true
      this.error = null
      this.$emit('webSocketStatus', SOCKET_STATUS.LOADING)

      try {
        const response = await startWebSession({
          asset_id: this.assetId,
          account_id: this.accountId || undefined,
          auth_mode: 'smart' // 默认使用智能认证
        })

        if (response.data.success) {
          this.sessionId = response.data.session_id
          this.proxyUrl = `/oneterm/v1${response.data.proxy_url}/?session_id=${this.sessionId}`
          this.currentUrl = this.proxyUrl
          this.loading = false
          this.$emit('webSocketStatus', SOCKET_STATUS.SUCCESS)
        } else {
          throw new Error(response.data.message || '启动Web会话失败')
        }
      } catch (err) {
        this.error = err.message || '连接Web站点失败'
        this.loading = false
        this.$emit('webSocketStatus', SOCKET_STATUS.ERROR)
        
        // 自动重试
        if (this.retryCount < this.maxRetries) {
          this.retryCount++
          setTimeout(() => {
            this.initWebConnection()
          }, 2000 * this.retryCount)
        }
      }
    },

    onIframeLoad() {
      // iframe加载完成
      this.loading = false
      this.$emit('webSocketStatus', SOCKET_STATUS.SUCCESS)
    },

    onIframeError() {
      this.error = 'Web站点加载失败'
      this.loading = false
      this.$emit('webSocketStatus', SOCKET_STATUS.ERROR)
    },

    reload() {
      if (this.$refs.webIframe) {
        this.$refs.webIframe.src = this.proxyUrl
      }
    },

    retry() {
      this.retryCount = 0
      this.initWebConnection()
    },

    openInNewWindow() {
      if (this.proxyUrl) {
        window.open(this.proxyUrl, '_blank', 'width=1200,height=800,scrollbars=yes,resizable=yes')
      }
    },

    toggleFullscreen() {
      this.isFullscreen = !this.isFullscreen
    },

    cleanup() {
      // 清理资源
      if (this.sessionId) {
        // 可以在这里添加关闭会话的逻辑
      }
    }
  }
}
</script>

<style lang="less" scoped>
.oneterm-web-container {
  width: 100%;
  height: 100%;
  display: flex;
  flex-direction: column;
  background: #f5f5f5;
}

.oneterm-web-loading {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;

  .loading-placeholder {
    width: 300px;
    height: 200px;
  }
}

.oneterm-web-error {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
}

.oneterm-web-content {
  flex: 1;
  display: flex;
  flex-direction: column;
}

.oneterm-web-toolbar {
  display: flex;
  align-items: center;
  padding: 8px 12px;
  background: #fff;
  border-bottom: 1px solid #e8e8e8;
  box-shadow: 0 1px 4px rgba(0, 0, 0, 0.1);

  .toolbar-left {
    flex: 0 0 auto;
    
    .ant-btn {
      margin-right: 8px;
    }
  }

  .toolbar-center {
    flex: 1;
    text-align: center;

    .web-url {
      color: #666;
      font-size: 12px;
      background: #f5f5f5;
      padding: 4px 8px;
      border-radius: 4px;
    }
  }

  .toolbar-right {
    flex: 0 0 auto;
  }
}

.oneterm-web-iframe-container {
  flex: 1;
  position: relative;

  iframe {
    width: 100%;
    height: 100%;
    border: none;
  }

  &.fullscreen {
    position: fixed;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    z-index: 9999;
    background: #fff;
  }
}
</style> 