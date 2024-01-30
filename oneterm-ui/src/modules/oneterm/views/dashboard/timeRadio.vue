<template>
  <div class="dashboard-timeradio">
    <a-space>
      <span
        :class="{ 'dashboard-timeradio-item': true, 'dashboard-timeradio-selected': radioValue === item.value }"
        v-for="item in timeList"
        :key="item.value"
        @click="
          () => {
            $emit('change', item.value)
          }
        "
      >{{ $t(`oneterm.dashboard.last${item.label}`) }}</span
      >
    </a-space>
  </div>
</template>

<script>
export default {
  name: 'TimeRadio',
  model: {
    prop: 'value',
    event: 'change',
  },
  props: {
    value: {
      type: String,
      default: 'week',
    },
  },
  data() {
    return {
      timeList: [
        {
          label: 'Day',
          value: 'day',
        },
        {
          label: 'Week',
          value: 'week',
        },
        {
          label: 'Month',
          value: 'month',
        },
      ],
    }
  },
  computed: {
    radioValue: {
      get() {
        return this.value
      },
      set(val) {
        this.$emit('change', val)
        return val
      },
    },
  },
}
</script>

<style lang="less" scoped>
@import '~@/style/static.less';
.dashboard-timeradio-item {
  cursor: pointer;
  padding: 5px 10px;
  border-radius: 2px;
}
.dashboard-timeradio-selected {
  color: @primary-color;
  background-color: #2f54eb1a;
}
</style>
