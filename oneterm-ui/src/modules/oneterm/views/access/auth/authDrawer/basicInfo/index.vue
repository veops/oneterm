<template>
  <div class="basic-form">
    <a-form-model-item :label="$t('name')" prop="name">
      <a-input
        :value="form.name"
        :placeholder="$t('placeholder1')"
        @change="(e) => handleFormChange(['name'], e.target.value)"
      />
    </a-form-model-item>
    <a-form-model-item :label="$t('description')" prop="description">
      <a-input
        :value="form.description"
        :placeholder="$t('placeholder1')"
        @change="(e) => handleFormChange(['description'], e.target.value)"
      />
    </a-form-model-item>
    <ValidTime
      :valid_from="form.valid_from"
      :valid_to="form.valid_to"
      @change="handleFormChange"
    />
    <a-form-model-item :label="$t('oneterm.isEnable')" prop="enabled">
      <a-switch
        :checked="form.enabled"
        @change="(checked) => handleFormChange(['enabled'], checked)"
      />
    </a-form-model-item>
  </div>
</template>

<script>
import ValidTime from './validTime.vue'

export default {
  name: 'BasicInfo',
  components: {
    ValidTime
  },
  props: {
    form: {
      type: Object,
      default: () => {}
    }
  },
  data() {
    return {}
  },
  methods: {
    handleFormChange(keys, value) {
      this.$emit(
        'change',
        {
          keys,
          value
        }
      )
    }
  },
}
</script>

<style lang="less" scoped>
.basic-form {
  width: 100%;
}
</style>
