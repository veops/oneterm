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
          <a-input
            v-model="pro.label"
            :min="0"
            :placeholder="$t('oneterm.assetList.protocolPlaceholder')"
            :precision="0"
            style="width: calc(100% - 150px)"
          />
        </a-input-group>
        <a-space>
          <a v-if="protocols.length < 3" @click="addPro">
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
import { getGatewayList } from '../../../api/gateway'
export default {
  name: 'Protocol',
  data() {
    return {
      form: {
        gateway_id: undefined,
      },
      rules: {},
      protocolSelectOption: [
        {
          title: 'oneterm.assetList.basic',
          list: [
            {
              key: 'ssh',
              label: 'ssh',
              icon: 'a-oneterm-ssh2'
            },
            {
              key: 'rdp',
              label: 'rdp',
              icon: 'a-oneterm-ssh1'
            },
            {
              key: 'vnc',
              label: 'vnc',
              icon: 'oneterm-rdp'
            },
            {
              key: 'telnet',
              label: 'telnet',
              icon: 'a-telnet1'
            },
          ]
        },
        {
          title: 'oneterm.assetList.database',
          list: [
            {
              key: 'redis',
              label: 'redis',
              icon: 'oneterm-redis'
            },
            {
              key: 'mysql',
              label: 'mysql',
              icon: 'oneterm-mysql'
            },
            {
              key: 'mongodb',
              label: 'mongodb',
              icon: 'a-mongoDB1'
            },
            {
              key: 'postgresql',
              label: 'postgresql',
              icon: 'a-postgreSQL1'
            }
          ]
        }
      ],
      protocolMap: {
        'ssh': 22,
        'rdp': 3389,
        'vnc': 5900,
        'telnet': 23,
        'redis': 6379,
        'mysql': 3306,
        'mongodb': 27017,
        'postgresql': 5432
      },
      protocols: [{ id: uuidv4(), value: 'ssh', label: '22' }],
      gatewayList: [],
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
        this.protocols.push({ id: uuidv4(), value, label: this.protocolMap?.[value] || 0 })
      }
    },
    deletePro(index) {
      this.protocols.splice(index, 1)
    },
    getValues() {
      const { gateway_id } = this.form
      const _protocols = this.protocols.map((pro) => `${pro.value}:${pro.label}`)
      return { gateway_id, protocols: _protocols }
    },
    setValues({ gateway_id = undefined, protocols = [] }) {
      this.form = { gateway_id: gateway_id || undefined }
      this.protocols = protocols.length
        ? protocols.map((p) => ({
            id: uuidv4(),
            value: p.split(':')[0],
            label: Number(p.split(':')[1]),
          }))
        : [{ id: uuidv4(), value: 'ssh', label: 22 }]
    },
    changeProValue(value, index) {
      const _pro = _.cloneDeep(this.protocols[index])
      if (Object.keys(this.protocolMap).includes(value)) {
        _pro.label = this.protocolMap[value]
      }
      this.$set(this.protocols, index, _pro)
    },
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
