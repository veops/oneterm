<template>
  <a-form-model ref="form" :model="form" :rules="rules" :label-col="{ span: 5 }" :wrapper-col="{ span: 16 }">
    <a-form-model-item :label="$t('oneterm.protocol')">
      <div class="protocol-box" v-for="(pro, index) in protocols" :key="pro.id">
        <a-input-group compact>
          <a-select v-model="pro.value" style="width: 150px" @change="(value) => changeProValue(value, index)">
            <a-select-opt-group
              v-for="(protocolGroup, protocolGroupIndex) in protocolSelectOption"
              :key="protocolGroupIndex"
            >
              <div slot="label">{{ $t(protocolGroup.title) }}</div>
              <a-select-option
                v-for="(protocol) in protocolGroup.list"
                :key="protocol.key"
                :value="protocol.key"
                :disabled="protocols.some((protocol) => protocol.value === protocol.key && protocol.value !== pro.value)"
              >
                <div class="protocol-select-item">
                  <ops-icon :type="protocol.icon" />
                  <span class="protocol-select-item-text">{{ protocol.label }}</span>
                </div>
              </a-select-option>
            </a-select-opt-group>
          </a-select>
          <a-input-number
            v-model="pro.port"
            :min="0"
            :placeholder="$t('oneterm.assetList.protocolPlaceholder')"
            :precision="0"
            style="width: calc(100% - 150px)"
          />
        </a-input-group>
        <a-space>
          <a
            v-if="showAddProtocol"
            @click="addPro"
          >
            <a-icon type="plus-circle"/>
          </a>
          <a
            v-if="protocols && protocols.length > 1"
            style="color:red"
            @click="deletePro(index)"
          >
            <ops-icon type="icon-xianxing-delete" />
          </a>
        </a-space>
      </div>

      <WebConfigForm
        v-if="hasWebProtocol"
        :config="form.web_config"
      />
    </a-form-model-item>

    <a-form-model-item
      :label="$t('oneterm.gateway')"
      prop="gateway_id"
      :style="{ display: 'flex', alignItems: 'center' }"
    >
      <treeselect
        class="custom-treeselect custom-treeselect-white"
        :style="{
          '--custom-height': '32px',
          lineHeight: '32px'
        }"
        v-model="form.gateway_id"
        :placeholder="`${$t(`placeholder2`)}`"
        :multiple="false"
        :clearable="true"
        searchable
        :options="gatewayList"
        :normalizer="
          (node) => {
            return {
              id: node.id,
              label: node.name,
            }
          }
        "
      >
        <div
          :title="node.label"
          slot="option-label"
          slot-scope="{ node }"
          :style="{ width: '100%', whiteSpace: 'nowrap', textOverflow: 'ellipsis', overflow: 'hidden' }"
        >
          {{ node.label }}
        </div>
      </treeselect>
    </a-form-model-item>
  </a-form-model>
</template>

<script>
import _ from 'lodash'
import { v4 as uuidv4 } from 'uuid'
import { getGatewayList } from '@/modules/oneterm/api/gateway'
import {
  protocolSelectOption,
  protocolMap,
  DEFAULT_WEB_CONFIG,
  AUTH_MODE,
  ACCESS_POLICY
} from './constants.js'

import WebConfigForm from './webConfigForm.vue'

export default {
  name: 'Protocol',
  components: {
    WebConfigForm
  },
  data() {
    return {
      form: {
        gateway_id: undefined,
        web_config: _.cloneDeep(DEFAULT_WEB_CONFIG)
      },
      rules: {},
      protocolSelectOption: _.cloneDeep(protocolSelectOption),
      protocols: [{ id: uuidv4(), value: 'ssh', port: 22 }],
      gatewayList: [],
    }
  },
  computed: {
    showAddProtocol() {
      return this.protocols?.length < 3 && !this.hasWebProtocol
    },
    hasWebProtocol() {
      return this.protocols.some((protocol) => ['https', 'http'].includes(protocol.value))
    }
  },
  watch: {
    protocols: {
      immediate: true,
      deep: true,
      handler(protocols) {
        const protocolTypeList = (protocols || []).map((protocol) => protocol.value)
        this.$emit('updateProtocols', _.uniq(protocolTypeList))
      }
    }
  },
  mounted() {
    getGatewayList({ page_index: 1 }).then((res) => {
      this.gatewayList = res?.data?.list || []
    })
  },
  methods: {
    addPro() {
      if (this.protocols.length < 3) {
        const value = ['ssh', 'rdp', 'vnc'].find((key) => this.protocols.every((protocol) => protocol.value !== key))
        this.protocols.push({ id: uuidv4(), value, port: protocolMap?.[value] || 0 })
      }
    },
    deletePro(index) {
      this.protocols.splice(index, 1)
    },
    getValues() {
      const { gateway_id, web_config } = this.form
      const _protocols = this.protocols.map((pro) => `${pro.value}:${pro.port}`)

      const cloneWebConfig = this.hasWebProtocol ? _.cloneDeep(web_config) : undefined
      if (cloneWebConfig) {
        this.handleWebConfigData(cloneWebConfig)

        if (cloneWebConfig?.login_accounts?.length) {
          cloneWebConfig.login_accounts = cloneWebConfig.login_accounts.map((account, index) => ({
            username: account.username,
            password: account.password,
            is_default: index === 0,
            status: 'active'
          }))
        }
      }

      return {
        gateway_id: gateway_id || undefined,
        web_config: cloneWebConfig,
        protocols: _protocols
      }
    },
    setValues({
      gateway_id,
      protocols,
      web_config
    }) {
      const cloneWebConfig = _.cloneDeep(web_config || DEFAULT_WEB_CONFIG)
      this.handleWebConfigData(cloneWebConfig)

      this.form = {
        gateway_id: gateway_id || undefined,
        web_config: cloneWebConfig
      }
      this.protocols = protocols?.length
        ? protocols.map((p) => ({
            id: uuidv4(),
            value: p.split(':')[0],
            port: Number(p.split(':')[1]),
          }))
        : [{ id: uuidv4(), value: 'ssh', port: 22 }]
    },

    handleWebConfigData(data) {
      if (data.auth_mode !== AUTH_MODE.SMART) {
        data.login_accounts = []
      }
      if (data.access_policy !== ACCESS_POLICY.READ_ONLY) {
        data.proxy_settings.allowed_methods = []
      }
    },

    changeProValue(value, index) {
      if (Object.keys(protocolMap).includes(value)) {
        this.protocols[index].port = protocolMap[value]
      }

      if (['https', 'http'].includes(value)) {
        this.protocols = this.protocols.filter((_, i) => index === i)
      }
    }
  },
}
</script>

<style lang="less" scoped>
.protocol-box {
  position: relative;
  .ant-space {
    position: absolute;
    right: -45px;
  }
}

/deep/ .protocol-select-item {
  display: flex;
  align-items: center;

  &-text {
    margin-left: 6px;
  }
}
</style>
