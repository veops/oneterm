<template>
  <div class="web-proxy-page" :class="{ 'toolbar-hidden': !showToolbar }">
    <!-- 顶部工具栏 -->
    <div v-if="showToolbar" class="web-proxy-toolbar">
      <div class="toolbar-left">
        <!-- 导航控制 -->
        <a-button-group size="small">
          <a-button @click="goBack" :disabled="!canGoBack" title="后退">
            <a-icon type="left" />
          </a-button>
          <a-button @click="goForward" :disabled="!canGoForward" title="前进">
            <a-icon type="right" />
          </a-button>
          <a-button @click="reload" title="刷新">
            <a-icon type="reload" />
          </a-button>
        </a-button-group>

        <!-- OneTerm标识 -->
        <div class="oneterm-badge">
          <a-icon type="shield" />
          <span>OneTerm安全代理</span>
        </div>
      </div>

      <div class="toolbar-center">
        <!-- 地址栏 -->
        <div class="address-bar">
          <a-icon type="lock" class="security-icon" />
          <span class="protocol">{{ protocol }}://</span>
          <span class="domain">{{ targetDomain }}</span>
          <span class="path">{{ currentPath }}</span>
        </div>
      </div>

      <div class="toolbar-right">
        <!-- 工具菜单 -->
        <a-dropdown placement="bottomRight">
          <a-button size="small" title="更多选项">
            <a-icon type="more" />
          </a-button>
          <a-menu slot="overlay" @click="handleMenuClick">
            <a-menu-item key="hide-toolbar">
              <a-icon type="eye-invisible" />
              隐藏工具栏
            </a-menu-item>
            <a-menu-item key="new-window">
              <a-icon type="export" />
              新窗口打开
            </a-menu-item>
            <a-menu-divider />
            <a-menu-item key="exit" class="danger-item">
              <a-icon type="logout" />
              退出代理
            </a-menu-item>
          </a-menu>
        </a-dropdown>
      </div>
    </div>

    <!-- 悬浮控制按钮（工具栏隐藏时） -->
    <div v-if="!showToolbar" class="floating-controls">
      <a-tooltip title="显示工具栏" placement="left">
        <a-button 
          type="primary" 
          shape="circle" 
          size="large"
          @click="showToolbar = true"
          class="show-toolbar-btn"
        >
          <a-icon type="eye" />
        </a-button>
      </a-tooltip>
      
      <a-tooltip title="退出代理" placement="left">
        <a-button 
          type="danger" 
          shape="circle" 
          size="large"
          @click="exitProxy"
          class="exit-btn"
        >
          <a-icon type="logout" />
        </a-button>
      </a-tooltip>
    </div>

    <!-- Web内容区域 -->
    <div class="web-content-area" id="web-content">
      <!-- 这里会动态加载代理的Web内容 -->
      <div v-if="loading" class="content-loading">
        <a-spin size="large" tip="正在加载页面...">
          <div class="loading-content"></div>
        </a-spin>
      </div>
    </div>
  </div>
</template>

<script>
import { startWebSession } from '@/modules/oneterm/api/connect.js'

