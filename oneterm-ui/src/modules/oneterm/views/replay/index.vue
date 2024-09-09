<template>
  <div class="oneterm-replay" id="oneterm-replay"></div>
</template>

<script>
import { mapState } from 'vuex'
import * as AsciinemaPlayer from 'asciinema-player'
import 'asciinema-player/dist/bundle/asciinema-player.css'
import { getSessionReplayData } from '@/modules/oneterm/api/replay'

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
  async mounted() {
    console.log(this.$route)
    const { session_id } = this.$route.params
    const res = await getSessionReplayData(session_id)
    const regexp = /\{(?:[^{}]*|\{.*?\})*\}/
    const resConfig = res.match(regexp)

    const config = {
      markers: [
        [5.0, 'Installation'], // time in seconds + label
        [25.0, 'Configuration'],
        [66.6, 'Usage'],
        [176.5, 'Tips & Tricks'],
      ],
      fit: 'both',
      terminalFontSize: 'medium',
      terminalFontFamily: 'monaco, Consolas, "Lucida Console", monospace',
    }

    if (resConfig?.[0]) {
      const data = JSON.parse(resConfig?.[0])
      config.cols = data.width
      config.rows = data.height
    }

    this.player = AsciinemaPlayer.create(
      { data: res },
      document.getElementById('oneterm-replay'),
      config
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
  background-color: rgb(27,27,27);
}
</style>

<style lang="less">
.oneterm-replay {
  .asciinema-player .control-bar .progressbar {
    padding-right: 50px;
  }
}
</style>
