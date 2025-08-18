<template>
  <div class="enabled-status">
    <div class="enabled-status-display">
      <span
        class="enabled-status-dot"
        :style="{
          backgroundColor: statusColor + '22',
          '--innerBackgroundColor': statusColor
        }"
      ></span>
      <span>
        {{ $t(statusText) }}
      </span>
    </div>
    <a-popconfirm
      :title="$t('oneterm.confirmEnable')"
      @confirm="$emit('change', !status)"
    >
      <a-switch
        :checked="status"
        :checked-children="$t('oneterm.enabled')"
        :un-checked-children="$t('oneterm.disabled')"
      />
    </a-popconfirm>
  </div>
</template>

<script>
export default {
  name: 'EnabledStatus',
  props: {
    status: {
      type: Boolean,
      default: true
    }
  },
  computed: {
    statusColor() {
      return this.status ? '#52c41a' : '#A5A9BC'
    },
    statusText() {
      return this.status ? 'oneterm.enabled' : 'oneterm.disabled'
    }
  }
}
</script>

<style lang="less" scoped>
.enabled-status {
  width: min-content;
  cursor: pointer;

  /deep/ .ant-switch {
    display: none;
  }

  &-display {
    display: flex;
    align-items: center;
  }

  &-dot {
    width: 14px;
    height: 14px;
    border-radius: 50%;
    position: relative;
    margin-right: 4px;

    &::before {
      content: '';
      position: absolute;
      top: 50%;
      left: 50%;
      width: 8px;
      height: 8px;
      border-radius: 50%;
      margin-top: -4px;
      margin-left: -4px;
      background-color: var(--innerBackgroundColor);
    }
  }

  &:hover {
    .enabled-status-display {
      display: none;
    }

    /deep/ .ant-switch {
      display: inline-block;
    }
  }
}
</style>
