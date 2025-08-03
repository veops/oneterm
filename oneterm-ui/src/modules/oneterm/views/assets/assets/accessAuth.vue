<template>
  <a-form-model ref="form" :model="form" :rules="rules" :label-col="{ span: 5 }" :wrapper-col="{ span: 16 }">
    <a-form-model-item :label="$t('oneterm.assetList.time')">
      <DragWeektime v-model="form.ranges" :data="weekTimeData" @onClear="clearWeektime" />
    </a-form-model-item>
    <a-form-model-item :wrapper-col="{ span: 16 }" :label="$t('oneterm.timeTemplate.timeZone')" prop="timezone">
      <a-select
        v-model="form.timezone"
        showSearch
        :placeholder="$t('placeholder2')"
        :options="timezoneSelectOptions"
      />
    </a-form-model-item>
    <a-form-model-item :label="$t(`oneterm.assetList.commandIntercept`)" prop="cmd_ids">
      <a-select
        v-model="form.cmd_ids"
        mode="multiple"
        :options="commandSelectOptions"
        :placeholder="$t('oneterm.auth.commandTip1')"
      />
      <a-select
        mode="multiple"
        v-model="form.template_ids"
        :options="commandTemplateSelectOptions"
        :placeholder="$t('oneterm.auth.commandTip2')"
      />
    </a-form-model-item>
  </a-form-model>
</template>

<script>
import _ from 'lodash'
import momentTimezone from 'moment-timezone'
import { getCommandList } from '@/modules/oneterm/api/command'
import { getCommandTemplateList } from '@/modules/oneterm/api/commandTemplate.js'
import { mergeTimeRange } from '@/modules/oneterm/views/access/time/mergeTimeRange.js'
import { splitTimeRange } from '@/modules/oneterm/views/access/time/splitTimeRange.js'

import DragWeektime from '@/modules/oneterm/components/dragWeektime'
import weekTimeData from '@/modules/oneterm/components/dragWeektime/weektimeData'

export default {
  name: 'AccessAuth',
  components: { DragWeektime },
  data() {
    return {
      weekTimeData: _.cloneDeep(weekTimeData),
      form: {
        ranges: [],
        timezone: momentTimezone.tz.guess(),
        cmd_ids: undefined,
        template_ids: undefined
      },
      rules: {},
      commandSelectOptions: [],
      commandTemplateSelectOptions: [],
    }
  },
  computed: {
    timezoneSelectOptions() {
      const names = momentTimezone.tz.names()
      return names.map((value) => {
        return {
          value,
          label: value
        }
      })
    }
  },
  mounted() {
    this.getCommandList()
    this.getCommandTemplateList()
  },
  beforeDestroy() {
    this.clearWeektime()
  },
  methods: {
    getCommandList() {
      getCommandList({
        page_index: 1,
        page_size: 9999
      }).then((res) => {
        const list = res?.data?.list || []
        this.commandSelectOptions = list.map((item) => ({
          value: item.id,
          label: item.name
        }))
      })
    },
    getCommandTemplateList() {
      getCommandTemplateList({
        page_index: 1,
        page_size: 9999
      }).then((res) => {
        const list = res?.data?.list || []
        this.commandTemplateSelectOptions = list.map((item) => ({
          value: item.id,
          label: item.name
        }))
      })
    },
    clearWeektime() {
      this.weekTimeData.forEach((item) => {
        item.child.forEach((t) => {
          this.$set(t, 'check', false)
        })
      })
      this.form.ranges = []
    },
    getValues() {
      const { ranges, timezone, cmd_ids, template_ids } = this.form
      let time_ranges = []
      if (ranges.length) {
        time_ranges = mergeTimeRange(ranges)
      }

      return {
        cmd_ids,
        template_ids,
        time_ranges,
        timezone
      }
    },
    async setValues({
      access_time_control,
      asset_command_control
    }) {
      const { time_ranges = [], timezone = momentTimezone.tz.guess() } = access_time_control || {}
      const { cmd_ids, template_ids } = asset_command_control || {}

      let ranges = []
      if (time_ranges?.length) {
        const timeRanges = splitTimeRange(time_ranges)
        ranges = timeRanges.map((item) => {
          const childData = this.weekTimeData?.[item?.day - 1]?.child
          if (childData?.length) {
            childData.forEach((t) => {
              this.$set(t, 'check', Boolean(item?.value?.length) && item.value.includes(t.value))
            })
          }

          return {
            id: item.day,
            day: item.day,
            value: item.value,
          }
        })
      }

      this.form = {
        cmd_ids: cmd_ids ?? undefined,
        template_ids: template_ids ?? undefined,
        ranges,
        timezone
      }
      if (!ranges.length) {
        this.clearWeektime()
      }
    }
  },
}
</script>

<style lang="less">
.access-auth-user {
  .ant-form-item-control {
    line-height: 32px;
  }
  .vue-treeselect__multi-value {
    line-height: 18px;
  }
}
</style>
