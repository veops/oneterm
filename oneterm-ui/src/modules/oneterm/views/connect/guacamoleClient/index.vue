<template>
  <div
    :class="[
      isFullScreen ? 'oneterm-guacamole-full' : 'oneterm-guacamole-panel'
    ]"
  >
    <div
      ref="onetermGuacamoleRef"
      class="oneterm-guacamole-wrap"
    ></div>

    <ClipboardModal
      ref="clipboardModalRef"
      @ok="handleClipboardOk"
    />

    <ResolutionModal
      ref="resolutionModalRef"
      @ok="handleResolutionOk"
    />

    <FileManagementDrawer
      ref="fileManagementDrawerRef"
      connectType="rdp"
      :sessionId="sessionId"
    />
  </div>
</template>

<script>
import _ from 'lodash'
import { v4 as uuidv4 } from 'uuid'
import Guacamole from 'guacamole-common-js'
import { pageBeforeUnload } from '@/modules/oneterm/utils/index.js'

import ClipboardModal from './clipboardModal.vue'
import ResolutionModal from './resolutionModal.vue'
import FileManagementDrawer from '../fileManagement/fileManagementDrawer.vue'

const STATE_IDLE = 0
const STATE_CONNECTING = 1
const STATE_WAITING = 2
const STATE_CONNECTED = 3
const STATE_DISCONNECTING = 4
const STATE_DISCONNECTED = 5

export default {
  name: 'GuacamoleClient',
  components: {
    ClipboardModal,
    ResolutionModal,
    FileManagementDrawer
  },
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
    },
    preferenceSetting: {
      type: [Object, null],
      default: null
    },
    controlConfig: {
      type: Object,
      default: () => {}
    }
  },
  data() {
    return {
      client: null,
      messageKey: 'message',
      resizeObserver: null, // guacamoleClient container size observer
      sessionId: '',
    }
  },
  computed: {
    resolution() {
      const resolution = this?.preferenceSetting?.settings?.resolution || 'auto'
      return resolution === 'auto' ? 'auto' : resolution.split('x')
    }
  },
  watch: {
    resolution: {
      handler() {
        this.handleResolutionUpdate()
      },
    }
  },
  async mounted() {
    this.init()
    if (!this.$route?.query?.is_monitor) {
      window.addEventListener('beforeunload', pageBeforeUnload)
    }
  },
  beforeDestroy() {
    this.$message.destroy(this.messageKey)

    if (this.resizeObserver) {
      this.resizeObserver.disconnect()
    }

    if (this.client) {
      this.client.disconnect()
    }

    if (!this.$route?.query?.is_monitor) {
      window.removeEventListener('beforeunload', pageBeforeUnload)
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
      // audit page (online session, offline session)
      if (is_monitor) {
        socketLink = `${protocol}://${document.location.host}/api/oneterm/v1/connect/monitor/${session_id}`
      // share page (temporary link)
      } else if (this.shareId) {
        socketLink = `${protocol}://${document.location.host}/api/oneterm/v1/share/connect/${this.shareId}`
      // work station
      } else {
        socketLink = `${protocol}://${document.location.host}/api/oneterm/v1/connect/${asset_id}/${account_id}/${queryProtocol}`
      }

      const tunnel = new Guacamole.WebSocketTunnel(socketLink)
      const client = new Guacamole.Client(tunnel)

      // 处理从虚拟机收到的剪贴板内容
      client.onclipboard = this.handleClipboardReceived

      if (this?.controlConfig?.[`${queryProtocol?.split?.(':')?.[0]}_config`]?.copy) {
        // handle the clipboard content received from the remote desktop.
        client.onclipboard = this.handleClipboardReceived
      }

      client.onerror = this.onError
      client.ondisconnect = this.onDisconnect
      tunnel.onerror = this.onError

      // Get display div from document
      const displayEle = this.$refs.onetermGuacamoleRef

      // Add client to display div
      const element = client.getDisplay().getElement()
      displayEle.appendChild(element)

      const { width, height } = this.getClientSize()
      let queryString = `w=${width}&h=${height}&dpi=96`
      // workstation page
      if (!is_monitor && !this.shareId) {
        const sessionId = uuidv4()
        this.sessionId = sessionId
        queryString += `&session_id=${sessionId}`
      }

      client.connect(queryString)
      const display = client.getDisplay()
      display.onresize = () => {
        const { width, height } = this.getClientSize()
        display.scale(Math.min(displayEle.clientWidth / width, displayEle.clientHeight / height))
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

      if (this.$refs.onetermGuacamoleRef) {
        this.resizeObserver = new ResizeObserver(_.debounce((entries) => {
          if (entries?.length) {
            this.handleDisplaySize()
          }
        }, 200))
        this.resizeObserver.observe(this.$refs.onetermGuacamoleRef)
      }
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
          this.$emit('open')
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
    handleDisplaySize() {
      const { width, height } = this.getClientSize()

      const guacamoleWrap = this.$refs.onetermGuacamoleRef
      const scaleW = guacamoleWrap.clientWidth / width
      const scaleH = guacamoleWrap.clientHeight / height

      const scale = Math.min(scaleW, scaleH)
      if (!scale) {
        return
      }
      this.client.getDisplay().scale(scale)
    },

    handleResolutionUpdate() {
      const { width, height } = this.getClientSize()
      this.client.sendSize(width, height)
    },

    getClientSize() {
      const guacamoleWrap = this.$refs.onetermGuacamoleRef
      const width = this.resolution === 'auto' ? guacamoleWrap.clientWidth : this.resolution?.[0]
      const height = this.resolution === 'auto' ? guacamoleWrap.clientHeight : this.resolution?.[1]

      return {
        width,
        height
      }
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

    handleClipboardReceived(stream, mimetype) {
      if (/^text\//.exec(mimetype)) {
        const stringReader = new Guacamole.StringReader(stream)
        let str = ''
        stringReader.ontext = function ontext(text) {
          str += text
        }

        stringReader.onend = () => {
          if (str) {
            this.$copyText(str)
          }
        }
      }
    },

    handleClipboardOk(content) {
      if (content?.length) {
        const stream = this.client.createClipboardStream('text/plain')
        const writer = new Guacamole.StringWriter(stream)

        for (let i = 0; i < content.length; i += 4096) {
          writer.sendText(content.substring(i, i + 4096))
        }

        writer.sendEnd()
      }
    },

    handleResolutionOk() {
      this.$emit('updatePreferenceSetting')
    },

    openClipboardModal() {
      this.$refs.clipboardModalRef.open()
    },

    openResolutionModal() {
      const resolution = this?.preferenceSetting?.settings?.resolution || 'auto'
      this.$refs.resolutionModalRef.open(resolution)
    },

    openFileManagementDrawer() {
      this.$refs.fileManagementDrawerRef.open()
    }
  }
}
</script>

<style lang="less" scoped>
.oneterm-guacamole-full {
  position: fixed;
  top: 0;
  left: 0;
  width: 100vw;
  height: 100vh !important;
  background-color: #000000 !important;
  z-index: 1000;
}

.oneterm-guacamole-panel {
  width: 100%;
  height: 100%;
  background-color: #000000;
}

.oneterm-guacamole-wrap {
  width: 100%;
  height: 100%;

  /deep/ & > div {
    margin: 0 auto;
  }
}
</style>
