<template>
  <div class="dashboard-assettype dashbboard-layout">
    <h4>{{ $t('oneterm.dashboard.assetType') }}</h4>
    <div id="dashboard-assettype-chart"></div>
  </div>
</template>

<script>
import * as echarts from 'echarts'
import { getAssetTypeStat } from '../../api/stat'

export default {
  name: 'AssetType',
  data() {
    return {
      chart: null,
    }
  },
  mounted() {
    window.addEventListener('resize', this.resize)
    getAssetTypeStat().then((res) => {
      const data = res?.data?.list
      const option = {
        color: ['#4C81E2', '#61D9AC', '#84A4F9', '#7BD5FF'],
        tooltip: {
          trigger: 'item',
        },
        legend: {
          type: 'scroll',
          bottom: '0',
          left: 'center',
        },
        series: [
          {
            type: 'pie',
            radius: ['45%', '70%'],
            left: 'center',
            top: '-10%',
            width: '100%',
            height: '100%',
            avoidLabelOverlap: false,
            label: {
              show: false,
              position: 'center',
            },
            emphasis: {
              label: {
                show: false,
                fontSize: 40,
                fontWeight: 'bold',
              },
            },
            labelLine: {
              show: false,
            },
            data: data.map((item) => ({ value: item.count, name: item.name })),
          },
        ],
        graphic: {
          type: 'text',
          left: 'center',
          top: '35%',
          style: {
            text: data.reduce((acc, cur) => acc + cur.count, 0),
            textAlign: 'center',
            fill: '#000',
            fontSize: 38,
            fontWeight: '700',
          },
        },
      }
      this.chart = echarts.init(document.getElementById('dashboard-assettype-chart'))
      this.chart.setOption(option)
    })
  },
  beforeDestroy() {
    window.removeEventListener('resize', this.resize)
  },
  methods: {
    resize() {
      this.$nextTick((res) => {
        this.chart.resize()
      })
    },
  },
}
</script>

<style lang="less" scoped>
#dashboard-assettype-chart {
  height: 100%;
}
</style>
