<template>
  <div class="assets-basic">
    <a-form-model ref="configForm" :model="form" :rules="rules" :label-col="{ span: 4 }" :wrapper-col="{ span: 14 }">
      <a-form-model-item :label="$t('oneterm.assetList.timeout')" prop="timeout">
        <a-input-number
          :min="0"
          :max="7200"
          v-model="form.timeout"
          :formatter="(value) => `${value}s`"
          :parser="(value) => value.replace('s', '')"
        />
      </a-form-model-item>
      <a-form-model-item label=" " :colon="false">
        <a-space>
          <a-button :loading="loading" @click="getConfig()">{{ $t('reset') }}</a-button>
          <a-button :loading="loading" type="primary" @click="handleSave">{{ $t('save') }}</a-button>
        </a-space>
      </a-form-model-item>
    </a-form-model>
  </div>
</template>

<script>
import { getConfig, postConfig } from '../../../api/config'
export default {
  name: 'BasicSetting',
  data() {
    return {
      loading: false,
      form: {
        timeout: 5,
      },
      rules: {},
    }
  },
  mounted() {
    this.getConfig()
  },
  methods: {
    async getConfig() {
      await getConfig().then((res) => {
        const { timeout = 5 } = res?.data
        this.form = {
          timeout,
        }
      })
    },
    handleSave() {
      this.loading = true
      postConfig({ ...this.form })
        .then(() => {
          this.$message.success(this.$t('saveSuccess'))
        })
        .finally(async () => {
          this.getConfig()
          this.loading = false
        })
    },
  },
}
</script>

<style lang="less" scoped>
.assets-basic {
  background-color: #fff;
  height: calc(100vh - 48px - 40px - 40px);
  border-bottom-left-radius: 15px;
  border-bottom-right-radius: 15px;
  padding: 18px;
}
</style>
