<template>
  <div
    :class="[
      'oneterm-terminal-container',
      isFullScreen || openFullScreen ? 'oneterm-terminal-full' : 'oneterm-terminal-panel'
    ]"
    :style="{
      backgroundColor: terminalBackground
    }"
  >
    <div
      class="oneterm-terminal-wrap"
      ref="onetermTerminalRef"
    ></div>

    <OperationMenu
      v-if="showOperationMenu"
      :openFullScreen="openFullScreen"
      @toggleFullScreen="toggleFullScreen"
      @writeCommand="writeCommand"
      @openSystemSetting="openSystemSetting"
    />
  </div>
</template>

<script>
import _ from 'lodash'
import 'xterm/css/xterm.css'
import { Terminal } from 'xterm'
import { FitAddon } from 'xterm-addon-fit'
import XtermTheme from 'xterm-theme'
import { defaultPreferenceSetting } from '@/modules/oneterm/views/systemSettings/terminalDisplay/constants.js'
import FullScreenMixin from '@/modules/oneterm/mixins/fullScreenMixin'

import OperationMenu from './operationMenu.vue'

export const initMessageStorageKey = 'init_oneterm_terminal_msg'

export default {
  name: 'Terminal',
  mixins: [FullScreenMixin],
  components: {
    OperationMenu
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
    showOperationMenu: {
      type: Boolean,
      default: false
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

      resizeObserver: null, // terminal container size observer
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
      window.addEventListener('beforeunload', this.beforeUnload)
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
      window.removeEventListener('beforeunload', this.beforeUnload)
    }
  },
  watch: {
    preferenceSetting: {
      deep: true,
      handler(preferenceSetting) {
        if (preferenceSetting) {
          this.updateTerm(preferenceSetting)
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

    updateTerm(preferenceSetting) {
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
      const { session_id, is_monitor } = this.$route.query
      let { asset_id, account_id, protocol: queryProtocol } = this.$route.query

      if (!this.isFullScreen) {
        asset_id = this.assetId
        account_id = this.accountId
        queryProtocol = this.protocol
      }

      const protocol = document.location.protocol.startsWith('https') ? 'wss' : 'ws'

      let socketLink = ''
      if (is_monitor) {
        socketLink = `${protocol}://${document.location.host}/api/oneterm/v1/connect/monitor/${session_id}?w=${this.term.cols}&h=${this.term.rows}`
      } else if (this.shareId) {
        socketLink = `${protocol}://${document.location.host}/api/oneterm/v1/share/connect/${this.shareId}?w=${this.term.cols}&h=${this.term.rows}`
      } else {
        socketLink = `${protocol}://${document.location.host}/api/oneterm/v1/connect/${asset_id}/${account_id}/${queryProtocol}?w=${this.term.cols}&h=${this.term.rows}`
      }

      if (!socketLink) {
        return
      }

      this.websocket = new WebSocket(
        socketLink,
        ['Sec-WebSocket-Protocol']
      )

      this.websocket.onopen = this.websocketOpen()
      this.websocket.onmessage = this.getMessage
      this.websocket.onclose = this.closeWebSocket
      this.websocket.onerror = this.errorWebSocket
    },

    websocketOpen() {
      this.$emit('open')

      if (this.$refs.onetermTerminalRef) {
        this.resizeObserver = new ResizeObserver(_.debounce((entries) => {
          if (entries?.length) {
            this.resize()
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

    resize() {
      if (this.fitAddon) {
        this.fitAddon.fit()
      }
    },

    writeCommand(content) {
      this.websocket.send(`1${content}`)
    },

    openSystemSetting(type) {
      if (this.openFullScreen) {
        this.toggleFullScreen()
      }

      this.$emit('openSystemSetting', type)
    },

    beforeUnload(event) {
      event.preventDefault()
      event.returnValue = this.$t('oneterm.workStation.pageUnloadMessage')
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
}

.oneterm-terminal-menu {
  position: absolute;
  bottom: 20px;
  right: 30px;
  z-index: 1001;

  &-btn {
    background-color: #FFFFFF;
    width: 32px;
    height: 32px;
    display: flex;
    align-items: center;
    justify-content: center;
    border-radius: 2px;
    cursor: pointer;
    box-shadow: 0 2px 8px rgba(0, 0, 0, 0.15);

    &:hover {
      color: @text-color_5;
    }
  }
}
</style>
