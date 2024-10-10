<template>
  <div v-if="shareId" >
    <Terminal
      v-if="protocol === 'ssh'"
      :shareId="shareId"
    />
    <GuacamolePanel
      v-else-if="protocol === 'rdp' || protocol === 'vnc'"
      :shareId="shareId"
    />
  </div>
</template>

<script>
import Terminal from '../terminal/index.vue'
import GuacamolePanel from '../terminal/guacamoleClient.vue'

export default {
  name: 'Share',
  components: {
    Terminal,
    GuacamolePanel
  },
  data() {
    return {
      shareId: '',
      protocol: ''
    }
  },
  created() {
    if (this.$route.params.id) {
      this.shareId = this.$route.params.id

      this.protocol = this.$route?.params?.protocol || ''
    }
  }
}
</script>

<style lang="less" scoped>
</style>
