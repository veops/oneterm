<template>
  <div class="oneterm-guacamole-replay-wrapper">
    <div class="oneterm-guacamole-replay" id="display"></div>
    <span
      v-if="waiting"
      @click="
        () => {
          waiting = false
          handlePlayPause()
        }
      "
    >{{ $t('oneterm.guacamole.play') }}</span
    >
    <a-row
      v-else
      class="oneterm-guacamole-replay-progress"
      type="flex"
      justify="space-around"
      align="middle"
      :gutter="[5, 5]"
    >
      <a-col><a-icon @click="handlePlayPause" :type="playBtnIcon"/></a-col>
      <a-col flex="1">
        <a-slider
          :tip-formatter="formatter"
          :max="max"
          :value="percent"
          @change="handleProgressChange"
        /></a-col>
      <a-col>
        <a-select
          size="small"
          :style="{ width: '110px' }"
          :value="speed"
          @change="
            (value) => {
              speed = value
              if (value === 1) {
                stopSpeedUp()
              } else {
                startSpeedUp()
              }
            }
          "
        >
          <a-select-option :value="1">
            {{ $t('oneterm.guacamole.speed1') }}
          </a-select-option>
          <a-select-option :value="1.25">
            {{ $t('oneterm.guacamole.speed2') }}
          </a-select-option>
          <a-select-option :value="1.5">
            {{ $t('oneterm.guacamole.speed3') }}
          </a-select-option>
          <a-select-option :value="2">
            {{ $t('oneterm.guacamole.speed4') }}
          </a-select-option>
        </a-select>
      </a-col>
      <a-col>{{ position }}/ {{ duration }}</a-col>
    </a-row>
  </div>
</template>

<script>
import Guacamole from 'guacamole-common-js'
import { times } from '../../utils'

export default {
  name: 'GuacamoleReplay',
  data() {
    return {
      recording: null,
      percent: 0,
      speed: 1,
      max: 0,
      position: '00:00',
      duration: '00:00',
      waiting: true,
      timer: null,
      playBtnIcon: 'play-circle',
    }
  },
  mounted() {
    window.addEventListener('resize', this.onWindowResize)
    this.init()
  },
  beforeDestroy() {
    window.removeEventListener('resize', this.onWindowResize)
    if (this.recording) {
      this.recording.disconnect()
    }
  },
  methods: {
    init() {
      const { session_id } = this.$route.params
      const RECORDING_URL = `/api/oneterm/v1/session/replay/${session_id}`
      const tunnel = new Guacamole.StaticHTTPTunnel(RECORDING_URL)
      tunnel.onstatechange = this.onTunnelStateChange
      const recording = new Guacamole.SessionRecording(tunnel)
      const recordingDisplay = recording.getDisplay()
      const display = document.getElementById('display')
      display.appendChild(recordingDisplay.getElement())
      recording.connect()

      // If playing, the play/pause button should read "Pause"
      recording.onplay = () => {
        this.playBtnIcon = 'pause-circle'
      }

      // If paused, the play/pause button should read "Play"
      recording.onpause = () => {
        this.playBtnIcon = 'play-circle'
      }

      recordingDisplay.onresize = () => {
        this.onWindowResize()
      }

      recording.onseek = (millis) => {
        this.percent = millis
        this.position = times.formatTime(millis)
      }

      recording.onprogress = (millis) => {
        this.max = millis
        this.duration = times.formatTime(millis)
      }

      this.recording = recording
    },
    onTunnelStateChange(state) {
      switch (state) {
        case Guacamole.Tunnel.State.OPEN:
          this.handlePlayPause()
          break
        case Guacamole.Tunnel.State.CLOSED:
          break
        default:
          break
      }
    },
    handlePlayPause() {
      if (this.percent === this.max) {
        // 重播
        this.percent = 0
        this.recording.seek(0, () => {
          this.recording.play()
          this.startSpeedUp()
        })
      }
      if (!this.recording.isPlaying()) {
        this.recording.play()
        this.startSpeedUp()
      } else {
        this.recording.pause()
        this.stopSpeedUp()
        this.$message.info('暂停')
      }
    },
    startSpeedUp() {
      this.stopSpeedUp()
      if (this.speed === 1) {
        return
      }
      if (!this.recording.isPlaying()) {
        return
      }
      const add_time = 100
      const delay = 1000 / (1000 / add_time) / (this.speed - 1)

      const max = this.recording.getDuration()
      const current = this.recording.getPosition()
      if (current >= max) {
        return
      }
      this.recording.seek(current + add_time, () => {
        this.timer = setTimeout(this.startSpeedUp, delay)
      })
    },
    stopSpeedUp() {
      if (this.timer) {
        clearTimeout(this.timer)
      }
    },
    handleProgressChange(value) {
      this.recording.seek(value, () => {
        console.log('complete')
      })
    },
    formatter(value) {
      return `${times.formatTime(value)}`
    },
    onWindowResize() {
      const { recording } = this
      const width = recording.getDisplay().getWidth()
      const height = recording.getDisplay().getHeight()

      const winWidth = window.innerWidth
      const winHeight = window.innerHeight - 40

      const scaleW = winWidth / width
      const scaleH = winHeight / height

      const scale = Math.min(scaleW, scaleH)
      if (!scale) {
        return
      }
      recording.getDisplay().scale(scale)
    },
  },
}
</script>

<style lang="less" scoped>
.oneterm-guacamole-replay-wrapper {
  background: black;
  > span {
    position: absolute;
    top: 50%;
    left: 50%;
    transform: translate(-50%, -50%);
    color: white;
    cursor: pointer;
    font-size: 20px;
    font-weight: 700;
  }
  .oneterm-guacamole-replay-progress {
    width: 100%;
    background: black;
    color: white;
    font-weight: bold;
    position: absolute;
    bottom: 0;
  }
}
.oneterm-guacamole-replay-wrapper,
.oneterm-guacamole-replay {
  position: fixed;
  top: 0;
  left: 0;
  width: 100vw;
  height: 100vh;
}
</style>

<style lang="less">
.oneterm-guacamole-replay > div {
  margin: 0 auto;
}
</style>
