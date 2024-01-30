<template>
  <a-space>
    <a-popover
      v-if="isShowRangeTime"
      @visibleChange="visibleChange"
      trigger="click"
      placement="bottomLeft"
      :overlayStyle="{ maxWidth: '328px' }"
      ref="chartTimePopover"
      overlayClassName="chart-time"
    >
      <template slot="content">
        <div
          v-for="item in list"
          :key="item.label"
          :class="{
            'chart-time-popover-item': true,
            'chart-time-popover-item-selected':
              range_date_remeber &&
              ((range_date_remeber.type !== 'last' && range_date_remeber.type === item.type) ||
              ((range_date_remeber.type === 'last' || !range_date_remeber.type) &&
              range_date_remeber.valueFormat === item.valueFormat &&
              range_date_remeber.number === item.number)),
          }"
          @click="clickRange(item.number, item.valueFormat, item.type)"
        >
          {{ renderI18n(item.number, item.valueFormat, item.type) }}
        </div>
        <a-divider orientation="left">{{ $t('chartTime.custom') }}</a-divider>
        <div class="chart-time-custom-time">
          <a-range-picker
            :allowClear="false"
            v-model="range_date"
            :show-time="showTime"
            :format="momnetFormat"
            @ok="timeChange"
            @change="
              (dates, dateStrings) => {
                if (!showTime) {
                  timeChange(dates, dateStrings)
                }
              }
            "
            size="small"
            @calendarChange="calendarChange"
            :disabled-date="disabledDate"
            :style="{ width: '315px' }"
          >
          </a-range-picker>
        </div>
      </template>
      <div class="chart-time-display-time">
        <slot name="displayTimeIcon"></slot>
        {{ displayTime }}
        <a-icon class="chart-time-display-icon" :type="displayTimeIcon" />
      </div>
    </a-popover>
    <a-select
      v-if="isShowInternalTime"
      v-model="intervalTime"
      :disabled="isFixedTime"
      @change="intervalTimeChange"
      dropdownClassName="custom-dashboard-interval-dropdown"
      :style="{ width: '70px' }"
    >
      <a-select-option v-for="time in intervalTimeList" :key="time.value" :label="time.value">{{
        time.name
      }}</a-select-option>
    </a-select>
  </a-space>
</template>

