<template>
  <div class="oneterm-replay" id="oneterm-replay"></div>
</template>

<script>
import { mapState } from 'vuex'
import * as AsciinemaPlayer from 'asciinema-player'
import 'asciinema-player/dist/bundle/asciinema-player.css'
export default {
  name: 'Replay',
  data() {
    return {
      player: null,
    }
  },
  computed: {
    ...mapState({
      windowHeight: (state) => state.windowHeight,
      windowWidth: (state) => state.windowWidth,
    }),
  },
  mounted() {
    console.log(this.$route)
    const { session_id } = this.$route.params
    this.player = AsciinemaPlayer.create(
      `/api/oneterm/v1/session/replay/${session_id}`,
      document.getElementById('oneterm-replay'),
      {
        markers: [
          [5.0, 'Installation'], // time in seconds + label
          [25.0, 'Configuration'],
          [66.6, 'Usage'],
          [176.5, 'Tips & Tricks'],
        ],
        cols: this.windowWidth,
        fit: 'height',
        terminalFontFamily: 'monaco, Consolas, "Lucida Console", monospace',
      }
    )
  },
}
</script>

<style lang="less" scoped>
.oneterm-replay {
  position: fixed;
  top: 0;
  left: 0;
  width: 100vw;
  height: 100vh;
}
</style>

<style lang="less">
.oneterm-replay {
  .asciinema-player .control-bar .progressbar {
    padding-right: 50px;
  }
}
</style>
