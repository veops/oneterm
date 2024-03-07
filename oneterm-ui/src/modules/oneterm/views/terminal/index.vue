<template>
  <div class="oneterm-terminal" id="oneterm-terminal"></div>
</template>

<script>
import 'xterm/css/xterm.css'
import { Terminal } from 'xterm'
import { FitAddon } from 'xterm-addon-fit'
export default {
  name: 'Terminal',
  data() {
    return {
      term: null,
      websocket: null,
      interval: null,
      prefix: '',
      inputText: '',
    }
  },
  async mounted() {
    const { is_monitor } = this.$route.query
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
      const fitAddon = new FitAddon()
      this.term = new Terminal({
        fontSize: 14,
        cursorBlink: !disableStdin,
        allowProposedApi: true,
        disableStdin: disableStdin,
      })

      this.term.loadAddon(fitAddon)
      this.term.open(document.getElementById('oneterm-terminal'))
      this.term.writeln('\x1b[1;1;32mwelcome to oneterm!\x1b[0m')
      if (!disableStdin) {
        this.term.onData((data) => {
          if (this.websocket) {
            console.log(data)
            this.websocket.send(`1${data}`)
          }
        })
      }

      fitAddon.fit()
      this.term.focus()
    },
    initWebsocket() {
      const { session_id, is_monitor } = this.$route.query
      const protocol = document.location.protocol.startsWith('https') ? 'wss' : 'ws'
      this.websocket = new WebSocket(
        is_monitor
          ? `${protocol}://${document.location.host}/api/oneterm/v1/connect/monitor/${session_id}?w=${this.term.cols}&h=${this.term.rows}`
          : `${protocol}://${document.location.host}/api/oneterm/v1/connect/${session_id}?w=${this.term.cols}&h=${this.term.rows}`,
        ['Sec-WebSocket-Protocol']
      )
      this.websocket.onopen = this.websocketOpen()
      this.websocket.onmessage = this.getMessage
      this.websocket.onclose = this.closeWebSocket
      this.websocket.onerror = this.errorWebSocket
    },
    websocketOpen() {
      window.addEventListener('resize', this.resize)
      this.interval = setInterval(() => {
        this.websocket.send('9')
      }, 10000)
    },
    closeWebSocket(e) {
      console.log(e)
      if (this.interval) {
        clearInterval(this.interval)
        this.interval = null
      }
    },
    errorWebSocket(e) {
      console.log(e)
    },
    getMessage(message) {
      this.term.write(message.data)
    },
    resize() {
      this.websocket.send(`w${this.term.cols},${this.term.rows}`)
    },
  },
}
</script>

<style lang="less" scoped>
.oneterm-terminal {
  position: fixed;
  top: 0;
  left: 0;
  width: 100vw;
  height: 100vh;
}
</style>

<style>
.xterm-screen {
  min-height: calc(100vh);
}
</style>
