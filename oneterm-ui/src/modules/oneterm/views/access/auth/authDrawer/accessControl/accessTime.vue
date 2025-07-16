<template>
  <a-form-model-item :label="$t('oneterm.auth.accessTime')" :prop="formItemProp">
    <a-radio-group
      :value="accessTimeType"
      @change="handleRadioChange"
    >
      <a-radio :value="ACCESS_TIME_TYPE.TIME_TEMPLATE">
        {{ $t('oneterm.auth.timeTemplate') }}
      </a-radio>
      <a-radio :value="ACCESS_TIME_TYPE.CUSTOM_TIME">
        {{ $t('oneterm.auth.customTime') }}
        <a
          v-if="accessTimeType === ACCESS_TIME_TYPE.CUSTOM_TIME"
          @click="openCustomTimeModal"
        >
          <ops-icon type="veops-edit"/>
        </a>
      </a-radio>
    </a-radio-group>

    <a-select
      v-if="accessTimeType === ACCESS_TIME_TYPE.TIME_TEMPLATE"
      :value="form.access_control.time_template.template_id"
      :options="timeTemplateSelectOptions"
      :placeholder="$t('oneterm.auth.timeTemplateTip')"
      @change="handleTimeTemplateChange"
    />

    <div class="custom-time" v-else-if="accessTimeType === ACCESS_TIME_TYPE.CUSTOM_TIME">
      <template v-if="!form.access_control.custom_time_ranges.length">
        <a-icon type="exclamation-circle" />
        {{ $t('oneterm.auth.customTimeTip') }}
      </template>
      <template v-else>
        <div class="custom-time-row">
          <span class="custom-time-label">{{ $t('oneterm.auth.timeZone') }}:</span>
          {{ form.access_control.timezone }}
        </div>
        <div class="custom-time-row">
          <span class="custom-time-label">{{ $t('oneterm.auth.time') }}:</span>
          <div>
            <div
              v-for="(item, index) in customTimeText"
              :key="index"
            >
              <span>{{ item.start_time }}~{{ item.end_time }}</span>
              <span class="custom-time-week">{{ item.weekText }}</span>
            </div>
          </div>
        </div>
      </template>
    </div>

    <CustomTimeModal
      ref="customTimeModalRef"
      @ok="handleCustomTimeModalOk"
    />
  </a-form-model-item>
</template>

<script>
import { getTimeTemplateList } from '@/modules/oneterm/api/timeTemplate.js'
import { ACCESS_TIME_TYPE } from './constants.js'

import CustomTimeModal from './customTimeModal.vue'

export default {
  name: 'AccessTime',
  components: {
    CustomTimeModal
  },
  props: {
    accessTimeType: {
      type: String,
      default: ACCESS_TIME_TYPE.TIME_TEMPLATE
    },
    form: {
      type: Object,
      default: () => {}
    }
  },
  data() {
    return {
      ACCESS_TIME_TYPE,
      timeTemplateSelectOptions: [],
    }
  },
  computed: {
    formItemProp() {
      return this.accessTimeType === ACCESS_TIME_TYPE.TIME_TEMPLATE ? 'access_control.time_template.template_id' : 'access_control.custom_time_ranges'
    },
    customTimeText() {
      const custom_time_ranges = this.form?.access_control?.custom_time_ranges || []

      return custom_time_ranges.map((item) => {
        return {
          start_time: item.start_time,
          end_time: item.end_time,
          weekText: item.weekdays.map((day) => this.$t(`oneterm.timeTemplate.day${day}`)).join(', ')
        }
      })
    }
  },
  mounted() {
    this.getTimeTemplateList()
  },
  methods: {
    getTimeTemplateList() {
      getTimeTemplateList({
        page_index: 1,
        page_size: 9999,
      }).then((res) => {
        const list = res?.data?.list || []
        this.timeTemplateSelectOptions = list.map((item) => ({
          value: item.id,
          label: item.name
        }))
      })
    },
    handleRadioChange(e) {
      const value = e?.target?.value
      if (value !== this.accessTimeType) {
        this.$emit('update:accessTimeType', value)
      }
    },
    handleTimeTemplateChange(value) {
      this.$emit('change', ['access_control', 'time_template', 'template_id'], value)
    },
    openCustomTimeModal() {
      const { custom_time_ranges = [], timezone } = this.form?.access_control || {}

      this.$refs.customTimeModalRef.open({
        custom_time_ranges,
        timezone
      })
    },
    handleCustomTimeModalOk(data) {
      const { custom_time_ranges, timezone } = data

      this.$emit('change', ['access_control', 'custom_time_ranges'], custom_time_ranges)
      this.$emit('change', ['access_control', 'timezone'], timezone)
    }
  }
}
</script>

<style lang="less" scoped>
.custom-time {
  &-row {
    display: flex;
  }

  &-label {
    flex-shrink: 0;
    margin-right: 6px;
  }

  &-week {
    margin-left: 6px;
    font-size: 12px;
    color: #999999;
  }

  &-edit {
    flex-shrink: 0;
    margin-left: 12px;
  }
}
</style>
