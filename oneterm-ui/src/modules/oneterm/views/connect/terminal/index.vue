<template>
  <div
    :class="[
      'oneterm-terminal-container',
      isFullScreen ? 'oneterm-terminal-full' : 'oneterm-terminal-panel'
    ]"
    :style="{
      backgroundColor: terminalBackground
    }"
  >
    <div
      class="oneterm-terminal-wrap"
      ref="onetermTerminalRef"
    ></div>

    <CommandDrawer
      ref="commandDrawerRef"
      @write="writeCommand"
    />

    <FileManagementDrawer
      ref="fileManagementDrawerRef"
      connectType="ssh"
      :sessionId="connectData.sessionId"
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
    }
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
        sessionId: this.sessionId || session_id
      }
    }
  },
  async mounted() {
    const { is_monitor } = this.$route.query
    const initMessage = localStorage.getItem(initMessageStorageKey)

    if (initMessage) {
      const { timestamp, data } = JSON.parse(initMessage) || {}
      if (
        timestamp &&
        data &&
        new Date().getTime() - timestamp < 1000 * 30
      ) {
        this.initMessage = data
      }

      localStorage.removeItem(initMessageStorageKey)
    }

    await this.initTerm({ disableStdin: !!is_monitor })
    this.initWebsocket()

    if (!is_monitor) {
      window.addEventListener('beforeunload', pageBeforeUnload)
    }
  },
  beforeDestroy() {
    if (this.resizeObserver) {
      this.resizeObserver.disconnect()
    }

    if (this.websocket) {
      this.websocket.close()
      this.webSocket = null
    }

    if (this.fitAddon) {
      this.fitAddon.dispose()
    }

    if (this.term) {
      this.term.dispose()
      this.term = null
    }

    if (this.interval) {
      clearInterval(this.interval)
      this.interval = null
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
    }
  },
  methods: {
    async initTerm({ disableStdin = false }) {
      this.fitAddon = new FitAddon()
      const preferenceSetting = this.preferenceSetting || {}

      const themeObj = XtermTheme?.[preferenceSetting?.theme] || {}
      this.terminalBackground = themeObj?.background || '#000000'

      this.term = new Terminal({
        fontSize: preferenceSetting?.font_size || defaultPreferenceSetting.font_size,
        fontFamily: preferenceSetting?.font_family === 'default' || !preferenceSetting?.font_family ? 'Consolas, courier-new, courier, monospace' : preferenceSetting.font_family,
        cursorStyle: preferenceSetting?.cursor_style || defaultPreferenceSetting.cursor_style,
        letterSpacing: preferenceSetting?.letter_spacing || defaultPreferenceSetting.letter_spacing,
        lineHeight: preferenceSetting?.line_height || defaultPreferenceSetting.line_height,
        cursorBlink: !disableStdin,
        allowProposedApi: true,
        disableStdin: disableStdin,
        theme: themeObj
      })

      this.term.loadAddon(this.fitAddon)
      this.term.open(this.$refs.onetermTerminalRef)

      this.term.writeln('\x1b[1;1;32mwelcome to oneterm!\x1b[0m')

      if (this?.initMessage?.length) {
        this.initMessage.map((msg) => {
          this.term.writeln(msg)
        })
      }

      if (!disableStdin) {
        this.term.onData((data) => {
          if (this.websocket) {
            this.websocket.send(`1${data}`)
          }
        })
      }

      this.term.onResize((size) => {
        if (this.websocket) {
          this.websocket.send(`w${size.cols},${size.rows}`)
        }
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
      options.fontFamily = preferenceSetting?.font_family === 'default' || !preferenceSetting?.font_family ? 'Consolas, courier-new, courier, monospace' : preferenceSetting.font_family
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
      const {
        assetId,
        accountId,
        isMonitor,
        sessionId,
        protocol: queryProtocol
      } = this.connectData

      const protocol = document.location.protocol.startsWith('https') ? 'wss' : 'ws'

      let socketLink = ''
      // audit page (online session, offline session)
      if (isMonitor) {
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

      this.websocket = new WebSocket(
        socketLink,
        ['Sec-WebSocket-Protocol']
      )

      this.websocket.onopen = this.websocketOpen
      this.websocket.onmessage = this.getMessage
      this.websocket.onclose = this.closeWebSocket
      this.websocket.onerror = this.errorWebSocket
    },

    websocketOpen() {
      this.$emit('open')

      if (this.$refs.onetermTerminalRef) {
        this.resizeObserver = new ResizeObserver(_.debounce((entries) => {
          if (entries?.length) {
            this.handleResize()
          }
        }, 200))
        this.resizeObserver.observe(this.$refs.onetermTerminalRef)
      }

      this.interval = setInterval(() => {
        this.websocket.send('9')
      }, 10000)
    },

    closeWebSocket(e) {
      console.log('closeWebSocket', e)
      if (this.term) {
        this.term.writeln('\r\n')
        this.term.writeln('\x1b[31mThe connection is closed!\x1b[0m')
      }

      this.$emit('close')
      if (this.interval) {
        clearInterval(this.interval)
        this.interval = null
      }
    },
    errorWebSocket(e) {
      console.log('errorWebSocket', e)
      this.$emit('close')
    },
    getMessage(message) {
      if (this.term) {
        this.term.write(message.data)
      }
    },

    handleResize() {
      if (this.fitAddon) {
        this.fitAddon.fit()
      }
    },

    writeCommand(content) {
      this.websocket.send(`1${content}`)
    },

    openCommandDrawer() {
      this.$refs.commandDrawerRef.open()
    },

    openFileManagementDrawer() {
      this.$refs.fileManagementDrawerRef.open()
    }
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
