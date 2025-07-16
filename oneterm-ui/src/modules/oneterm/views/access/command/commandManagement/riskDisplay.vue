<template>
  <span class="risk">
    <span
      class="risk-status"
      :style="{
        backgroundColor: color + '22',
        '--innerBackgroundColor': color
      }"
    ></span>
    <span class="risk-text">
      {{ $t(text) }}
    </span>
  </span>
</template>

<script>
import { COMMAND_RISK_NAME } from '../constants.js'

export default {
  name: 'RiskDisplay',
  props: {
    type: {
      type: Number,
      default: 0
    }
  },
  data() {
    return {
      COMMAND_RISK_COLOR: {
        0: '#52c41a',
        1: '#faad14',
        2: '#ff4d4f',
        3: '#722323'
      }
    }
  },
  computed: {
    text() {
      return COMMAND_RISK_NAME?.[this.type]
    },
    color() {
      return this.COMMAND_RISK_COLOR?.[this.type]
    }
  }
}
</script>

<style lang="less" scoped>
.risk {
  display: inline-flex;
  align-items: center;

  &-status {
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
}
</style>
