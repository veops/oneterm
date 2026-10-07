<template>
  <div
    :class="['oneterm-terminal-container', mode === 'FullScreen' ? 'oneterm-terminal-full' : 'oneterm-terminal-panel']"
    :style="{
      backgroundColor: terminalBackground,
    }"
  >
    <div class="oneterm-terminal-wrap" ref="onetermTerminalRef"></div>

    <CommandDrawer ref="commandDrawerRef" @write="writeCommand" />

    <FileManagementDrawer
      ref="fileManagementDrawerRef"
      connectType="ssh"
      :sessionId="connectData.sessionId"
      :assetPermissions="assetPermissions"
    />
  </div>
</template>

<script>
import _ from 'lodash'
import { v4 as uuidv4 } from 'uuid'
import 'xterm/css/xterm.css'
import { Terminal } from 'xterm'
import { FitAddon } from 'xterm-addon-fit'
import XtermTheme from 'xterm-theme'
import { defaultPreferenceSetting } from '@/modules/oneterm/views/systemSettings/terminalDisplay/constants.js'
import { pageBeforeUnload } from '@/modules/oneterm/utils/index.js'

import CommandDrawer from '@/modules/oneterm/views/systemSettings/quickCommand/commandDrawer.vue'
import FileManagementDrawer from '../fileManagement/fileManagementDrawer.vue'

export const initMessageStorageKey = 'init_oneterm_terminal_msg'

