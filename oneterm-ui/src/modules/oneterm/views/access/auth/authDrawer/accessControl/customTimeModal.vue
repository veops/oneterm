<template>
  <a-modal
    :title="$t('oneterm.auth.editCustomTime')"
    :visible="visible"
    :width="1000"
    :bodyStyle="{
      maxHeight: '60vh',
      overflowY: 'auto'
    }"
    @cancel="handleCancel"
    @ok="handleOk"
  >
    <a-form-model
      ref="customTimeFormRef"
      :model="form"
      :rules="rules"
      :label-col="{ span: 5 }"
    >
      <a-form-model-item :wrapper-col="{ span: 16 }" :label="$t('oneterm.timeTemplate.timeZone')" prop="timezone">
        <a-select
          v-model="form.timezone"
          showSearch
          :placeholder="$t('placeholder2')"
          :options="timezoneSelectOptions"
        />
      </a-form-model-item>
      <a-form-model-item :wrapper-col="{ span: 16 }" :label="$t('oneterm.timeTemplate.timeRange')" prop="custom_time_ranges">
        <DragWeekTime
          v-model="form.custom_time_ranges"
          :data="weekTimeData"
          @onClear="clearWeektime"
        />
      </a-form-model-item>
    </a-form-model>
  </a-modal>
</template>

<script>
import _ from 'lodash'
import momentTimezone from 'moment-timezone'
import { mergeTimeRange } from '@/modules/oneterm/views/access/time/mergeTimeRange.js'
import { splitTimeRange } from '@/modules/oneterm/views/access/time/splitTimeRange.js'

import DragWeekTime from '@/modules/oneterm/components/dragWeektime'
import weekTimeData from '@/modules/oneterm/components/dragWeektime/weektimeData'

const DEFAULT_FORM = {
  timezone: momentTimezone.tz.guess(),
  custom_time_ranges: []
}

export default {
  name: 'CustomTimeModal',
  components: {
    DragWeekTime
  },
  data() {
    return {
      visible: false,
      form: { ...DEFAULT_FORM },
      rules: {
        timezone: [{ required: true, message: this.$t(`placeholder1`) }],
        custom_time_ranges: [{ required: true, message: this.$t(`placeholder2`) }],
      },
      weekTimeData: _.cloneDeep(weekTimeData)
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
  methods: {
    open(data) {
      this.visible = true

      if (data) {
        let custom_time_ranges = []
        if (data?.custom_time_ranges?.length) {
          const timeRanges = splitTimeRange(data.custom_time_ranges)
          custom_time_ranges = timeRanges.map((item) => {
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
          timezone: data?.timezone ?? momentTimezone.tz.guess(),
          custom_time_ranges
        }
      }
    },
    clearWeektime() {
      this.weekTimeData.forEach((item) => {
        item.child.forEach((t) => {
          this.$set(t, 'check', false)
        })
      })
      this.form.custom_time_ranges = []
    },
    handleCancel() {
      this.visible = false
    },
    async handleOk() {
      this.$refs.customTimeFormRef.validate(async (valid) => {
        if (!valid) return
        let custom_time_ranges = []
        if (this?.form?.custom_time_ranges?.length) {
          custom_time_ranges = mergeTimeRange(this.form.custom_time_ranges)
        }
        this.$emit('ok', {
          custom_time_ranges,
          timezone: this.form.timezone
        })
        this.handleCancel()
      })
    },
  },
}
</script>

<style></style>
