const FullScreenMixin = {
  data() {
    return {
      openFullScreen: false
    }
  },
  mounted() {
    document.addEventListener('fullscreenchange', this.handleFullScreenChange)
    document.addEventListener('webkitfullscreenchange', this.handleFullScreenChange)
    document.addEventListener('mozfullscreenchange', this.handleFullScreenChange)
    document.addEventListener('MSFullscreenChange', this.handleFullScreenChange)
  },
  beforeDestroy() {
    document.removeEventListener('fullscreenchange', this.handleFullScreenChange)
    document.removeEventListener('webkitfullscreenchange', this.handleFullScreenChange)
    document.removeEventListener('mozfullscreenchange', this.handleFullScreenChange)
    document.removeEventListener('MSFullscreenChange', this.handleFullScreenChange)
  },

  methods: {
    toggleFullScreen() {
      if (this.openFullScreen) {
        this.exitFullScreen()
      } else {
        this.enterFullScreen()
      }
    },

    enterFullScreen() {
      const element = document.documentElement
      const requestMethod = element.requestFullScreen ||
          element.webkitRequestFullScreen ||
          element.mozRequestFullScreen ||
          element.msRequestFullScreen

      if (requestMethod) {
          requestMethod.call(element)
      } else if (typeof window.ActiveXObject !== 'undefined') {
          const wScript = new window.ActiveXObject('WScript.Shell')
          if (wScript !== null) {
              wScript.SendKeys('{F11}')
          }
      }

      this.openFullScreen = true
    },

    exitFullScreen() {
      const exitMethod = document.exitFullscreen ||
        document.mozCancelFullScreen ||
        document.webkitExitFullscreen ||
        document.webkitExitFullscreen

      if (exitMethod) {
        exitMethod.call(document)
      } else if (typeof window.ActiveXObject !== 'undefined') {
        const wScript = new window.ActiveXObject('WScript.Shell')
        if (wScript !== null) {
            wScript.SendKeys('{F11}')
        }
      }

      this.openFullScreen = false
    },

    handleFullScreenChange() {
      const isPageFullscreen = !!(document.fullscreenElement || document.mozFullScreenElement || document.webkitFullscreenElement || document.msFullscreenElement)
      this.openFullScreen = isPageFullscreen ?? this.openFullScreen
    }
  }
}

export default FullScreenMixin
