<template>
  <div class="dashboard-assetactive dashbboard-layout">
    <h4>{{ $t('oneterm.dashboard.assetActive') }}</h4>
    <TimeRadio v-model="type" @change="getOptions" />
    <div id="dashboard-assetactive-chart"></div>
  </div>
</template>

<script>
import * as echarts from 'echarts'
import { mapState } from 'vuex'
import TimeRadio from './timeRadio.vue'
import { getAssetStat } from '../../api/stat'

export default {
  name: 'AssetActive',
  components: {
    TimeRadio,
  },
  data() {
    return {
      chart: null,
      type: 'week',
    }
  },
  computed: {
    ...mapState(['locale']),
  },
  watch: {
    locale() {
      this.getOptions()
    },
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
      getAssetStat({ type: this.type }).then((res) => {
        const data = res?.data?.list
        const option = {
          tooltip: {
            trigger: 'axis',
            axisPointer: {
              type: 'cross',
              label: {
                backgroundColor: '#6a7985',
              },
            },
          },
          legend: {
            data: [
              this.$t('oneterm.connect'),
              this.$t('oneterm.session'),
              this.$t('oneterm.connectedAssets'),
              this.$t('oneterm.connectedUsers'),
            ],
            bottom: 0,
            itemGap: 60
          },
          grid: {
            top: '3%',
            left: '3%',
            right: '4%',
            bottom: '30px',
            containLabel: true,
          },
          xAxis: [
            {
              type: 'category',
              boundaryGap: false,
              data: data.map((item) => item.time),
            },
          ],
          yAxis: [
            {
              type: 'value',
            },
          ],
          series: [
            {
              name: this.$t('oneterm.connect'),
              type: 'line',
              symbol: 'circle',
              symbolSize: 5,
              areaStyle: {
                color: 'rgba(56, 125, 255, 0.05)',
              },
              emphasis: {
                focus: 'series',
              },
              smooth: true,
              color: 'rgba(56, 125, 255, 1)',
              lineStyle: {
                width: 1.5
              },
              data: data.map((item) => item.connect),
            },
            {
              name: this.$t('oneterm.session'),
              type: 'line',
              symbol: 'circle',
              symbolSize: 5,
              areaStyle: {
                color: 'rgba(35, 184, 153, 0.05)',
              },
              emphasis: {
                focus: 'series',
              },
              smooth: true,
              color: 'rgba(35, 184, 153, 1)',
              lineStyle: {
                width: 1.5
              },
              data: data.map((item) => item.session),
            },
            {
              name: this.$t('oneterm.connectedAssets'),
              type: 'line',
              symbol: 'circle',
              symbolSize: 5,
              areaStyle: {
                color: 'rgba(254, 124, 75, 0.05)',
              },
              emphasis: {
                focus: 'series',
              },
              smooth: true,
              color: 'rgba(254, 124, 75, 1)',
              lineStyle: {
                width: 1.5
              },
              data: data.map((item) => item.asset),
            },
            {
              name: this.$t('oneterm.connectedUsers'),
              type: 'line',
              symbol: 'circle',
              symbolSize: 5,
              areaStyle: {
                color: 'rgba(78, 194, 239, 0.05)',
              },
              emphasis: {
                focus: 'series',
              },
              smooth: true,
              color: 'rgba(78, 194, 239, 1)',
              lineStyle: {
                width: 1.5
              },
              data: data.map((item) => item.user),
            },
          ],
        }
        if (!this.chart) {
          this.chart = echarts.init(document.getElementById('dashboard-assetactive-chart'))
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
#dashboard-assetactive-chart {
  height: 100%;
}
</style>