export default {
  name: 'Terminal',
  components: {
    CommandDrawer,
    FileManagementDrawer,
  },
  props: {
    mode: {
      type: String,
      default: 'FullScreen', // FullScreen | Asset | WebSSH
    },
    assetId: {
      type: [String, Number],
      default: '',
    },
    accountId: {
      type: [String, Number],
      default: '',
    },
    protocol: {
      type: String,
      default: '',
    },
    shareId: {
      type: String,
      default: '',
    },
    preferenceSetting: {
      type: [Object, null],
      default: null,
    },
    assetPermissions: {
      type: Object,
      default: () => {},
    },
  },
  data() {
    return {
      term: null,
      websocket: null,
      interval: null,
      initMessage: [],
      terminalBackground: '#000000',
      sessionId: '',

      resizeObserver: null, // terminal container size observer
      resizeHandler: null,
      stdinEnabled: false,
      socketClosed: false,
      receivedOutput: false,
      terminalDestroyed: false,
      flowRequested: false,
      flowControl: false,
      flowReceived: 0,
      flowAckAt: 0,
    }
  },
  computed: {
    connectData() {
      /**
       * fullscreen page (route query)
       * workstation page (props)
       */
      const { asset_id, account_id, protocol, is_monitor, session_id } = this.$route.query

      return {
        assetId: this.assetId || asset_id,
        accountId: this.accountId || account_id,
        protocol: this.protocol || protocol,
        isMonitor: is_monitor,
        sessionId: this.sessionId || session_id,
      }
    },
  },
  async mounted() {
    const { is_monitor } = this.$route.query
    const initMessage = localStorage.getItem(initMessageStorageKey)

    if (initMessage) {
      try {
        const { timestamp, data } = JSON.parse(initMessage) || {}
        if (timestamp && Array.isArray(data) && Date.now() - timestamp < 1000 * 30) {
          this.initMessage = data
        }
      } catch (error) {
        this.initMessage = []
      }
      localStorage.removeItem(initMessageStorageKey)
    }

    await this.initTerm({ disableStdin: !!is_monitor })
    if (this.terminalDestroyed) {
      return
    }
    this.initWebsocket()

    if (!is_monitor) {
      window.addEventListener('beforeunload', pageBeforeUnload)
    }
  },
  beforeDestroy() {
    this.terminalDestroyed = true
    this.socketClosed = true
    this.cleanupSocket()
    if (this.term) {
      this.term.dispose()
      this.term = null
      this.fitAddon = null
    }
    if (!this.$route?.query?.is_monitor) {
      window.removeEventListener('beforeunload', pageBeforeUnload)
    }
  },
  watch: {
    preferenceSetting: {
      deep: true,
      handler(preferenceSetting) {
        if (preferenceSetting) {
          this.updateTermDisplay(preferenceSetting)
        }
      },
    },
  },
  methods: {
    async initTerm({ disableStdin = false }) {
      this.stdinEnabled = !disableStdin
      this.fitAddon = new FitAddon()
      const preferenceSetting = this.preferenceSetting || {}

      const themeObj = XtermTheme?.[preferenceSetting?.theme] || {}
      this.terminalBackground = themeObj?.background || '#000000'

      this.term = new Terminal({
        fontSize: preferenceSetting?.font_size || defaultPreferenceSetting.font_size,
        fontFamily:
          preferenceSetting?.font_family === 'default' || !preferenceSetting?.font_family
            ? 'Consolas, courier-new, courier, monospace'
            : preferenceSetting.font_family,
        cursorStyle: preferenceSetting?.cursor_style || defaultPreferenceSetting.cursor_style,
        letterSpacing: preferenceSetting?.letter_spacing || defaultPreferenceSetting.letter_spacing,
        lineHeight: preferenceSetting?.line_height || defaultPreferenceSetting.line_height,
        cursorBlink: !disableStdin,
        allowProposedApi: true,
        disableStdin: true,
        theme: themeObj,
      })

      this.term.loadAddon(this.fitAddon)
      this.term.open(this.$refs.onetermTerminalRef)

      if (this.mode !== 'WebSSH') {
        this.term.writeln('\x1b[1;1;32mwelcome to oneterm!\x1b[0m')
      }

      if (this?.initMessage?.length) {
        this.initMessage.map((msg) => {
          this.term.writeln(msg)
        })
      }

      if (!disableStdin) {
        this.term.onData((data) => {
          this.sendInput(data)
        })
      }

      this.term.onResize((size) => {
        this.sendSocket(`w${size.cols},${size.rows}`)
      })

      this.fitAddon.fit()
      this.term.focus()
    },

    updateTermDisplay(preferenceSetting) {
      if (!this.term) {
        return
      }

      const options = this.term.options

      options.fontSize = preferenceSetting?.font_size || defaultPreferenceSetting.font_size
      options.fontFamily =
        preferenceSetting?.font_family === 'default' || !preferenceSetting?.font_family
          ? 'Consolas, courier-new, courier, monospace'
          : preferenceSetting.font_family
      options.cursorStyle = preferenceSetting?.cursor_style || defaultPreferenceSetting.cursor_style
      options.letterSpacing = preferenceSetting?.letter_spacing || defaultPreferenceSetting.letter_spacing
      options.lineHeight = preferenceSetting?.line_height || defaultPreferenceSetting.line_height

      const themeObj = XtermTheme?.[preferenceSetting?.theme] || {}
      this.terminalBackground = themeObj?.background || '#000000'
      options.theme = themeObj

      if (this.fitAddon) {
        this.fitAddon.fit()
      }
    },

    initWebsocket() {
      if (!this.term || this.terminalDestroyed) {
        return
      }
      this.cleanupSocket()
      this.socketClosed = false
      this.receivedOutput = false
      const { assetId, accountId, isMonitor, sessionId, protocol: queryProtocol } = this.connectData

      const protocol = document.location.protocol.startsWith('https') ? 'wss' : 'ws'

      let socketLink = ''
      if (this.mode === 'WebSSH') {
        socketLink = `${protocol}://${document.location.host}/api/oneterm/v1/connect/webssh?w=${this.term.cols}&h=${this.term.rows}`
        // audit page (online session, offline session)
      } else if (isMonitor) {
        socketLink = `${protocol}://${document.location.host}/api/oneterm/v1/connect/monitor/${sessionId}?w=${this.term.cols}&h=${this.term.rows}`
        // share page (temporary link)
      } else if (this.shareId) {
        socketLink = `${protocol}://${document.location.host}/api/oneterm/v1/share/connect/${this.shareId}?w=${this.term.cols}&h=${this.term.rows}`
        // work station
      } else {
        const sessionId = uuidv4()
        this.sessionId = sessionId
        socketLink = `${protocol}://${document.location.host}/api/oneterm/v1/connect/${assetId}/${accountId}/${queryProtocol}?w=${this.term.cols}&h=${this.term.rows}&session_id=${sessionId}`
      }

      if (!socketLink) {
        return
      }

      this.flowRequested = !isMonitor
      socketLink += `${socketLink.includes('?') ? '&' : '?'}binary=true`
      if (this.flowRequested) {
        socketLink += '&flow=true'
      }
      this.websocket = new WebSocket(socketLink, ['Sec-WebSocket-Protocol'])

      this.websocket.binaryType = 'arraybuffer'
      this.websocket.onopen = this.websocketOpen
      this.websocket.onmessage = this.getMessage
      this.websocket.onclose = this.closeWebSocket
      this.websocket.onerror = this.errorWebSocket
    },

    websocketOpen(event) {
      if (this.terminalDestroyed || (event?.target && event.target !== this.websocket)) {
        return
      }
      this.socketClosed = false
      this.term.options.disableStdin = !this.stdinEnabled
      this.sendSocket(`w${this.term.cols},${this.term.rows}`)
      this.$emit('open')
      if (this.$refs.onetermTerminalRef) {
        this.resizeHandler = _.debounce(() => this.handleResize(), 200)
        this.resizeObserver = new ResizeObserver(this.resizeHandler)
        this.resizeObserver.observe(this.$refs.onetermTerminalRef)
      }
      this.interval = setInterval(() => this.sendSocket('9'), 10000)
    },

    cleanupSocket() {
      this.flowControl = false
      this.flowReceived = 0
      this.flowAckAt = 0
      if (this.interval) {
        clearInterval(this.interval)
        this.interval = null
      }
      if (this.resizeObserver) {
        this.resizeObserver.disconnect()
        this.resizeObserver = null
      }
      if (this.resizeHandler) {
        this.resizeHandler.cancel()
        this.resizeHandler = null
      }
      if (this.websocket) {
        const socket = this.websocket
        this.websocket = null
        socket.onopen = socket.onmessage = socket.onclose = socket.onerror = null
        socket.close()
      }
    },

    closeWebSocket(event) {
      if (this.socketClosed || this.terminalDestroyed || (event?.target && event.target !== this.websocket)) {
        return
      }
      this.socketClosed = true
      this.cleanupSocket()
      if (this.term) {
        this.term.options.disableStdin = true
        const message = this.receivedOutput
          ? 'The connection is closed.'
          : 'Unable to connect. Check your sign-in and server availability.'
        this.term.writeln(`\r\n\x1b[31m${message}\x1b[0m`)
      }
      this.$emit('close')
    },

    errorWebSocket(event) {
      this.closeWebSocket(event)
    },

    sendSocket(data) {
      const socket = this.websocket
      if (!socket || socket.readyState !== WebSocket.OPEN) {
        return false
      }
      if (socket.bufferedAmount > 1024 * 1024) {
        this.closeWebSocket()
        return false
      }
      try {
        socket.send(data)
        return true
      } catch (error) {
        this.errorWebSocket({ target: socket })
        return false
      }
    },

    sendInput(content) {
      for (let start = 0; start < content.length;) {
        let end = Math.min(start + 8192, content.length)
        const last = content.charCodeAt(end - 1)
        if (end < content.length && last >= 0xd800 && last <= 0xdbff) {
          end--
        }
        if (!this.sendSocket(`1${content.slice(start, end)}`)) {
          return false
        }
        start = end
      }
      return true
    },

    getMessage(message) {
      if (!this.term || this.terminalDestroyed || (message.target && message.target !== this.websocket)) {
        return
      }
      if (this.flowRequested && !this.receivedOutput && message.data === '0flow') {
        this.flowControl = true
        return
      }
      const data = message.data instanceof ArrayBuffer ? new Uint8Array(message.data) : message.data
      if (data?.length) {
        this.receivedOutput = true
        if (this.flowControl && data instanceof Uint8Array) {
          this.flowReceived += data.length
          if (!Number.isSafeInteger(this.flowReceived)) {
            this.closeWebSocket()
            return
          }
          if (this.flowReceived - this.flowAckAt >= 65536) {
            const count = this.flowReceived
            const socket = this.websocket
            this.flowAckAt = count
            this.term.write(data, () => {
              if (!this.terminalDestroyed && socket && socket === this.websocket) {
                this.sendSocket(`a${count}`)
              }
            })
            return
          }
        }
        this.term.write(data)
      }
    },

    handleResize() {
      if (this.fitAddon) {
        this.fitAddon.fit()
      }
    },

    writeCommand(content) {
      this.sendInput(content)
    },

    openCommandDrawer() {
      this.$refs.commandDrawerRef.open()
    },

    openFileManagementDrawer() {
      this.$refs.fileManagementDrawerRef.open()
    },
  },
}
</script>

<style lang="less" scoped>
.oneterm-terminal-container {
  width: 100%;
  height: 100%;
  position: relative;
  padding: 10px;
}

.oneterm-terminal-full {
  position: fixed;
  top: 0;
  left: 0;
  width: 100vw;
  height: 100vh !important;
  z-index: 1000;
}

.oneterm-terminal-wrap {
  width: 100%;
  height: 100%;

  /deep/ .xterm-viewport {
    overflow-y: auto;

    &::-webkit-scrollbar-thumb {
      background-color: @text-color_4;
    }
  }
}
</style>
