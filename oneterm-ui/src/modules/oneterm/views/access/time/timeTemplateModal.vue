<template>
  <a-modal
    :title="title"
    :visible="visible"
    :confirmLoading="confirmLoading"
    :width="1000"
    :bodyStyle="{
      maxHeight: '60vh',
      overflowY: 'auto'
    }"
    @cancel="handleCancel"
    @ok="handleOk"
  >
    <a-form-model
      ref="timeTemplateFormRef"
      :model="form"
      :rules="rules"
      :label-col="{ span: 5 }"
      :wrapper-col="{ span: 16 }"
    >
      <a-form-model-item :label="$t('name')" prop="name">
        <a-input v-model="form.name" :placeholder="$t(`placeholder1`)" />
      </a-form-model-item>
      <a-form-model-item :label="$t('description')" prop="description">
        <a-input v-model="form.description" :placeholder="$t(`placeholder1`)" />
      </a-form-model-item>
      <a-form-model-item :label="$t('oneterm.timeTemplate.category')" prop="category">
        <a-select
          v-model="form.category"
          :placeholder="$t('placeholder2')"
          :options="categorySelectOptions"
        />
      </a-form-model-item>
      <a-form-model-item :label="$t('oneterm.timeTemplate.timeZone')" prop="timezone">
        <a-select
          v-model="form.timezone"
          showSearch
          :placeholder="$t('placeholder2')"
          :options="timezoneSelectOptions"
        />
      </a-form-model-item>
      <a-form-model-item :label="$t('oneterm.timeTemplate.timeRange')" prop="time_ranges">
        <DragWeekTime
          v-model="form.time_ranges"
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
import { TIME_TEMPLATE_CATEGORY, TIME_TEMPLATE_CATEGORY_NAME } from './constants.js'
import { postTimeTemplate, putTimeTemplateById } from '@/modules/oneterm/api/timeTemplate.js'
import { mergeTimeRange } from './mergeTimeRange.js'
import { splitTimeRange } from './splitTimeRange.js'

import DragWeekTime from '@/modules/oneterm/components/dragWeektime'
import weekTimeData from '@/modules/oneterm/components/dragWeektime/weektimeData'

const DEFAULT_FORM = {
  name: '',
  description: '',
  category: TIME_TEMPLATE_CATEGORY.WORK,
  timezone: momentTimezone.tz.guess(),
  time_ranges: [],
  is_active: false
}

export default {
  name: 'TimeTemplateModal',
  components: {
    DragWeekTime
  },
  data() {
    return {
      visible: false,
      timeTemplateId: '',
      form: { ...DEFAULT_FORM },
      rules: {
        name: [{ required: true, message: this.$t(`placeholder1`) }],
        timezone: [{ required: true, message: this.$t(`placeholder1`) }],
        time_ranges: [{ required: true, message: this.$t(`placeholder2`) }],
      },
      confirmLoading: false,
      weekTimeData: _.cloneDeep(weekTimeData)
    }
  },
  computed: {
    title() {
      if (this.timeTemplateId) {
        return this.$t('oneterm.timeTemplate.editTimeTemplate')
      }
      return this.$t('oneterm.timeTemplate.createTimeTemplate')
    },
    categorySelectOptions() {
      return Object.values(TIME_TEMPLATE_CATEGORY).map((value) => {
        return {
          value,
          label: this.$t(TIME_TEMPLATE_CATEGORY_NAME[value])
        }
      })
    },
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
        let time_ranges = []
        if (data?.time_ranges?.length) {
          const timeRanges = splitTimeRange(data.time_ranges)
          time_ranges = timeRanges.map((item) => {
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
          name: data?.name ?? '',
          description: data?.description ?? '',
          category: data?.category ?? TIME_TEMPLATE_CATEGORY.WORK,
          timezone: data?.timezone ?? momentTimezone.tz.guess(),
          is_active: Boolean(data.is_active),
          time_ranges
        }

        this.timeTemplateId = data?.id ?? ''
      }
    },
    clearWeektime() {
      this.weekTimeData.forEach((item) => {
        item.child.forEach((t) => {
          this.$set(t, 'check', false)
        })
      })
      this.form.time_ranges = []
    },
    handleCancel() {
      this.form = { ...DEFAULT_FORM }
      this.clearWeektime()
      this.timeTemplateId = ''

      this.$refs.timeTemplateFormRef.resetFields()
      this.visible = false
    },
    async handleOk() {
      this.$refs.timeTemplateFormRef.validate(async (valid) => {
        if (!valid) return
        this.confirmLoading = true
        try {
          let time_ranges = []
          if (this?.form?.time_ranges?.length) {
            time_ranges = mergeTimeRange(this.form.time_ranges)
          }
          const params = {
            ...this.form,
            time_ranges
          }
          if (this.timeTemplateId) {
            await putTimeTemplateById(this.timeTemplateId, params)
            this.$message.success(this.$t('editSuccess'))
          } else {
            await postTimeTemplate(params)
            this.$message.success(this.$t('createSuccess'))
          }

          this.$emit('submit')
          this.handleCancel()
        } catch (e) {
          console.error('submit error:', e)
        } finally {
          this.confirmLoading = false
        }
      })
    },
  },
}
</script>
