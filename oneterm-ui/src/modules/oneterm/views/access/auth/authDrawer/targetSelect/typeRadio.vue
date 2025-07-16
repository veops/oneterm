<template>
  <div>
    <a-radio-group
      :value="selectData.type"
      :options="radioOptions"
      @change="handleRadioChange"
    />
    <slot
      v-if="selectData.type === TARGET_SELECT_TYPE.ID"
      name="id"
    ></slot>
    <a-input
      v-else-if="selectData.type === TARGET_SELECT_TYPE.REGEX"
      :value="selectData.values[0]"
      :placeholder="$t('oneterm.auth.regexTip')"
      @change="handleRegexInputChange"
    ></a-input>
    <a-select
      v-else-if="selectData.type === TARGET_SELECT_TYPE.TAG"
      mode="tags"
      :value="selectData.values"
      :options="tagSelectOptions"
      :placeholder="$t('oneterm.auth.tagsTip')"
      @change="handleTagChange"
    />
  </div>
</template>

<script>
import _ from 'lodash'
import { TARGET_SELECT_TYPE, TARGET_SELECT_TYPE_NAME } from '../../constants.js'

export default {
  name: 'TypeRadio',
  props: {
    idName: {
      type: String,
      default: ''
    },
    selectData: {
      type: Object,
      default: () => {}
    }
  },
  data() {
    return {
      TARGET_SELECT_TYPE,
      TARGET_SELECT_TYPE_NAME,
      tagSelectOptions: []
    }
  },
  computed: {
    radioOptions() {
      return Object.values(TARGET_SELECT_TYPE).map((key) => {
        return {
          value: key,
          label: key === TARGET_SELECT_TYPE.ID ? this.$t(TARGET_SELECT_TYPE_NAME[key], { name: this.$t(this.idName) }) : this.$t(TARGET_SELECT_TYPE_NAME[key])
        }
      })
    }
  },
  methods: {
    handleRadioChange(e) {
      const type = e?.target?.value || TARGET_SELECT_TYPE.ALL
      const values = []
      switch (type) {
        case TARGET_SELECT_TYPE.REGEX:
          values.push('')
          break
        default:
          break
      }

      this.$emit('change', {
        type,
        values,
        exclude_ids: this.selectData.exclude_ids
      })
    },

    handleRegexInputChange(e) {
      const value = e?.target?.value
      this.$emit('change', {
        type: this.selectData.type,
        exclude_ids: this.selectData.exclude_ids,
        values: [value],
      })
    },

    handleTagChange(value) {
      this.$emit('change', {
        type: this.selectData.type,
        exclude_ids: this.selectData.exclude_ids,
        values: value,
      })
      const tagSelectOptions = this.tagSelectOptions.concat(value.map((item) => ({
        label: item,
        value: item
      })))
      this.tagSelectOptions = _.uniqBy(tagSelectOptions, 'value')
    }
  }
}
</script>
