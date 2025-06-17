<template>
  <div class="display-setting">
    <a-form-model
      ref="displaySettingForm"
      :model="form"
      :rules="rules"
      :label-col="{ span: 7 }"
      :wrapper-col="{ span: 14 }"
    >
      <a-form-model-item :label="$t('oneterm.terminalDisplay.fontFamily')" prop="font_family">
        <a-select
          v-model="form.font_family"
          :placeholder="$t('placeholder2')"
          class="display-setting-input"
        >
          <a-select-option
            v-for="item in fontFamilySelectOptions"
            :key="item.value"
            :value="item.value"
            :title="item.name"
          >
            <span
              :style="{
                fontFamily: item.value !== 'default' ? item.value : 'inherit'
              }"
            >
              {{ item.name }}
            </span>
          </a-select-option>
        </a-select>
      </a-form-model-item>

      <a-form-model-item :label="$t('oneterm.terminalDisplay.fontSize')" prop="fontSize">
        <a-select
          v-model="form.font_size"
          :placeholder="$t('placeholder2')"
          class="display-setting-input"
        >
          <a-select-option
            v-for="item in fontSizeSelectOptions"
            :key="item.value"
            :value="item.value"
            :title="item.name"
          >
            {{ item.name }}
          </a-select-option>
        </a-select>
      </a-form-model-item>

      <a-form-model-item :label="$t('oneterm.terminalDisplay.lineHeight')" prop="line_height">
        <a-input-number
          v-model="form.line_height"
          :min="1"
          :max="2"
          :precision="1"
          :step="0.1"
          class="display-setting-input"
        />
      </a-form-model-item>

      <a-form-model-item :label="$t('oneterm.terminalDisplay.letterSpacing')" prop="letter_spacing">
        <a-input-number
          v-model="form.letter_spacing"
          :min="0"
          :max="20"
          :precision="0"
          :step="1"
          class="display-setting-input"
        />
      </a-form-model-item>

      <a-form-model-item :label="$t('oneterm.terminalDisplay.cursorStyle')" prop="cursorStyle">
        <a-radio-group
          v-model="form.cursor_style"
          button-style="solid"
        >
          <a-radio-button value="block">
            <div class="display-setting-cursor">▋</div>
          </a-radio-button>
          <a-radio-button value="underline">
            <div class="display-setting-cursor">▁</div>
          </a-radio-button>
          <a-radio-button value="bar">
            <div class="display-setting-cursor">▏</div>
          </a-radio-button>
        </a-radio-group>
      </a-form-model-item>

      <a-form-model-item :label="$t('oneterm.terminalDisplay.remoteDesktopResolution')" prop="cursorStyle">
        <ResolutionSetting
          class="display-setting-input"
          v-model="form.settings.resolution"
        />
      </a-form-model-item>

      <a-form-model-item :wrapper-col="{ span: 14, offset: 7 }">
        <a-button type="primary" @click="handleSubmit">
          {{ $t('save') }}
        </a-button>
        <a-button class="display-setting-reset" @click="resetForm">
          {{ $t('reset') }}
        </a-button>
      </a-form-model-item>
    </a-form-model>
  </div>
</template>

<script>
import _ from 'lodash'
import { defaultPreferenceSetting } from './constants.js'
import { getPreference, putPreference } from '@/modules/oneterm/api/preference.js'
import { isFontAvailable } from '@/modules/oneterm/utils/index.js'

import ResolutionSetting from './resolutionSetting/index.vue'

const allFontFamily = [
  // Windows
  'Consolas', 'Courier New', 'Lucida Console', 'MS Gothic', 'NSimSun', 'SimSun-ExtB', 'Cascadia Mono', 'Cascadia Code',

  // macOS
  'Menlo', 'Monaco', 'Courier', 'Andale Mono', 'Helvetica Neue', 'PT Mono', 'SF Mono',

  // Linux
  'DejaVu Sans Mono', 'Liberation Mono', 'Ubuntu Mono', 'Noto Mono', 'Droid Sans Mono', 'FreeMono',
]

export default {
  name: 'DisplaySetting',
  components: {
    ResolutionSetting
  },
  data() {
    return {
      form: _.omit(defaultPreferenceSetting, 'theme'),
      rules: {
        font_family: [{ required: true, message: this.$t('placeholder2') }],
        font_size: [{ required: true, message: this.$t('placeholder2') }],
        line_height: [{ required: true, message: this.$t('placeholder1') }],
        letter_spacing: [{ required: true, message: this.$t('placeholder1') }],
        cursor_style: [{ required: true, message: this.$t('placeholder2') }],
      },
      fontFamilySelectOptions: [
        {
          name: this.$t('default'),
          value: 'default'
        },
        ...allFontFamily.filter((v) => isFontAvailable(v)).map((v) => ({
          name: v,
          value: v
        }))
      ],
      fontSizeSelectOptions: [12, 14, 16, 18, 20, 22, 24, 26, 28].map(value => ({
        value,
        name: `${value}px`
      }))
    }
  },

  mounted() {
    this.initData()
  },

  methods: {
    async initData() {
      const res = await getPreference()
      const data = res?.data || {}

      const form = {}
      Object.keys(this.form).map((key) => {
        form[key] = data?.[key] || this.form[key]
      })
      this.form = form
    },

    handleSubmit() {
      this.$refs.displaySettingForm.validate(async (valid) => {
        if (valid) {
          const params = {
            ...this.form
          }
          await putPreference(params)

          this.$message.success(this.$t('saveSuccess'))

          this.$emit('ok')
        }
      })
    },

    resetForm() {
      this.form = _.omit(defaultPreferenceSetting, 'theme')
    }
  },
}
</script>

<style lang="less" scoped>
.display-setting {
  background-color: #FFFFFF;
  width: 100%;
  padding: 24px 0px 1px;
  height: calc(100vh - 130px);

  &-input {
    width: 250px;
  }

  &-cursor {
    padding: 0 8px;
    display: flex;
    align-items: center;
    justify-content: center;
  }

  &-reset {
    margin-left: 12px;
  }
}
</style>
