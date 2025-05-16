<template>
  <div v-if="shareId" >
    <Terminal
      v-if="['ssh', 'telnet', 'mysql', 'redis', 'postgresql', 'mongodb'].includes(protocol)"
      :shareId="shareId"
    />
    <GuacamolePanel
      v-else-if="protocol === 'rdp' || protocol === 'vnc'"
      :shareId="shareId"
    />
  </div>
</template>

<script>
import Terminal from '@/modules/oneterm/views/connect/terminal/index.vue'
import GuacamolePanel from '@/modules/oneterm/views/connect/guacamoleClient/index.vue'

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
