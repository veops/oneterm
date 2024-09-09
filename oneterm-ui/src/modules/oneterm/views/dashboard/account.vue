<template>
  <div class="dashbboard-layout">
    <h4>{{ $t('oneterm.dashboard.loginAccount') }}</h4>
    <TimeRadio v-model="type" @change="getOptions" />
    <div id="dashboard-account-chart"></div>
  </div>
</template>

<script>
import * as echarts from 'echarts'
import TimeRadio from './timeRadio.vue'
import { getAccountStat } from '../../api/stat'

export default {
  name: 'Account',
  components: {
    TimeRadio,
  },
  data() {
    return {
      chart: null,
      type: 'week',
    }
  },
  mounted() {
    window.addEventListener('resize', this.resize)
    this.getOptions()
  },
  beforeDestroy() {
    window.removeEventListener('resize', this.resize)
  },
  methods: {
    getOptions() {
      getAccountStat({ type: this.type }).then((res) => {
        const data = res?.data?.list
        const option = {
          xAxis: {
            type: 'category',
            data: data.map((item) => item.name),
          },
          yAxis: {
            type: 'value',
          },
          tooltip: {
            trigger: 'axis',
          },
          grid: {
            top: '3%',
            left: '3%',
            right: '4%',
            bottom: '3%',
            containLabel: true,
          },
          series: [
            {
              data: data.map((item) => item.count),
              type: 'bar',
              color: '#84A4F9',
              barMaxWidth: '16px'
            },
          ],
        }
        if (!this.chart) {
          this.chart = echarts.init(document.getElementById('dashboard-account-chart'))
        }
        this.chart.setOption(option)
      })
    },
    resize() {
      this.$nextTick((res) => {
        this.chart.resize()
      })
    },
  },
}
</script>

<style lang="less" scoped>
#dashboard-account-chart {
  height: 100%;
}
</style>
