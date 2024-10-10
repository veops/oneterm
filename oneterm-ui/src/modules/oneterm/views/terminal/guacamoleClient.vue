<template>
  <div
    :class="[isFullScreen ? 'oneterm-guacamole-full' : 'oneterm-guacamole-panel']"
    ref="onetermGuacamoleRef"
  ></div>
</template>

<script>
import _ from 'lodash'
import Guacamole from 'guacamole-common-js'

const STATE_IDLE = 0
const STATE_CONNECTING = 1
const STATE_WAITING = 2
const STATE_CONNECTED = 3
const STATE_DISCONNECTING = 4
const STATE_DISCONNECTED = 5

export default {
  name: 'GuacamoleClient',
  props: {
    assetId: {
      type: [String, Number],
      default: ''
    },
    accountId: {
      type: [String, Number],
      default: ''
    },
    protocol: {
      type: String,
      default: ''
    },
    isFullScreen: {
      type: Boolean,
      default: true,
    },
    shareId: {
      type: String,
      default: ''
    }
  },
  data() {
    return {
      client: null,
      session_id: null,
      is_monitor: this.$route.query.is_monitor,
      messageKey: 'message'
    }
  },
  mounted() {
    window.addEventListener('resize', this.onWindowResize)
    this.init()
  },
  beforeDestroy() {
    this.$message.destroy(this.messageKey)
    window.removeEventListener('resize', this.onWindowResize)
    if (this.client) {
      this.client.disconnect()
    }
  },
  methods: {
    init() {
      const { session_id, is_monitor } = this.$route.query
      let { asset_id, account_id, protocol: queryProtocol } = this.$route.params

      if (!this.isFullScreen) {
        asset_id = this.assetId
        account_id = this.accountId
        queryProtocol = this.protocol
      }

      const protocol = document.location.protocol.startsWith('https') ? 'wss' : 'ws'

      let socketLink = ''
      if (is_monitor) {
        socketLink = `${protocol}://${document.location.host}/api/oneterm/v1/connect/monitor/${session_id}`
      } else if (this.shareId) {
        socketLink = `${protocol}://${document.location.host}/api/oneterm/v1/share/connect/${this.shareId}`
      } else {
        socketLink = `${protocol}://${document.location.host}/api/oneterm/v1/connect/${asset_id}/${account_id}/${queryProtocol}`
      }

      const tunnel = new Guacamole.WebSocketTunnel(socketLink)
      const client = new Guacamole.Client(tunnel)

      // 处理从虚拟机收到的剪贴板内容
      //   client.onclipboard = handleClipboardReceived

      // 处理客户端的状态变化事件
      client.onstatechange = (state) => {
        this.onClientStateChange(state)
      }

      client.onerror = this.onError
      client.ondisconnect = this.onDisconnect
      tunnel.onerror = this.onError

      // Get display div from document
      const displayEle = this.$refs.onetermGuacamoleRef

      // Add client to display div
      const element = client.getDisplay().getElement()
      displayEle.appendChild(element)

      client.connect(`w=${displayEle.clientWidth}&h=${displayEle.clientHeight}&dpi=96`)
      const display = client.getDisplay()
      display.onresize = function(width, height) {
        display.scale(Math.min(displayEle.clientHeight / display.getHeight(), displayEle.clientWidth / display.getHeight()))
      }

      const sink = new Guacamole.InputSink()
      displayEle.appendChild(sink.getElement())
      sink.focus()

      const keyboard = new Guacamole.Keyboard(sink.getElement())

      keyboard.onkeydown = (keysym) => {
        client.sendKeyEvent(1, keysym)
        if (keysym === 65288) {
          return false
        }
      }
      keyboard.onkeyup = (keysym) => {
        client.sendKeyEvent(0, keysym)
      }

      const sinkFocus = _.debounce(() => {
        sink.focus()
      })

      const mouse = new Guacamole.Mouse(element)

      mouse.onmousedown = mouse.onmouseup = function(mouseState) {
        sinkFocus()
        if (!is_monitor) {
          client.sendMouseState(mouseState)
        }
      }
      mouse.onmousemove = function(mouseState) {
        sinkFocus()
        client.getDisplay().showCursor(false)
        mouseState.x = mouseState.x / display.getScale()
        mouseState.y = mouseState.y / display.getScale()
        if (!is_monitor) {
          client.sendMouseState(mouseState)
        }
      }

      const touch = new Guacamole.Mouse.Touchpad(element) // or Guacamole.Touchscreen

      touch.onmousedown = touch.onmousemove = touch.onmouseup = function(state) {
        if (!is_monitor) {
          client.sendMouseState(state)
        }
      }
      this.client = client
    },
    onClientStateChange(state) {
      console.log('onClientStateChange', state)
      switch (state) {
        case STATE_IDLE:
          this.$message.destroy(this.messageKey)
          this.$message.loading({ content: this.$t('oneterm.guacamole.idle'), duration: 0, key: this.messageKey })
          break
        case STATE_CONNECTING:
          this.$message.destroy(this.messageKey)
          this.$message.loading({ content: this.$t('oneterm.guacamole.connecting'), duration: 0, key: this.messageKey })
          break
        case STATE_WAITING:
          this.$message.destroy(this.messageKey)
          this.$message.loading({ content: this.$t('oneterm.guacamole.waiting'), duration: 0, key: this.messageKey })
          break
        case STATE_CONNECTED:
          this.$message.destroy(this.messageKey)
          this.$message.success({ content: this.$t('oneterm.guacamole.connected'), duration: 3, key: this.messageKey })
          // 向后台发送请求，更新会话的状态
          //   sessionApi.connect(sessionId)
          break
        case STATE_DISCONNECTING:
          break
        case STATE_DISCONNECTED:
        if (this.client) {
            this.client.disconnect()
          }
          const guacamoleClient = this.$refs.onetermGuacamoleRef
          guacamoleClient.innerHTML = ''
          this.$message.destroy(this.messageKey)
          this.$emit('close')
          break
        default:
          break
      }
    },
    onWindowResize() {
      const { client } = this
      const width = client.getDisplay().getWidth()
      const height = client.getDisplay().getHeight()

      const winWidth = window.innerWidth
      const winHeight = window.innerHeight

      const scaleW = winWidth / width
      const scaleH = winHeight / height

      const scale = Math.min(scaleW, scaleH)
      if (!scale) {
        return
      }
      client.getDisplay().scale(scale)
    },
    onError(status) {
      console.log('onError', status)
      this.$message.destroy(this.messageKey)
      this.$emit('close')

      if (status.code > 1000) {
        this.$message.info({
          content: decodeURIComponent(
            atob(status.message)
              .split('')
              .map(function(c) {
                return '%' + ('00' + c.charCodeAt(0).toString(16)).slice(-2)
              })
              .join('')
          ),
        })
      } else if (status.code < 1000 && status.message) {
        this.$message.info(status.message)
      }
    },
    onDisconnect() {
      console.log('onDisconnect')
      this.$emit('close')
    },
  },
}
</script>

<style lang="less" scoped>
.oneterm-guacamole-full {
  position: fixed;
  top: 0;
  left: 0;
  width: 100vw;
  height: 100vh;
  background-color: black;
}

.oneterm-guacamole-panel {
  width: 100%;
  height: 100%;
  background-color: black;
}
</style>

<style lang="less">
.oneterm-guacamole-full {
  & > div {
    margin: 0 auto;
  }
}

.oneterm-guacamole-panel {
  & > div {
    margin: 0 auto;
  }
}
</style>
