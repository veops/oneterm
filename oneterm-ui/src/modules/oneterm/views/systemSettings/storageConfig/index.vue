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

  /deep/ .ant-tabs-card-bar {
    border-bottom: 1px solid #e8eaed;

    .ant-tabs-tab {
      background-color: #fafafa;
      border: 1px solid #e8eaed;
      color: @text-color_2;

      &:hover {
        color: @primary-color;
      }
    }

    .ant-tabs-tab-active {
      background-color: #fff;
      color: @primary-color;
      font-weight: 600;
    }
  }
}
</style>
