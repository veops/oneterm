<template>
  <a-tabs
    type="card"
    class="ops-tab terminal-settings-tab"
  >
    <a-tab-pane
      key="sessionRecording"
      :tab="$t('oneterm.storageConfig.sessionRecording')"
      :forceRender="true"
    >
      <SessionRecording />
    </a-tab-pane>
  </a-tabs>
</template>

<script>
import SessionRecording from './sessionRecording/index.vue'

export default {
  name: 'StorageConfig',
  components: {
    SessionRecording
  }
}
</script>

<style lang="less" scoped>
.terminal-settings-tab {
  height: 100%;
}
</style>
