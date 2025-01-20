<template>
  <div
    :class="[isFullScreen ? 'oneterm-terminal-full' : 'oneterm-terminal-panel']"
    ref="onetermTerminalRef"
  ></div>
</template>

<script>
import 'xterm/css/xterm.css'
import { Terminal } from 'xterm'
import { FitAddon } from 'xterm-addon-fit'

export const initMessageStorageKey = 'init_oneterm_terminal_msg'

export default {
  name: 'Terminal',
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
      term: null,
      websocket: null,
      interval: null,
      prefix: '',
      inputText: '',
      initMessage: [],
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
  },
  beforeDestroy() {
    window.removeEventListener('resize', this.resize)
    this.websocket.close()
    this.webSocket = null
    this.term = null
    if (this.interval) {
      clearInterval(this.interval)
      this.interval = null
    }
  },
  methods: {
    async initTerm({ disableStdin = false }) {
      this.fitAddon = new FitAddon()
      this.term = new Terminal({
        fontSize: 14,
        fontFamily: 'Consolas, courier-new, courier, monospace',
        lineHeight: 1.1,
        cursorBlink: !disableStdin,
        allowProposedApi: true,
        disableStdin: disableStdin,
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
            console.log(data)
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
      window.addEventListener('resize', this.resize)
      this.interval = setInterval(() => {
        this.websocket.send('9')
      }, 10000)
    },
    closeWebSocket(e) {
      console.log(e)
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
      console.log(e)
      this.$emit('close')
    },
    getMessage(message) {
      this.term.write(message.data)
    },
    resize() {
      if (this.fitAddon) {
        this.fitAddon.fit()
      }
    },
  },
}
</script>

<style lang="less" scoped>
.oneterm-terminal-full {
  position: fixed;
  top: 0;
  left: 0;
  width: 100vw;
  height: 100vh;
  padding: 5px;
  background-color: #000;
}

.oneterm-terminal-panel {
  width: 100%;
  height: 100%;
  background-color: #000 !important;
}
</style>

<style lang="less">
.oneterm-terminal-full {
  .terminal {
    margin-left: 5px;
  }
  .xterm-screen {
    min-height: 100vh;
  }
}

.oneterm-terminal-panel {
  .terminal {
    margin-left: 5px;
  }
  .xterm-screen {
    min-height: 100%;
  }
}
</style>