<script>
import moment from 'moment'
import { intervalTimeList } from './constants'
export default {
  name: 'ChartTime',
  props: {
    localStorageKey: {
      type: String,
      default: '',
    },
    list: {
      type: Array,
      default: () => [],
    },
    isShowInternalTime: {
      type: Boolean,
      default: true,
    },
    isShowRangeTime: {
      type: Boolean,
      default: true,
    },
    default_range_date_remeber: {
      type: Object,
      default: () => {
        return {
          number: 30,
          valueFormat: 'minutes',
        }
      },
    },
    momnetFormat: {
      type: String,
      default: 'YYYY-MM-DD HH:mm:ss',
    },
    showTime: {
      type: Boolean,
      default: true,
    },
    limitDate: {
      type: Object,
      default: () => null,
    },
  },
  data() {
    return {
      intervalTimeList,
      range_date: [moment().subtract(30, 'minutes'), moment()],
      from_ts: moment()
        .subtract(30, 'minutes')
        .unix(),
      to_ts: moment().unix(),
      range_date_remeber: null,
      intervalTime: 30 * 1000,
      isFixedTime: false,
      displayTimeIcon: 'down',
      currentSelectedDate: null,
      timeType: '',
    }
  },
  computed: {
    displayTime() {
      const { range_date_remeber, isFixedTime } = this
      if (!isFixedTime) {
        const { number, valueFormat, type } = range_date_remeber
        return this.renderI18n(number, valueFormat, type)
      }
      return `${moment(this.from_ts * 1000).format(this.momnetFormat)} ~ ${moment(this.to_ts * 1000).format(
        this.momnetFormat
      )}`
    },
  },
  created() {
    this.init()
  },
  methods: {
    init() {
      const storageData = JSON.parse(localStorage.getItem(this.localStorageKey))
      if (storageData && storageData.intervalTime) {
        this.intervalTime = storageData.intervalTime
      }
      if (storageData && storageData.isFixedTime) {
        this.isFixedTime = true
        this.range_date = [moment(storageData.range_date_detail[0]), moment(storageData.range_date_detail[1])]
        this.from_ts = moment(storageData.range_date_detail[0]).unix()
        this.to_ts = moment(storageData.range_date_detail[1]).unix()
        this.emitTimeChange(true)
      } else {
        this.isFixedTime = false
        if (storageData && storageData.range_date) {
          const { number, valueFormat, type } = storageData.range_date
          this.range_date_remeber = storageData.range_date
          this.clickRange(number, valueFormat, type, true)
          this.from_ts = moment(this.range_date[0]).unix()
          this.to_ts = moment(this.range_date[1]).unix()
        } else {
          this.range_date_remeber = this.default_range_date_remeber

          const fromMoment = moment().subtract(
            this.default_range_date_remeber.number,
            this.default_range_date_remeber.valueFormat
          )
          const toMoment = moment()
          this.range_date = [fromMoment, toMoment]
          this.from_ts = fromMoment.unix()
          this.to_ts = toMoment.unix()
          this.emitTimeChange(true)
        }
      }
    },
    visibleChange(visible) {
      this.displayTimeIcon = visible ? 'up' : 'down'
    },
    timeChange(dates, dateStrings) {
      this.from_ts = dates[0].unix()
      this.to_ts = dates[1].unix()
      this.isFixedTime = true
      this.intervalTime = 'off'
      this.range_date_remeber = null
      this.currentSelectedDate = null
      this.emitTimeChange(false)
      if (this.$refs.chartTimePopover) {
        this.$refs.chartTimePopover.$refs.tooltip.onVisibleChange(false)
      }
    },
    clickRange(number, valueFormat, type, isInit = false) {
      this.isFixedTime = false
      this.timeType = type
      if (type === 'Today') {
        this.range_date = [moment().startOf('day'), moment()]
      } else if (type === 'This Month') {
        this.range_date = [moment().startOf('month'), moment()]
      } else {
        this.range_date = [moment().subtract(number, valueFormat), moment()]
      }
      if (!isInit) {
        this.range_date_remeber = { number, valueFormat, type }
        this.from_ts = this.range_date[0].unix()
        this.to_ts = this.range_date[1].unix()
      }
      if (this.$refs.chartTimePopover) {
        this.$refs.chartTimePopover.$refs.tooltip.onVisibleChange(false)
      }

      this.emitTimeChange(isInit)
    },
    intervalTimeChange() {
      this.emitTimeChange(false)
    },
    emitTimeChange(isInit) {
      const { from_ts, to_ts, isFixedTime, intervalTime, range_date_remeber, range_date, timeType } = this
      this.$emit(
        'chartTimeChange',
        {
          from_ts,
          to_ts,
          isFixedTime,
          intervalTime,
          range_date_remeber,
          range_date,
          timeType,
        },
        isInit
      )
    },
    resetTime() {
      this.range_date_remeber = this.default_range_date_remeber
      this.isFixedTime = false
      const fromMoment = moment().subtract(
        this.default_range_date_remeber.number,
        this.default_range_date_remeber.valueFormat
      )
      const toMoment = moment()
      this.range_date = [fromMoment, toMoment]
      this.from_ts = fromMoment.unix()
      this.to_ts = toMoment.unix()
      this.emitTimeChange(false)
    },
    calendarChange(dates, dateStrings) {
      if (this.limitDate) {
        if (dates && dates.length === 1) {
          this.currentSelectedDate = dates[0]
        }
      }
    },
    disabledDate(current) {
      if (this.currentSelectedDate) {
        return (
          current < moment(this.currentSelectedDate).subtract(this.limitDate.number, this.limitDate.dateType) ||
          current > moment(this.currentSelectedDate).add(this.limitDate.number, this.limitDate.dateType)
        )
      }
      return null
    },
    renderI18n(number, valueFormat, type) {
      if (type === 'Today') {
        return this.$t('chartTime.today')
      } else if (type === 'This Month') {
        return this.$t('chartTime.thisMonth')
      } else if (type === 'all') {
        return this.$t('chartTime.all')
      } else {
        return `${this.$t('chartTime.last')} ${number} ${this.$t(`chartTime.${valueFormat}`)}`
      }
    },
  },
}
</script>

<style lang="less" scoped>
@import '~@/style/static.less';
.chart-time-display-time {
  .ops_display_wrapper(#fff);
}

.blue {
  .chart-time-display-time {
    background-color: #custom_colors[color_2];
  }
}
.chart-time-display-icon {
  color: #custom_colors[color_1];
  font-size: 12px;
}
.chart-time {
  .chart-time-popover-item {
    .ops_popover_item();
  }
  .chart-time-popover-item-selected {
    .ops_popover_item_selected();
  }
  .chart-time-custom-time {
    padding: 6px;
  }
}
</style>
<style lang="less">
.custom-dashboard-interval-dropdown {
  .ant-select-dropdown-menu {
    max-height: 360px;
  }
}
.chart-time {
  .ant-popover-inner-content {
    padding: 0;
  }
  .ant-divider {
    margin: 0;
    .ant-divider-inner-text {
      font-size: 12px;
      color: #b9b9b9;
    }
  }
}
</style>
