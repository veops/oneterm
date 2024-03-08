<template>
  <div class="oneterm-guacamole" id="display"></div>
</template>

<script>
import _ from 'lodash'
import Guacamole from 'guacamole-common-js'
import { postConnectIsRight } from '../../api/connect'

const STATE_IDLE = 0
const STATE_CONNECTING = 1
const STATE_WAITING = 2
const STATE_CONNECTED = 3
const STATE_DISCONNECTING = 4
const STATE_DISCONNECTED = 5

export default {
  name: 'GuacamoleClient',
  data() {
    return {
      client: null,
      session_id: null,
      is_monitor: this.$route.query.is_monitor,
    }
  },
  mounted() {
    window.addEventListener('resize', this.onWindowResize)
    const guacamoleClient = document.getElementById('display')
    const { asset_id, account_id, protocol } = this.$route.params
    const { session_id, is_monitor } = this.$route.query
    if (session_id) {
      this.session_id = session_id
      this.init()
    } else {
      postConnectIsRight(
        asset_id,
        account_id,
        protocol,
        `w=${guacamoleClient.clientWidth}&h=${guacamoleClient.clientHeight}&dpi=${96}`
      ).then((res) => {
        this.session_id = res?.data?.session_id
        if (this.session_id) {
          this.init()
        }
      })
    }
  },
  beforeDestroy() {
    window.removeEventListener('resize', this.onWindowResize)
    if (this.client) {
      this.client.disconnect()
    }
  },
  methods: {
    init() {
      const { session_id, is_monitor } = this
      const tunnel = new Guacamole.WebSocketTunnel(
        is_monitor
          ? `ws://${document.location.host}/api/oneterm/v1/connect/monitor/${session_id}`
          : `ws://${document.location.host}/api/oneterm/v1/connect/${session_id}`
      )
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
      const displayEle = document.getElementById('display')

      // Add client to display div
      const element = client.getDisplay().getElement()
      displayEle.appendChild(element)

      client.connect(`w=${displayEle.clientWidth}&h=${displayEle.clientHeight}&dpi=96`)
      const display = client.getDisplay()
      display.onresize = function(width, height) {
        display.scale(Math.min(window.innerHeight / display.getHeight(), window.innerWidth / display.getHeight()))
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
      console.log(state)
      const key = 'message'
      switch (state) {
        case STATE_IDLE:
          this.$message.destroy(key)
          this.$message.loading({ content: this.$t('oneterm.guacamole.idle'), duration: 0, key: key })
          break
        case STATE_CONNECTING:
          this.$message.destroy(key)
          this.$message.loading({ content: this.$t('oneterm.guacamole.connecting'), duration: 0, key: key })
          break
        case STATE_WAITING:
          this.$message.destroy(key)
          this.$message.loading({ content: this.$t('oneterm.guacamole.waiting'), duration: 0, key: key })
          break
        case STATE_CONNECTED:
          this.$message.destroy(key)
          this.$message.success({ content: this.$t('oneterm.guacamole.connected'), duration: 3, key: key })
          // 向后台发送请求，更新会话的状态
          //   sessionApi.connect(sessionId)
          break
        case STATE_DISCONNECTING:
          break
        case STATE_DISCONNECTED:
          if (this.client) {
            this.client.disconnect()
          }
          const guacamoleClient = document.getElementById('display')
          guacamoleClient.innerHTML = ''
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
      console.log(2222)
    },
  },
}
</script>

<style lang="less" scoped>
.oneterm-guacamole {
  position: fixed;
  top: 0;
  left: 0;
  width: 100vw;
  height: 100vh;
  background-color: black;
}
</style>

<style lang="less">
.oneterm-guacamole > div {
  margin: 0 auto;
}
</style>