export default {
  name: 'WebProxyPage',
  
  data() {
    return {
      showToolbar: true,
      loading: true,
      
      // 会话信息
      assetId: null,
      sessionId: null,
      protocol: 'https',
      targetDomain: '',
      currentPath: '/',
      connectTime: new Date().toLocaleString(),
      
      // 导航历史
      navigationHistory: [],
      historyIndex: -1
    }
  },

  computed: {
    canGoBack() {
      return this.historyIndex > 0
    },
    
    canGoForward() {
      return this.historyIndex < this.navigationHistory.length - 1
    }
  },

  mounted() {
    this.initializeProxy()
    this.setupKeyboardShortcuts()
  },

  methods: {
    // 初始化代理
    async initializeProxy() {
      this.assetId = this.$route.params.assetId
      this.protocol = this.$route.query.protocol || 'https'
      this.targetDomain = this.$route.query.asset_name || 'Web站点'
      
      try {
        // 启动Web会话
        const response = await startWebSession({
          asset_id: this.assetId,
          auth_mode: 'smart'
        })

        if (response.data.success) {
          this.sessionId = response.data.session_id
          
          // 构建完整的代理URL并跳转
          const fullProxyUrl = `/oneterm/v1${response.data.proxy_url}?session_id=${this.sessionId}`
          
          // 直接替换当前页面内容为代理内容
          window.location.replace(fullProxyUrl)
        } else {
          throw new Error(response.data.message || '启动Web会话失败')
        }
      } catch (err) {
        this.$message.error('连接Web站点失败: ' + err.message)
        this.exitProxy()
      }
    },

    // 设置键盘快捷键
    setupKeyboardShortcuts() {
      document.addEventListener('keydown', (e) => {
        // Ctrl+Shift+H: 切换工具栏
        if (e.ctrlKey && e.shiftKey && e.key === 'H') {
          e.preventDefault()
          this.showToolbar = !this.showToolbar
        }
        // Escape: 退出全屏
        else if (e.key === 'Escape') {
          this.exitProxy()
        }
      })
    },

    // 导航控制
    goBack() {
      window.history.back()
    },

    goForward() {
      window.history.forward()
    },

    reload() {
      window.location.reload()
    },

    // 菜单点击处理
    handleMenuClick({ key }) {
      switch (key) {
        case 'hide-toolbar':
          this.showToolbar = false
          break
        case 'new-window':
          window.open(window.location.href, '_blank')
          break
        case 'exit':
          this.exitProxy()
          break
      }
    },

    // 退出代理
    exitProxy() {
      const returnRoute = sessionStorage.getItem('oneterm_return_route') || '/oneterm/workstation'
      this.$router.push(returnRoute)
    }
  }
}
</script>

<style lang="less" scoped>
.web-proxy-page {
  width: 100vw;
  height: 100vh;
  display: flex;
  flex-direction: column;
  background: #fff;
  overflow: hidden;

  &.toolbar-hidden {
    .web-content-area {
      height: 100vh;
    }
  }
}

.web-proxy-toolbar {
  display: flex;
  align-items: center;
  height: 48px;
  padding: 0 16px;
  background: #f8f9fa;
  border-bottom: 1px solid #e1e4e8;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
  z-index: 1000;

  .toolbar-left {
    display: flex;
    align-items: center;
    gap: 12px;

    .oneterm-badge {
      display: flex;
      align-items: center;
      gap: 4px;
      color: #1890ff;
      font-size: 12px;
      font-weight: 500;
      margin-left: 8px;
    }
  }

  .toolbar-center {
    flex: 1;
    display: flex;
    justify-content: center;
    padding: 0 20px;

    .address-bar {
      display: flex;
      align-items: center;
      background: #fff;
      border: 1px solid #d1d5db;
      border-radius: 6px;
      padding: 6px 12px;
      min-width: 400px;
      max-width: 600px;
      font-family: 'Monaco', 'Menlo', monospace;
      font-size: 13px;

      .security-icon {
        color: #52c41a;
        margin-right: 8px;
      }

      .protocol {
        color: #666;
      }

      .domain {
        color: #1890ff;
        font-weight: 500;
      }

      .path {
        color: #333;
      }
    }
  }

  .toolbar-right {
    display: flex;
    align-items: center;
  }
}

.floating-controls {
  position: fixed;
  top: 20px;
  right: 20px;
  display: flex;
  flex-direction: column;
  gap: 12px;
  z-index: 9999;

  .show-toolbar-btn {
    background: rgba(24, 144, 255, 0.9);
    border: none;
    backdrop-filter: blur(10px);
  }

  .exit-btn {
    background: rgba(255, 77, 79, 0.9);
    border: none;
    backdrop-filter: blur(10px);
  }
}

.web-content-area {
  flex: 1;
  position: relative;
  background: #fff;
  
  .content-loading {
    display: flex;
    align-items: center;
    justify-content: center;
    height: 100%;
    
    .loading-content {
      width: 200px;
      height: 100px;
    }
  }
}

.danger-item {
  color: #ff4d4f !important;
}

@media (max-width: 768px) {
  .web-proxy-toolbar {
    height: 44px;
    padding: 0 8px;

    .toolbar-center {
      padding: 0 8px;

      .address-bar {
        min-width: 200px;
        font-size: 12px;
      }
    }

    .oneterm-badge span {
      display: none;
    }
  }

  .floating-controls {
    top: 12px;
    right: 12px;
    gap: 8px;
  }
}
</style> 